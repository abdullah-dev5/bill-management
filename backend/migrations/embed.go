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
)

//go:embed *.sql
var FS embed.FS

// Run applies ("up"), rolls back one ("down") or prints ("status") migrations.
func Run(ctx context.Context, db *sql.DB, direction string) error {
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
