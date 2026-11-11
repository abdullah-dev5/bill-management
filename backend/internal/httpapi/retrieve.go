package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/abdullah-dev5/bill-management/backend/internal/domain"
	"github.com/abdullah-dev5/bill-management/backend/internal/extract"
	"github.com/abdullah-dev5/bill-management/backend/internal/retrieval"
	"github.com/abdullah-dev5/bill-management/backend/internal/store"
)

const (
	connectorFailureThreshold = 5              // consecutive failures before a connector is switched off
	connectorCoolDown         = 24 * time.Hour // how long it stays off
	retrievalEvery            = 24 * time.Hour // at most one scheduled check per profile per day
	manualRetrievalGap        = 15 * time.Minute
	retrievalsPerTick         = 20
)

// Retrieval outcomes, stored on the profile and in retrieval_runs.
const (
	outcomeCreated   = "created"
	outcomeMerged    = "merged"
	outcomeUnchanged = "unchanged"
	outcomeNotFound  = "not_found"
	outcomeFailed    = "failed"
	outcomeSkipped   = "skipped"
)

type retrievalResult struct {
	Outcome string
	BillID  *uuid.UUID
	Message string
}

var errNoConnector = errors.New("no automatic connector for this provider")

// retrieveProfile fetches the official bill for one profile and reconciles it like any other
// copy (FR-RETR-04). Every outcome is recorded; a failing connector only affects itself.
func (s *Server) retrieveProfile(ctx context.Context, p store.BillProfile, providerShort, tz, currency, trigger, requestedPeriod string) retrievalResult {
	started := s.now()
	conn := s.retrieval.For(providerShort)
	res := s.runRetrieval(ctx, conn, p, providerShort, tz, currency, requestedPeriod)
	connID := "none"
	if conn != nil {
		connID = conn.ID()
	}
	finished := s.now()
	_ = s.db.SetProfileRetrievalResult(ctx, store.SetProfileRetrievalResultParams{
		ID: p.ID, LastRetrievalAt: &finished, LastRetrievalOutcome: &res.Outcome, LastRetrievalMessage: strPtr(res.Message),
	})
	_ = s.db.CreateRetrievalRun(ctx, store.CreateRetrievalRunParams{
		ID: uuid.Must(uuid.NewV7()), HouseholdID: p.HouseholdID, ProfileID: p.ID, Connector: connID, Trigger: trigger,
		PeriodKey: ptr(periodOrToday(s.now(), tz, requestedPeriod)), Outcome: res.Outcome, BillID: res.BillID, Message: strPtr(res.Message), StartedAt: started, FinishedAt: finished,
	})
	return res
}

func (s *Server) retrievalPeriod(_ store.BillProfile, tz string) string {
	return domain.MonthKey(s.today(tz))
}

func (s *Server) runRetrieval(ctx context.Context, conn retrieval.Connector, p store.BillProfile, providerShort, tz, currency, requestedPeriod string) retrievalResult {
	if conn == nil {
		return retrievalResult{Outcome: outcomeSkipped, Message: errNoConnector.Error()}
	}
	if h, err := s.db.GetConnectorHealth(ctx, conn.ID()); err == nil && h.DisabledUntil != nil && h.DisabledUntil.After(s.now()) {
		return retrievalResult{Outcome: outcomeSkipped, Message: "The official source isn't responding properly, so checks are paused until " +
			h.DisabledUntil.In(time.UTC).Format("2 Jan 15:04 UTC") + ". Add the bill by scanning or uploading it."}
	}
	ref := s.refDisplay(ctx, p)
	if ref == nil {
		return retrievalResult{Outcome: outcomeSkipped, Message: "Add the reference number to this profile first."}
	}
	today := s.today(tz)
	period := requestedPeriod
	if period == "" {
		period = domain.MonthKey(today)
	}
	req := retrieval.Request{Provider: providerShort, Reference: domain.NormalizeReference(*ref), Period: period}

	doc, err := s.retrieval.Fetch(ctx, conn, req)
	switch {
	case errors.Is(err, retrieval.ErrNotFound):
		s.connectorOK(ctx, conn)
		return retrievalResult{Outcome: outcomeNotFound, Message: "The official site has no bill for this reference number yet."}
	case errors.Is(err, retrieval.ErrThrottled):
		return retrievalResult{Outcome: outcomeSkipped, Message: err.Error()}
	case err != nil:
		s.connectorFailed(ctx, conn, err)
		return retrievalResult{Outcome: outcomeFailed, Message: "Couldn't reach the official site. Nothing else is affected; try again later or add the bill yourself."}
	}
	s.connectorOK(ctx, conn)

	res, err := s.reconcileRetrieved(ctx, conn, p, doc, today, currency)
	if err != nil {
		s.log.ErrorContext(ctx, "retrieved bill not saved", "profile_id", p.ID, "connector", conn.ID(), "err", err)
		return retrievalResult{Outcome: outcomeFailed, Message: "The official bill was fetched but couldn't be read. Add it by uploading it instead."}
	}
	return res
}

