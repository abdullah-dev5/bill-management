package retrieval

import "context"

// --- Template for a real connector -------------------------------------------------
//
// This is the place to add an automatic connector for a provider. Copy this file, fill in
// Fetch against the provider's source, and register it in httpapi.connectors
// (internal/httpapi/server.go).

// TemplateConnector shows the shape of a connector. It is never registered; it returns
// ErrNotImplemented until a real Fetch is written against the provider's source.
type TemplateConnector struct {
	// APIKey, BaseURL, etc. for the provider's official API go here.
}

func (TemplateConnector) ID() string   { return "template" }
func (TemplateConnector) Name() string { return "Template (not active)" }

// Providers lists the catalogue short names this connector would serve, e.g. {"LESCO"}.
func (TemplateConnector) Providers() []string { return nil }

// Fetch retrieves one bill from the provider's source and returns it as a Document
// (PDF or image); the app reads, validates and reconciles it like any other copy. Return
// ErrNotFound when the source has no bill for req.Reference yet — that is not a failure.
func (TemplateConnector) Fetch(ctx context.Context, req Request) (Document, error) {
	return Document{}, ErrNotImplemented
}
