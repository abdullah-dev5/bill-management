// Package retrieval is the connector framework for fetching bills from official sources
// (M6, ADR-002, FR-RETR-01…06). Retrieval is one optional input channel: a connector only
// fetches a document; the app reads, validates and reconciles it like any other copy, and a
// failing connector affects nothing else.
//
// Connectors may only use official public lookup pages or APIs that permit automated access.
// Providers whose lookup pages use CAPTCHAs (PITC, SSGC) are served by the assisted flow instead:
// the person opens the official page in their own browser and uploads what they save.
package retrieval

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/abdullah-dev5/bill-management/backend/internal/ratelimit"
)

// ErrNotFound means the source answered but has no bill for this reference (yet). It is not
// a connector failure.
var ErrNotFound = errors.New("no bill found for this reference number")

// Request is what a connector looks up.
type Request struct {
	Provider  string // catalogue short name, e.g. "HESCO"
	Reference string // normalised reference / consumer number
	Period    string // the billing month expected (YYYY-MM), if known
}

// Document is the official bill as the source returned it.
type Document struct {
	Data        []byte
	ContentType string
	FileName    string
}

// Connector fetches official bills for some providers.
type Connector interface {
	ID() string
	Name() string
	Providers() []string // catalogue short names it serves
	Fetch(ctx context.Context, req Request) (Document, error)
}

// Registry holds the enabled connectors and throttles each one (FR-RETR-03: a global limit per
// provider site, on top of the one-request-per-profile-per-day rule enforced by the caller).
type Registry struct {
	connectors []Connector
	limiter    *ratelimit.Limiter
	Timeout    time.Duration
}

func NewRegistry(perMinute int, connectors ...Connector) *Registry {
	return &Registry{connectors: connectors, limiter: ratelimit.New(perMinute, time.Minute), Timeout: 30 * time.Second}
}

// For returns the connector serving a provider, if any.
func (r *Registry) For(provider string) Connector {
	if r == nil {
		return nil
	}
	for _, c := range r.connectors {
		for _, p := range c.Providers() {
			if strings.EqualFold(p, provider) {
				return c
			}
		}
	}
	return nil
}

func (r *Registry) All() []Connector {
	if r == nil {
		return nil
	}
	return r.connectors
}

// ErrThrottled means the connector's site has had its share of requests this minute.
var ErrThrottled = errors.New("too many requests to this source; try again in a minute")

// Fetch runs one connector with the throttle and a timeout. Panics inside a connector are
// turned into errors so a broken connector can't take the app down.
func (r *Registry) Fetch(ctx context.Context, c Connector, req Request) (doc Document, err error) {
	if !r.limiter.Allow(c.ID()) {
		return Document{}, ErrThrottled
	}
	ctx, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()
	defer func() {
		if p := recover(); p != nil {
			err = errors.New("the connector crashed")
		}
	}()
	return c.Fetch(ctx, req)
}