func (s *Server) connectorOK(ctx context.Context, c retrieval.Connector) {
	_ = s.db.ConnectorSucceeded(ctx, store.ConnectorSucceededParams{ID: c.ID(), LastSuccessAt: ptr(s.now())})
}

func (s *Server) connectorFailed(ctx context.Context, c retrieval.Connector, cause error) {
	msg := cause.Error()
	if len(msg) > 300 {
		msg = msg[:300]
	}
	h, err := s.db.ConnectorFailed(ctx, store.ConnectorFailedParams{
		ID: c.ID(), Now: ptr(s.now()), Error: &msg, Threshold: connectorFailureThreshold, DisableUntil: s.now().Add(connectorCoolDown),
	})
	if err == nil && h.DisabledUntil != nil && h.ConsecutiveFailures == connectorFailureThreshold {
		s.log.WarnContext(ctx, "retrieval connector disabled after repeated failures", "connector", c.ID(), "until", h.DisabledUntil, "last_error", msg)
	}
}

// reconcileRetrieved stores the official document, reads it, and merges it into the bill of that
// month or creates the bill. HESCO bills are verified only after the automatic validation gate;
// other connectors retain the existing unverified behavior until they define their own gate.
func (s *Server) reconcileRetrieved(ctx context.Context, conn retrieval.Connector, p store.BillProfile, doc retrieval.Document, today time.Time, currency string) (retrievalResult, error) {
	sum := sha256.Sum256(doc.Data)
	if existing, err := s.db.FindDocumentBySHA(ctx, store.FindDocumentBySHAParams{HouseholdID: p.HouseholdID, Sha256: sum[:]}); err == nil {
		if existing.BillID == nil {
			_ = s.db.AttachDocumentToProfile(ctx, store.AttachDocumentToProfileParams{
				ID: existing.ID, HouseholdID: p.HouseholdID, ProfileID: &p.ID,
			})
		}
		return retrievalResult{Outcome: outcomeUnchanged, BillID: existing.BillID, Message: "Already up to date: this official bill is already saved."}, nil
	} else if !store.IsNotFound(err) {
		return retrievalResult{}, err
	}

	docRow, err := s.storeRetrievedDocument(ctx, p.HouseholdID, &p.ID, doc)
	if err != nil {
		return retrievalResult{}, err
	}

	hints, err := s.db.ListProviderHints(ctx, &p.HouseholdID)
	if err != nil {
		return retrievalResult{}, err
	}
	providers := make([]extract.Provider, 0, len(hints))
	for _, h := range hints {
		providers = append(providers, extract.Provider{ShortName: h.ShortName, Name: h.Name, RefPattern: deref(h.RefNumberPattern)})
	}
	// Official PDFs carry their text; images go through the normal reading chain (no AI without consent).
	hh, err := s.db.GetHousehold(ctx, p.HouseholdID)
	if err != nil {
		return retrievalResult{}, errors.New("household reading settings could not be loaded")
	}
	read, trace, err := s.extractor.Run(ctx, extract.Input{
		Data: doc.Data, ContentType: doc.ContentType, FileName: doc.FileName,
		Providers: providers, AIConsent: hh.AiExtractionConsent,
	})
	if err != nil {
		s.log.WarnContext(ctx, "retrieved bill saved but could not be extracted", "profile_id", p.ID, "document_id", docRow.ID, "connector", conn.ID(), "trace", trace, "err", err)
		return retrievalResult{}, err
	}
	cand := candidateFromResult(read)
	period := cand["periodKey"]
	if period == "" || cand["payableMinor"] == "" {
		s.log.WarnContext(ctx, "retrieved bill saved but required fields were not extracted", "profile_id", p.ID, "document_id", docRow.ID, "connector", conn.ID(), "trace", trace)
		return retrievalResult{}, errors.New("billing month or amount not found in the official bill")
	}
	if read.RawText != "" {
		if err := s.db.SetDocumentText(ctx, store.SetDocumentTextParams{ID: docRow.ID, OcrText: ptr(extract.Redact(read.RawText, read.Text(extract.FCustomerName)))}); err != nil {
			return retrievalResult{}, err
		}
	}

	var out retrievalResult
	err = s.db.InTx(ctx, func(q *store.Queries) error {
		connector := conn.ID()
		src := sourceInput{Kind: store.BillSourceKindOfficial, DocumentID: &docRow.ID, Connector: &connector}
		system := authInfo{HouseholdID: p.HouseholdID} // no person behind it

		existingID, err := q.FindBillForPeriod(ctx, store.FindBillForPeriodParams{ProfileID: p.ID, PeriodKey: &period, ExcludeID: uuid.Nil})
		switch {
		case err == nil:
			b, err := q.GetBill(ctx, store.GetBillParams{ID: existingID, HouseholdID: p.HouseholdID, Today: today})
			if err != nil {
				return err
			}
			merged, err := mergeCandidate(ctx, q, s.now(), system, b.Bill, cand, src)
			if err != nil {
				return err
			}
			out = retrievalResult{Outcome: outcomeMerged, BillID: &existingID, Message: mergedMessage(merged)}
		case store.IsNotFound(err):
			id, err := s.createRetrievedBill(ctx, q, p, cand, currency, src, conn.ID() == "hesco-pitc")
			if err != nil {
				return err
			}
			out = retrievalResult{Outcome: outcomeCreated, BillID: &id, Message: "New bill fetched from the official site. Check it."}
		default:
			return err
		}
		if _, err := q.UpdateDocument(ctx, store.UpdateDocumentParams{ID: docRow.ID, HouseholdID: p.HouseholdID, BillID: out.BillID, Kind: docRow.Kind}); err != nil {
			return err
		}
		if out.Outcome == outcomeCreated {
			return s.notifyRetrieved(ctx, q, p, *out.BillID, period, cand)
		}
		return nil
	})
	return out, err
}

