// Package config loads runtime configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Env          string // "development" or "production"
	Port         int
	DatabaseURL  string
	AppBaseURL   string // public URL of the web app, used in emailed links
	CookieSecure bool
	TickToken    string // bearer token for POST /internal/tick
	ResendAPIKey string
	MailFrom     string
}

func (c Config) IsProduction() bool { return c.Env == "production" }

func Load() (Config, error) {
	c := Config{
		Env:          getenv("APP_ENV", "development"),
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		AppBaseURL:   strings.TrimRight(getenv("APP_BASE_URL", "http://localhost:5173"), "/"),
		TickToken:    os.Getenv("TICK_TOKEN"),
		ResendAPIKey: os.Getenv("RESEND_API_KEY"),
		MailFrom:     getenv("MAIL_FROM", "Bill Manager <no-reply@localhost>"),
	}

	port, err := strconv.Atoi(getenv("PORT", "8080"))
	if err != nil {
		return c, fmt.Errorf("PORT: %w", err)
	}
	c.Port = port

	secure, err := strconv.ParseBool(getenv("COOKIE_SECURE", strconv.FormatBool(c.IsProduction())))
	if err != nil {
		return c, fmt.Errorf("COOKIE_SECURE: %w", err)
	}
	c.CookieSecure = secure

	var problems []error
	if c.DatabaseURL == "" {
		problems = append(problems, errors.New("DATABASE_URL is required"))
	}
	if c.IsProduction() {
		if len(c.TickToken) < 32 {
			problems = append(problems, errors.New("TICK_TOKEN must be at least 32 characters in production"))
		}
		if !c.CookieSecure {
			problems = append(problems, errors.New("COOKIE_SECURE must be true in production"))
		}
	}
	return c, errors.Join(problems...)
}

func getenv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
