package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DB wraps the pool with the generated queries and a transaction helper.
type DB struct {
	*Queries
	Pool *pgxpool.Pool
}

func NewDB(pool *pgxpool.Pool) *DB {
	return &DB{Queries: New(pool), Pool: pool}
}

// InTx runs fn inside a transaction. It commits if fn returns nil and rolls back otherwise.
func (db *DB) InTx(ctx context.Context, fn func(q *Queries) error) error {
	tx, err := db.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit
	if err := fn(db.Queries.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// IsNotFound reports whether err means "no row".
func IsNotFound(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

// UniqueViolation returns the violated constraint name, or "" if err is not a unique violation.
func UniqueViolation(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return pgErr.ConstraintName
	}
	return ""
}