func (s *Server) storeRetrievedDocument(ctx context.Context, householdID uuid.UUID, profileID *uuid.UUID, doc retrieval.Document) (store.Document, error) {
	sum := sha256.Sum256(doc.Data)
	key := "households/" + householdID.String() + "/" + uuid.NewString()
	if err := s.files.Put(ctx, key, doc.ContentType, bytes.NewReader(doc.Data), int64(len(doc.Data))); err != nil {
		return store.Document{}, err
	}
	var out store.Document
	err := s.db.InTx(ctx, func(q *store.Queries) error {
		var err error
		out, err = q.CreateDocument(ctx, store.CreateDocumentParams{
			ID: uuid.Must(uuid.NewV7()), HouseholdID: householdID, ProfileID: profileID, Kind: store.DocumentKindBill, Source: store.DocumentSourceUpload,
			StorageKey: key, FileName: doc.FileName, ContentType: doc.ContentType, SizeBytes: int64(len(doc.Data)), Sha256: sum[:],
		})
		if err != nil {
			return err
		}
		_, err = q.MarkDocumentReady(ctx, store.MarkDocumentReadyParams{ID: out.ID, HouseholdID: householdID})
		return err
	})
	return out, err
}

func mergedMessage(m mergeOutcome) string {
	switch {
	case len(m.Conflicts) > 0:
		return "Fetched the official bill; some details differ from yours. Review them on the bill."
	case len(m.Filled) > 0:
		return "Fetched the official bill and filled in missing details."
	}
	return "Fetched the official bill; it matches what you have."
}

