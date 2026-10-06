package retrieval

import "context"

// --- Template for a real connector -------------------------------------------------
//
// This is the place to add an automatic connector for a provider ONCE that provider has
// granted access: an official API key or data-sharing agreement, or a public endpoint whose
// terms allow automated use. Copy this file, fill in Fetch, and register it in
// httpapi.connectors (internal/httpapi/server.go).
//
// Do NOT add a connector that works around a login, a CAPTCHA, or any other access control a
// provider put up to stop automated access. Those providers use the assisted flow instead (the
// person opens the official page in their own browser and uploads the bill). See ADR-002 and
// docs/08. Keeping this boundary is why Fetch below is left unimplemented.

// TemplateConnector shows the shape of an authorized connector. It is never registered; it
// returns ErrNotImplemented until a real Fetch is written against a permitted source.
type TemplateConnector struct {
	// APIKey, BaseURL, etc. for the provider's official API go here.
}

func (TemplateConnector) ID() string   { return "template" }
func (TemplateConnector) Name() string { return "Template (not active)" }

// Providers lists the catalogue short names this connector would serve, e.g. {"LESCO"}.
func (TemplateConnector) Providers() []string { return nil }

// Fetch retrieves one bill from the provider's authorized source and returns it as a Document
// (PDF or image); the app reads, validates and reconciles it like any other copy. Return
// ErrNotFound when the source has no bill for req.Reference yet — that is not a failure.
//
// Implement this only against a source the provider permits automated access to. Until then it
// stays unimplemented on purpose.
func (TemplateConnector) Fetch(ctx context.Context, req Request) (Document, error) {
	return Document{}, ErrNotImplemented
}
