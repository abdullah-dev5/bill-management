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

	// DataKey derives the keys for field encryption, lookup hashes and signed file links.
	DataKey string

	StorageDriver     string // "local" (development) or "s3"
	StorageDir        string // local driver only
	S3                S3Config
	DirectUploads     bool  // browser talks to S3 through presigned URLs instead of via the API
	StorageQuotaBytes int64 // per household
}

type S3Config struct {
	Endpoint, Region, Bucket, AccessKeyID, SecretAccessKey string
}

// devDataKey is only accepted outside production, so local data stays readable across restarts.
const devDataKey = "development-only-data-key-do-not-use-in-production"

func (c Config) IsProduction() bool { return c.Env == "production" }

func Load() (Config, error) {
	c := Config{
		Env:          getenv("APP_ENV", "development"),
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		AppBaseURL:   strings.TrimRight(getenv("APP_BASE_URL", "http://localhost:5173"), "/"),
		TickToken:    os.Getenv("TICK_TOKEN"),
		ResendAPIKey: os.Getenv("RESEND_API_KEY"),
		MailFrom:     getenv("MAIL_FROM", "Bill Manager <no-reply@localhost>"),
		DataKey:      os.Getenv("DATA_ENCRYPTION_KEY"),
		StorageDir:   getenv("STORAGE_DIR", "var/storage"),
		S3: S3Config{
			Endpoint:        os.Getenv("S3_ENDPOINT"),
			Region:          getenv("S3_REGION", "ap-southeast-1"),
			Bucket:          getenv("S3_BUCKET", "documents"),
			AccessKeyID:     os.Getenv("S3_ACCESS_KEY_ID"),
			SecretAccessKey: os.Getenv("S3_SECRET_ACCESS_KEY"),
		},
	}
	defaultDriver := "local"
	if c.S3.Endpoint != "" {
		defaultDriver = "s3"
	}
	c.StorageDriver = getenv("STORAGE_DRIVER", defaultDriver)

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

	direct, err := strconv.ParseBool(getenv("STORAGE_DIRECT_UPLOADS", "true"))
	if err != nil {
		return c, fmt.Errorf("STORAGE_DIRECT_UPLOADS: %w", err)
	}
	c.DirectUploads = direct

	quotaMB, err := strconv.ParseInt(getenv("STORAGE_QUOTA_MB", "100"), 10, 64)
	if err != nil || quotaMB <= 0 {
		return c, fmt.Errorf("STORAGE_QUOTA_MB must be a positive number")
	}
	c.StorageQuotaBytes = quotaMB << 20

	var problems []error
	if c.DatabaseURL == "" {
		problems = append(problems, errors.New("DATABASE_URL is required"))
	}
	switch c.StorageDriver {
	case "local":
	case "s3":
		if c.S3.Endpoint == "" || c.S3.AccessKeyID == "" || c.S3.SecretAccessKey == "" {
			problems = append(problems, errors.New("S3_ENDPOINT, S3_ACCESS_KEY_ID and S3_SECRET_ACCESS_KEY are required for STORAGE_DRIVER=s3"))
		}
	default:
		problems = append(problems, fmt.Errorf("STORAGE_DRIVER must be local or s3, not %q", c.StorageDriver))
	}
	if c.DataKey == "" && !c.IsProduction() {
		c.DataKey = devDataKey
	}
	if len(c.DataKey) < 32 {
		problems = append(problems, errors.New("DATA_ENCRYPTION_KEY must be at least 32 characters"))
	}
	if c.IsProduction() {
		if c.DataKey == devDataKey {
			problems = append(problems, errors.New("DATA_ENCRYPTION_KEY must be set in production"))
		}
		if c.StorageDriver != "s3" {
			problems = append(problems, errors.New("production needs STORAGE_DRIVER=s3 (the instance disk is not persistent)"))
		}
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