// candidateFromResult maps extracted fields to the reconciled field names.
func candidateFromResult(r extract.Result) map[string]string {
	m := map[string]string{}
	for from, to := range map[string]string{
		extract.FReference: "referenceNumber", extract.FBillingMonth: "periodKey", extract.FBillNumber: "billNumber", extract.FIssueDate: "issueDate", extract.FDueDate: "dueDate",
		extract.FCurrentCharges: "currentChargesMinor", extract.FArrears: "arrearsMinor", extract.FPayable: "payableMinor",
		extract.FPayableAfterDue: "payableAfterDueMinor", extract.FPreviousReading: "previousReading",
		extract.FCurrentReading: "currentReading", extract.FUnits: "units",
	} {
		if v := r.Text(from); v != "" {
			m[to] = v
		}
	}
	if m["arrearsMinor"] == "0" {
		delete(m, "arrearsMinor")
	}
	return m
}

func (s *Server) createRetrievedBill(ctx context.Context, q *store.Queries, p store.BillProfile, cand map[string]string, currency string, src sourceInput, autoVerify bool) (uuid.UUID, error) {
	num := func(k string) int64 { n, _ := strconv.ParseInt(cand[k], 10, 64); return n }
	payable, arrears := num("payableMinor"), num("arrearsMinor")
	current := num("currentChargesMinor")
	if cand["currentChargesMinor"] == "" {
		current = max(payable-arrears, 0)
	}
	expense, _ := time.Parse("2006-01", cand["periodKey"])
	params := updateParams(store.Bill{
		HouseholdID: p.HouseholdID, ProfileID: p.ID, CategoryID: p.CategoryID, ExpenseDate: expense, PeriodKey: ptr(cand["periodKey"]),
		CurrentChargesMinor: current, ArrearsMinor: arrears, PayableMinor: payable,
	})
	for _, f := range []string{"billNumber", "issueDate", "dueDate", "payableAfterDueMinor", "previousReading", "currentReading", "units"} {
		if v := cand[f]; v != "" {
			if err := setField(&params, f, v); err != nil {
				return uuid.Nil, err
			}
		}
	}
	id := uuid.Must(uuid.NewV7())
	var verifiedAt *time.Time
	if autoVerify && s.retrievedBillPassesGate(ctx, p, cand) {
		now := s.now()
		verifiedAt = &now
	}
	if _, err := q.CreateBill(ctx, store.CreateBillParams{
		ID: id, HouseholdID: p.HouseholdID, ProfileID: p.ID, CategoryID: p.CategoryID, BillNumber: params.BillNumber,
		PeriodKey: params.PeriodKey, IssueDate: params.IssueDate, DueDate: params.DueDate, ExpenseDate: params.ExpenseDate,
		CurrentChargesMinor: params.CurrentChargesMinor, ArrearsMinor: params.ArrearsMinor, PayableMinor: params.PayableMinor,
		PayableAfterDueMinor: params.PayableAfterDueMinor, Currency: currency, Tags: []string{},
		PreviousReading: params.PreviousReading, CurrentReading: params.CurrentReading, Units: params.Units,
		VerifiedAt: verifiedAt,
	}); err != nil {
		return uuid.Nil, err
	}

	created, err := q.GetBill(ctx, store.GetBillParams{ID: id, HouseholdID: p.HouseholdID, Today: s.now()})
	if err != nil {
		return uuid.Nil, err
	}
	if err := recordCreatedSource(ctx, q, s.now(), p.HouseholdID, nil, created.Bill, src); err != nil {
		return uuid.Nil, err
	}
	changes := diff(snapshot{}, billSnapshot(created.Bill))
	changes["source"] = []any{nil, "official"}
	raw, _ := json.Marshal(changes)
	return id, q.CreateSystemAuditEvent(ctx, store.CreateSystemAuditEventParams{
		ID: uuid.Must(uuid.NewV7()), HouseholdID: p.HouseholdID, EntityType: "bill", EntityID: id, Action: "create", Changes: raw,
	})
}

