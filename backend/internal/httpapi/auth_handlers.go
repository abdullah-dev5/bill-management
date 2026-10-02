package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/abdullah-dev5/bill-management/backend/internal/apigen"
	"github.com/abdullah-dev5/bill-management/backend/internal/auth"
	"github.com/abdullah-dev5/bill-management/backend/internal/domain"
	"github.com/abdullah-dev5/bill-management/backend/internal/mail"
	"github.com/abdullah-dev5/bill-management/backend/internal/store"
)

// System type used for every property's default "General expenses" profile.
var generalExpensesTypeID = uuid.MustParse("a0000000-0000-4000-8000-000000000025")

const (
	minPasswordLen     = 10
	passwordResetTTL   = time.Hour
	accountDeletionTTL = 14 * 24 * time.Hour
	defaultProfileName = "General expenses"
)

func (s *Server) Signup(w http.ResponseWriter, r *http.Request) {
	var in apigen.SignupRequest
	if err := decodeJSON(r, &in); err != nil {
		s.fail(w, r, err)
		return
	}
	if !s.signupByIP.Allow(clientIP(r)) {
		s.fail(w, r, errTooManyRequests)
		return
	}
	fields := domain.FieldErrors{}
	email, ok := domain.NormalizeEmail(string(in.Email))
	if !ok {
		fields.Add("email", "Enter a valid email address.")
	}
	name := strings.TrimSpace(in.DisplayName)
	if name == "" {
		fields.Add("displayName", "Enter your name.")
	}
	if len([]rune(in.Password)) < minPasswordLen {
		fields.Add("password", fmt.Sprintf("Use at least %d characters.", minPasswordLen))
	}
	if fields.Any() {
		s.fail(w, r, errValidation(fields))
		return
	}

	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	ctx := r.Context()
	userID := uuid.Must(uuid.NewV7())
	err = s.db.InTx(ctx, func(q *store.Queries) error {
		if _, err := q.CreateUser(ctx, store.CreateUserParams{
			ID: userID, Email: email, PasswordHash: hash, DisplayName: name, Locale: "en-PK",
		}); err != nil {
			if store.UniqueViolation(err) != "" {
				return errConflict("email_taken", "An account with this email already exists. Try signing in.")
			}
			return err
		}
		return createHousehold(ctx, q, userID)
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if err := s.startSession(ctx, w, r, userID); err != nil {
		s.fail(w, r, err)
		return
	}
	s.writeMe(w, r, http.StatusCreated, userID)
}

// createHousehold sets up a new user's household: owner membership, default property
// ("My Home") and its default "General expenses" profile (FR-AUTH-04, FR-ACCT-13).
func createHousehold(ctx context.Context, q *store.Queries, userID uuid.UUID) error {
	hh, err := q.CreateHousehold(ctx, store.CreateHouseholdParams{
		ID: uuid.Must(uuid.NewV7()), Name: "My household", Currency: "PKR", Timezone: "Asia/Karachi",
	})
	if err != nil {
		return err
	}
	if err := q.CreateMembership(ctx, store.CreateMembershipParams{
		HouseholdID: hh.ID, UserID: userID, Role: store.MemberRoleOwner,
	}); err != nil {
		return err
	}
	prop, err := q.CreateProperty(ctx, store.CreatePropertyParams{
		ID: uuid.Must(uuid.NewV7()), HouseholdID: hh.ID, Name: "My Home", IsDefault: true,
	})
	if err != nil {
		return err
	}
	_, err = q.CreateProfile(ctx, store.CreateProfileParams{
		ID:          uuid.Must(uuid.NewV7()),
		HouseholdID: hh.ID,
		PropertyID:  prop.ID,
		CategoryID:  generalExpensesTypeID,
		Kind:        store.ProfileKindExpenseLog,
		Name:        defaultProfileName,
		Icon:        "receipt",
		Color:       7,
		IsDefault:   true,
		SortOrder:   1000,
	})
	return err
}

func (s *Server) Login(w http.ResponseWriter, r *http.Request) {
	var in apigen.LoginRequest
	if err := decodeJSON(r, &in); err != nil {
		s.fail(w, r, err)
		return
	}
	email, _ := domain.NormalizeEmail(in.Email)
	if !s.loginByEmail.Allow(email) || !s.loginByIP.Allow(clientIP(r)) {
		s.fail(w, r, errTooManyRequests)
		return
	}
	ctx := r.Context()
	bad := errUnauthorized("invalid_credentials", "That email and password don't match.")
	user, err := s.db.GetUserByEmail(ctx, email)
	if err != nil {
		if store.IsNotFound(err) {
			auth.VerifyDummy(in.Password)
			s.fail(w, r, bad)
			return
		}
		s.fail(w, r, err)
		return
	}
	if err := auth.VerifyPassword(user.PasswordHash, in.Password); err != nil {
		s.fail(w, r, bad)
		return
	}
	// Signing in again cancels a pending account deletion.
	if user.DeletionRequestedAt != nil {
		if err := s.db.SetUserDeletionRequested(ctx, store.SetUserDeletionRequestedParams{ID: user.ID}); err != nil {
			s.fail(w, r, err)
			return
		}
	}
	if err := s.startSession(ctx, w, r, user.ID); err != nil {
		s.fail(w, r, err)
		return
	}
	s.writeMe(w, r, http.StatusOK, user.ID)
}

func (s *Server) Logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
		if err := s.db.DeleteSessionByTokenHash(r.Context(), auth.HashToken(c.Value)); err != nil {
			s.fail(w, r, err)
			return
		}
	}
	s.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) LogoutAll(w http.ResponseWriter, r *http.Request) {
	a := mustAuth(r)
	if err := s.db.DeleteUserSessions(r.Context(), a.UserID); err != nil {
		s.fail(w, r, err)
		return
	}
	s.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) GetMe(w http.ResponseWriter, r *http.Request) {
	s.writeMe(w, r, http.StatusOK, mustAuth(r).UserID)
}

