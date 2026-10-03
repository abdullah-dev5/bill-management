// Package mail sends transactional email (password resets now; reminders later).
package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

type Message struct {
	To      string
	Subject string
	Text    string
}

type Mailer interface {
	Send(ctx context.Context, m Message) error
}

// LogMailer writes messages to the log instead of sending them (development).
type LogMailer struct{ Log *slog.Logger }

func (l LogMailer) Send(_ context.Context, m Message) error {
	l.Log.Info("email (not sent: development mailer)", "to", m.To, "subject", m.Subject, "body", m.Text)
	return nil
}

// ResendMailer sends through the Resend HTTP API (free tier, no card).
type ResendMailer struct {
	APIKey string
	From   string
	Client *http.Client
}

func NewResend(apiKey, from string) *ResendMailer {
	return &ResendMailer{APIKey: apiKey, From: from, Client: &http.Client{Timeout: 10 * time.Second}}
}

func (r *ResendMailer) Send(ctx context.Context, m Message) error {
	body, err := json.Marshal(map[string]any{
		"from":    r.From,
		"to":      []string{m.To},
		"subject": m.Subject,
		"text":    m.Text,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+r.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("resend: status %d", resp.StatusCode)
	}
	return nil
}
