// Package migrations embeds the goose SQL migrations and runs them.
package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log"
	"os"

	"github.com/pressly/goose/v3"

	"github.com/abdullah-dev5/bill-management/backend/internal/secure"
)

//go:embed *.sql
var FS embed.FS

// runKeys is available to Go migrations that encrypt or decrypt data.
var runKeys *secure.Keys

// Run applies ("up"), rolls back one ("down") or prints ("status") migrations.
// keys may be nil when there is no existing data to encrypt.
func Run(ctx context.Context, db *sql.DB, direction string, keys *secure.Keys) error {
	runKeys = keys
	goose.SetBaseFS(FS)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	switch direction {
	case "up":
		return goose.UpContext(ctx, db, ".")
	case "down":
		return goose.DownContext(ctx, db, ".")
	case "status":
		goose.SetLogger(log.New(os.Stdout, "", 0))
		return goose.StatusContext(ctx, db, ".")
	default:
		return fmt.Errorf("unknown migrate direction %q (want up, down or status)", direction)
	}
}