func (s *Server) RequestPasswordReset(w http.ResponseWriter, r *http.Request) {
	var in apigen.PasswordResetRequest
	if err := decodeJSON(r, &in); err != nil {
		s.fail(w, r, err)
		return
	}
	email, ok := domain.NormalizeEmail(in.Email)
	if !s.resetByKey.Allow("email:"+email) || !s.resetByKey.Allow("ip:"+clientIP(r)) {
		s.fail(w, r, errTooManyRequests)
		return
	}
	// Always 204 so the response never reveals whether an account exists.
	defer w.WriteHeader(http.StatusNoContent)
	if !ok {
		return
	}
	ctx := r.Context()
	user, err := s.db.GetUserByEmail(ctx, email)
	if err != nil {
		if !store.IsNotFound(err) {
			s.log.ErrorContext(ctx, "password reset lookup", "err", err)
		}
		return
	}
	token, hash, err := auth.NewToken()
	if err != nil {
		s.log.ErrorContext(ctx, "password reset token", "err", err)
		return
	}
	if err := s.db.CreatePasswordResetToken(ctx, store.CreatePasswordResetTokenParams{
		ID: uuid.Must(uuid.NewV7()), UserID: user.ID, TokenHash: hash, ExpiresAt: s.now().Add(passwordResetTTL),
	}); err != nil {
		s.log.ErrorContext(ctx, "password reset store", "err", err)
		return
	}
	link := s.cfg.AppBaseURL + "/reset-password?token=" + token
	if err := s.mail.Send(ctx, mail.Message{
		To:      user.Email,
		Subject: "Reset your Bill Manager password",
		Text: "Hi " + user.DisplayName + ",\n\nUse this link to choose a new password. It works for one hour:\n\n" +
			link + "\n\nIf you didn't ask for this, you can ignore this email.",
	}); err != nil {
		s.log.ErrorContext(ctx, "password reset email", "err", err)
	}
}

func (s *Server) ConfirmPasswordReset(w http.ResponseWriter, r *http.Request) {
	var in apigen.PasswordResetConfirm
	if err := decodeJSON(r, &in); err != nil {
		s.fail(w, r, err)
		return
	}
	if len([]rune(in.Password)) < minPasswordLen {
		s.fail(w, r, errField("password", fmt.Sprintf("Use at least %d characters.", minPasswordLen)))
		return
	}
	ctx := r.Context()
	tok, err := s.db.GetValidPasswordResetToken(ctx, auth.HashToken(in.Token))
	if err != nil {
		if store.IsNotFound(err) {
			s.fail(w, r, errField("token", "This reset link has expired or was already used. Request a new one."))
			return
		}
		s.fail(w, r, err)
		return
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	err = s.db.InTx(ctx, func(q *store.Queries) error {
		if err := q.UpdateUserPassword(ctx, store.UpdateUserPasswordParams{ID: tok.UserID, PasswordHash: hash}); err != nil {
			return err
		}
		if err := q.MarkPasswordResetTokenUsed(ctx, tok.ID); err != nil {
			return err
		}
		if err := q.DeleteUserPasswordResetTokens(ctx, tok.UserID); err != nil {
			return err
		}
		return q.DeleteUserSessions(ctx, tok.UserID)
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) RequestAccountDeletion(w http.ResponseWriter, r *http.Request) {
	a := mustAuth(r)
	ctx := r.Context()
	now := s.now()
	err := s.db.InTx(ctx, func(q *store.Queries) error {
		if err := q.SetUserDeletionRequested(ctx, store.SetUserDeletionRequestedParams{ID: a.UserID, DeletionRequestedAt: &now}); err != nil {
			return err
		}
		return q.DeleteUserSessions(ctx, a.UserID)
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) startSession(ctx context.Context, w http.ResponseWriter, r *http.Request, userID uuid.UUID) error {
	token, hash, err := auth.NewToken()
	if err != nil {
		return err
	}
	ua := r.UserAgent()
	if len(ua) > 300 {
		ua = ua[:300]
	}
	ip := clientIP(r)
	if err := s.db.CreateSession(ctx, store.CreateSessionParams{
		ID: uuid.Must(uuid.NewV7()), UserID: userID, TokenHash: hash,
		UserAgent: &ua, Ip: &ip, ExpiresAt: s.now().Add(sessionTTL),
	}); err != nil {
		return err
	}
	s.setSessionCookie(w, token)
	return nil
}

func (s *Server) writeMe(w http.ResponseWriter, r *http.Request, status int, userID uuid.UUID) {
	ctx := r.Context()
	user, err := s.db.GetUserByID(ctx, userID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	m, err := s.db.GetPrimaryMembership(ctx, userID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	hh, err := s.db.GetHousehold(ctx, m.HouseholdID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, status, apigen.Me{
		User: apigen.User{
			Id: user.ID, Email: openapiEmail(user.Email), DisplayName: user.DisplayName, Locale: user.Locale,
		},
		Household: toHousehold(hh),
		Role:      apigen.Role(m.Role),
	})
}
