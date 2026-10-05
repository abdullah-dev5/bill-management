// Package httpapi implements the REST API defined in api/openapi.yaml.
package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	oapimw "github.com/oapi-codegen/nethttp-middleware"

	"github.com/abdullah-dev5/bill-management/backend/internal/apigen"
	"github.com/abdullah-dev5/bill-management/backend/internal/config"
	"github.com/abdullah-dev5/bill-management/backend/internal/extract"
	"github.com/abdullah-dev5/bill-management/backend/internal/mail"
	"github.com/abdullah-dev5/bill-management/backend/internal/ratelimit"
	"github.com/abdullah-dev5/bill-management/backend/internal/secure"
	"github.com/abdullah-dev5/bill-management/backend/internal/storage"
	"github.com/abdullah-dev5/bill-management/backend/internal/store"
)

const apiPrefix = "/api/v1"

// Server implements apigen.ServerInterface.
type Server struct {
	db    *store.DB
	cfg   config.Config
	log   *slog.Logger
	mail  mail.Mailer
	keys  *secure.Keys
	files storage.Store
	now   func() time.Time

	extractor *extract.Chain
	wake      chan struct{} // nudges idle extraction workers

	loginByEmail *ratelimit.Limiter
	loginByIP    *ratelimit.Limiter
	signupByIP   *ratelimit.Limiter
	resetByKey   *ratelimit.Limiter
	uploadsByHH  *ratelimit.Limiter
	backupsByHH  *ratelimit.Limiter
	push         pushSender

	lastTick atomic.Int64 // unix seconds of the last successful tick
}

var _ apigen.ServerInterface = (*Server)(nil)

func New(db *store.DB, cfg config.Config, log *slog.Logger, mailer mail.Mailer, keys *secure.Keys, files storage.Store) *Server {
	vapidPub, vapidPriv := keys.VAPID()
	return &Server{
		db:           db,
		cfg:          cfg,
		log:          log,
		mail:         mailer,
		keys:         keys,
		files:        files,
		now:          time.Now,
		extractor:    NewExtractionChain(cfg),
		wake:         make(chan struct{}, 1),
		loginByEmail: ratelimit.New(5, 15*time.Minute),
		loginByIP:    ratelimit.New(30, 15*time.Minute),
		signupByIP:   ratelimit.New(signupLimit(cfg), time.Hour),
		resetByKey:   ratelimit.New(3, time.Hour),
		uploadsByHH:  ratelimit.New(100, time.Hour),
		backupsByHH:  ratelimit.New(10, time.Hour), // a backup reads every document
		push:         webPushSender{publicKey: vapidPub, privateKey: vapidPriv, subject: pushSubject(cfg)},
	}
}

// pushSubject identifies the sender to push services (an https URL or mailto:).
func pushSubject(cfg config.Config) string {
	if strings.HasPrefix(cfg.AppBaseURL, "https://") {
		return cfg.AppBaseURL
	}
	return "mailto:admin@example.com"
}

// Handler builds the full HTTP handler: health, internal tick and the versioned API.
func (s *Server) Handler() (http.Handler, error) {
	spec, err := apigen.GetSwagger()
	if err != nil {
		return nil, err
	}
	spec.Servers = nil // paths are matched with the prefix trimmed below

	validator := oapimw.OapiRequestValidatorWithOptions(spec, &oapimw.Options{
		Prefix:                apiPrefix,
		SilenceServersWarning: true,
		DoNotValidateServers:  true,
		Options: openapi3filter.Options{
			AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
			MultiError:         true,
		},
		ErrorHandlerWithOpts: func(_ context.Context, err error, w http.ResponseWriter, r *http.Request, opts oapimw.ErrorHandlerOpts) {
			if opts.MatchedRoute == nil {
				writeError(w, errNotFound("route"))
				return
			}
			writeError(w, specValidationError(err))
		},
	})

	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, s.logRequests, middleware.Recoverer, securityHeaders)
	r.NotFound(func(w http.ResponseWriter, _ *http.Request) { writeError(w, errNotFound("route")) })
	r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, &apiError{status: http.StatusMethodNotAllowed, code: "method_not_allowed", message: "Method not allowed."})
	})

	r.Get("/healthz", s.healthz)
	r.Post("/internal/tick", s.tick)
	// Signed file links for stores without presigned URLs (local) or when direct uploads are off.
	// Authorised by the signature in the link, not by the session.
	r.Put(filesPath+"{id}", s.putFile)
	r.Get(filesPath+"{id}", s.getFile)

	r.Group(func(api chi.Router) {
		api.Use(limitBody(1<<20), s.csrf, s.loadSession, s.requireAuth, validator)
		apigen.HandlerWithOptions(s, apigen.ChiServerOptions{
			BaseURL:    apiPrefix,
			BaseRouter: api,
			ErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, err error) {
				writeError(w, &apiError{status: http.StatusBadRequest, code: "invalid_parameter", message: err.Error()})
			},
		})
	})
	return r, nil
}

// today is the current date in the household's timezone.
func (s *Server) today(tz string) time.Time {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	y, m, d := s.now().In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// signupLimit is 10 sign-ups per IP per hour, relaxed on a developer's machine where
// end-to-end test runs create many accounts from one address.
func signupLimit(cfg config.Config) int {
	if cfg.Env == "development" {
		return 500
	}
	return 10
}