func (s *Server) retrievedBillPassesGate(ctx context.Context, p store.BillProfile, cand map[string]string) bool {
	if cand["periodKey"] == "" || cand["payableMinor"] == "" || cand["dueDate"] == "" || cand["referenceNumber"] == "" {
		return false
	}
	ref := s.refDisplay(ctx, p)
	return ref != nil && domain.NormalizeReference(*ref) == domain.NormalizeReference(cand["referenceNumber"])
}

// notifyRetrieved tells everyone in the household a new official bill arrived (FR-REM-06).
func (s *Server) notifyRetrieved(ctx context.Context, q *store.Queries, p store.BillProfile, billID uuid.UUID, period string, cand map[string]string) error {
	members, err := q.ListHouseholdMemberIDs(ctx, p.HouseholdID)
	if err != nil {
		return err
	}
	var due *time.Time
	if d, err := time.Parse("2006-01-02", cand["dueDate"]); err == nil {
		due = &d
	}
	amount, _ := strconv.ParseInt(cand["payableMinor"], 10, 64)
	for _, userID := range members {
		if _, err := q.InsertNotification(ctx, store.InsertNotificationParams{
			ID: uuid.Must(uuid.NewV7()), HouseholdID: p.HouseholdID, UserID: userID, Kind: store.NotificationKindBillRetrieved,
			ProfileID: &p.ID, BillID: &billID, PeriodKey: &period, DueDate: due, AmountMinor: &amount,
			DedupeKey: "retrieved:" + billID.String(), CreatedAt: s.now(),
		}); err != nil {
			return err
		}
	}
	return nil
}

// runRetrievals is the scheduled part (FR-RETR-03): opted-in profiles, at most once a day each,
// only while this month's bill is missing and its usual issue day is near.
func (s *Server) runRetrievals(ctx context.Context) (int, error) {
	if len(s.retrieval.All()) == 0 {
		return 0, nil
	}
	rows, err := s.db.ListRetrievalCandidates(ctx, store.ListRetrievalCandidatesParams{Cutoff: ptr(s.now().Add(-retrievalEvery)), RowLimit: retrievalsPerTick})
	if err != nil {
		return 0, err
	}
	n := 0
	for _, r := range rows {
		if s.retrieval.For(r.ProviderShort) == nil {
			continue
		}
		p := r.BillProfile
		today := s.today(r.Timezone)
		period := domain.MonthKey(today)
		if _, err := s.db.FindBillForPeriod(ctx, store.FindBillForPeriodParams{ProfileID: p.ID, PeriodKey: &period, ExcludeID: uuid.Nil}); err == nil {
			continue // this month's bill is already in
		} else if !store.IsNotFound(err) {
			return n, err
		}
		if p.TypicalIssueDay != nil {
			issueDate := time.Date(today.Year(), today.Month(), min(int(*p.TypicalIssueDay), daysInMonth(today)), 0, 0, 0, 0, time.UTC)
			if today.Before(issueDate.AddDate(0, 0, -1)) || today.After(issueDate.AddDate(0, 0, 2)) {
				continue
			}
		}
		attempts, err := s.db.CountScheduledRetrievalAttempts(ctx, store.CountScheduledRetrievalAttemptsParams{
			ProfileID: p.ID, PeriodKey: &period,
		})
		if err != nil {
			return n, err
		}
		if attempts >= 4 {
			continue
		}
		s.retrieveProfile(ctx, p, r.ProviderShort, r.Timezone, r.Currency, "schedule", "")
		n++
	}
	return n, nil
}

func daysInMonth(t time.Time) int {
	return time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func periodOrToday(now time.Time, tz, requested string) string {
	if requested != "" {
		return requested
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	return domain.MonthKey(now.In(loc))
}
