// Command devdb creates the local development and test databases if they don't exist.
//
//	go run ./cmd/devdb                       # uses postgres://postgres:postgres@localhost:5432
//	go run ./cmd/devdb -admin <url> bill_dev bill_test
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
)

func main() {
	admin := flag.String("admin", "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable", "admin connection URL")
	flag.Parse()
	names := flag.Args()
	if len(names) == 0 {
		names = []string{"bill_dev", "bill_test"}
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, *admin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "connect:", err)
		os.Exit(1)
	}
	defer conn.Close(ctx)
	for _, name := range names {
		var exists bool
		if err := conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)", name).Scan(&exists); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if exists {
			fmt.Println("exists:", name)
			continue
		}
		if _, err := conn.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println("created:", name)
	}
}
