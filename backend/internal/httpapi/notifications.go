package httpapi

import (
	"net/http"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/abdullah-dev5/bill-management/backend/internal/apigen"
	"github.com/abdullah-dev5/bill-management/backend/internal/domain"
	"github.com/abdullah-dev5/bill-management/backend/internal/store"
)

// ListNotifications returns the signed-in person's reminders (FR-REM-01/04).
func (s *Server) ListNotifications(w http.ResponseWriter, r *http.Request, params apigen.ListNotificationsParams) {
	a := mustAuth(r)
	ctx := r.Context()
	limit := 30
	if params.Limit != nil {
		limit = *params.Limit
	}
	rows, err := s.db.ListNotifications(ctx, store.ListNotificationsParams{UserID: a.UserID, HouseholdID: a.HouseholdID, Limit: int32(limit)})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	unread, err := s.db.CountUnreadNotifications(ctx, store.CountUnreadNotificationsParams{UserID: a.UserID, HouseholdID: a.HouseholdID})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	out := apigen.NotificationList{Items: make([]apigen.Notification, 0, len(rows)), UnreadCount: int(unread)}
	for _, n := range rows {
		item := apigen.Notification{
			Id: n.ID, Kind: apigen.NotificationKind(n.Kind), CreatedAt: n.CreatedAt, Read: n.ReadAt != nil,
			ProfileId: n.ProfileID, ProfileName: n.ProfileName, ProfileIcon: n.ProfileIcon, BillId: n.BillID,
			PeriodKey: n.PeriodKey, Description: n.BillDescription, AmountMinor: n.AmountMinor,
		}
		if n.ProfileColor != nil {
			item.ProfileColor = ptr(int(*n.ProfileColor))
		}
		if n.DueDate != nil {
			item.DueDate = &openapi_types.Date{Time: *n.DueDate}
		}
		out.Items = append(out.Items, item)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) MarkNotificationsRead(w http.ResponseWriter, r *http.Request) {
	a := mustAuth(r)
	var in apigen.MarkNotificationsReadJSONRequestBody
	if err := decodeJSON(r, &in); err != nil {
		s.fail(w, r, err)
		return
	}
	all := in.All != nil && *in.All
	var ids []openapi_types.UUID
	if in.Ids != nil {
		ids = *in.Ids
	}
	if !all && len(ids) == 0 {
		s.fail(w, r, errField("ids", "Say which notifications to mark as read."))
		return
	}
	if _, err := s.db.MarkNotificationsRead(r.Context(), store.MarkNotificationsReadParams{
		Now: ptr(s.now()), UserID: a.UserID, HouseholdID: a.HouseholdID, AllUnread: all, Ids: ids,
	}); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

var defaultNotificationSettings = apigen.NotificationSettings{LeadDays: 3, DueSoon: true, Overdue: true, Missing: true}

func (s *Server) GetNotificationSettings(w http.ResponseWriter, r *http.Request) {
	a := mustAuth(r)
	row, err := s.db.GetNotificationSettings(r.Context(), store.GetNotificationSettingsParams{UserID: a.UserID, HouseholdID: a.HouseholdID})
	if store.IsNotFound(err) {
		writeJSON(w, http.StatusOK, defaultNotificationSettings)
		return
	} else if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toNotificationSettings(row))
}

func (s *Server) UpdateNotificationSettings(w http.ResponseWriter, r *http.Request) {
	a := mustAuth(r)
	var in apigen.NotificationSettings
	if err := decodeJSON(r, &in); err != nil {
		s.fail(w, r, err)
		return
	}
	fields := domain.FieldErrors{}
	if in.LeadDays < 0 || in.LeadDays > 14 {
		fields.Add("leadDays", "Choose between 0 and 14 days.")
	}
	validHour := func(h *int) bool { return h == nil || (*h >= 0 && *h <= 23) }
	if !validHour(in.QuietStart) {
		fields.Add("quietStart", "Choose an hour from 0 to 23.")
	}
	if !validHour(in.QuietEnd) {
		fields.Add("quietEnd", "Choose an hour from 0 to 23.")
	}
	if (in.QuietStart == nil) != (in.QuietEnd == nil) {
		fields.Add("quietEnd", "Set both the start and the end of quiet hours, or neither.")
	}
	if fields.Any() {
		s.fail(w, r, errValidation(fields))
		return
	}
	hour := func(h *int) *int16 {
		if h == nil {
			return nil
		}
		v := int16(*h)
		return &v
	}
	row, err := s.db.UpsertNotificationSettings(r.Context(), store.UpsertNotificationSettingsParams{
		UserID: a.UserID, HouseholdID: a.HouseholdID, LeadDays: int16(in.LeadDays), DueSoon: in.DueSoon, Overdue: in.Overdue,
		Missing: in.Missing, Push: in.Push, QuietStart: hour(in.QuietStart), QuietEnd: hour(in.QuietEnd),
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toNotificationSettings(row))
}

func toNotificationSettings(row store.NotificationSetting) apigen.NotificationSettings {
	hour := func(h *int16) *int {
		if h == nil {
			return nil
		}
		v := int(*h)
		return &v
	}
	return apigen.NotificationSettings{
		LeadDays: int(row.LeadDays), DueSoon: row.DueSoon, Overdue: row.Overdue, Missing: row.Missing, Push: row.Push,
		QuietStart: hour(row.QuietStart), QuietEnd: hour(row.QuietEnd),
	}
}
