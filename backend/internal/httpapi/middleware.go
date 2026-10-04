package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"github.com/abdullah-dev5/bill-management/backend/internal/auth"
	"github.com/abdullah-dev5/bill-management/backend/internal/store"
)

const (
	sessionCookie = "bm_session"
	csrfCookie    = "bm_csrf"
	csrfHeader    = "X-CSRF-Token"
	sessionTTL    = 30 * 24 * time.Hour
)

// Routes that work without a session.
var publicRoutes = map[string]bool{
	apiPrefix + "/auth/signup":                 true,
	apiPrefix + "/auth/login":                  true,
	apiPrefix + "/auth/logout":                 true,
	apiPrefix + "/auth/password-reset/request": true,
	apiPrefix + "/auth/password-reset/confirm": true,
}

type ctxKey int

const authKey ctxKey = 1

// authInfo is the signed-in user and the household they act in.
type authInfo struct {
	UserID      uuid.UUID
	HouseholdID uuid.UUID
	Role        store.MemberRole
	SessionID   uuid.UUID
}

func authFrom(ctx context.Context) (authInfo, bool) {
	a, ok := ctx.Value(authKey).(authInfo)
	return a, ok
}

// mustAuth is used by handlers behind requireAuth, where the session is guaranteed.
func mustAuth(r *http.Request) authInfo {
	a, ok := authFrom(r.Context())
	if !ok {
		panic("httpapi: handler reached without auth")
	}
	return a
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		if r.URL.Path == "/healthz" {
			return
		}
		s.log.Info("http",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", ww.Status()),
			slog.Duration("took", time.Since(start)),
			slog.String("req_id", middleware.GetReqID(r.Context())),
		)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func limitBody(n int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, n)
			next.ServeHTTP(w, r)
		})
	}
}

// csrf implements double-submit protection: a readable bm_csrf cookie is issued to every client,
// and every state-changing request must echo it in the X-CSRF-Token header.
func (s *Server) csrf(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(csrfCookie)
		if err != nil || c.Value == "" {
			token := randomToken()
			http.SetCookie(w, &http.Cookie{
				Name: csrfCookie, Value: token, Path: "/", MaxAge: int((365 * 24 * time.Hour).Seconds()),
				SameSite: http.SameSiteLaxMode, Secure: s.cfg.CookieSecure,
			})
			c = &http.Cookie{Value: token}
		}
		if isUnsafe(r.Method) {
			sent := r.Header.Get(csrfHeader)
			if sent == "" || subtle.ConstantTimeCompare([]byte(sent), []byte(c.Value)) != 1 {
				writeError(w, errForbidden("Security check failed. Refresh the page and try again."))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// loadSession attaches authInfo when a valid session cookie is present.
func (s *Server) loadSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		if err != nil || c.Value == "" {
			next.ServeHTTP(w, r)
			return
		}
		ctx := r.Context()
		sess, err := s.db.GetSessionByTokenHash(ctx, store.GetSessionByTokenHashParams{TokenHash: auth.HashToken(c.Value), Now: s.now()})
		if err != nil {
			if !store.IsNotFound(err) {
				s.fail(w, r, err)
				return
			}
			s.clearSessionCookie(w)
			next.ServeHTTP(w, r)
			return
		}
		m, err := s.db.GetPrimaryMembership(ctx, sess.UserID)
		if err != nil {
			if !store.IsNotFound(err) {
				s.fail(w, r, err)
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		// Sliding expiry, refreshed at most hourly to avoid a write per request.
		if s.now().Sub(sess.LastSeenAt) > time.Hour {
			if err := s.db.TouchSession(ctx, store.TouchSessionParams{ID: sess.ID, ExpiresAt: s.now().Add(sessionTTL)}); err == nil {
				s.setSessionCookie(w, c.Value)
			}
		}
		a := authInfo{UserID: sess.UserID, HouseholdID: m.HouseholdID, Role: m.Role, SessionID: sess.ID}
		next.ServeHTTP(w, r.WithContext(context.WithValue(ctx, authKey, a)))
	})
}

// requireAuth rejects anonymous requests to private routes and read-only roles on writes.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if publicRoutes[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}
		a, ok := authFrom(r.Context())
		if !ok {
			writeError(w, errUnauthorized("unauthenticated", "Please sign in."))
			return
		}
		isAccountRoute := strings.HasPrefix(r.URL.Path, apiPrefix+"/auth/") || r.URL.Path == apiPrefix+"/account"
		if isUnsafe(r.Method) && a.Role == store.MemberRoleViewer && !isAccountRoute {
			writeError(w, errForbidden("Viewers can't make changes."))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: token, Path: "/", MaxAge: int(sessionTTL.Seconds()),
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: s.cfg.CookieSecure,
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: s.cfg.CookieSecure,
	})
}

func isUnsafe(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	}
	return true
}

func randomToken() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
