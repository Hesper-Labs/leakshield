// Package store contains the gateway's PostgreSQL access layer.
//
// All tenant-scoped tables have row-level security enabled. The convention
// is: open a transaction, set `app.tenant_id` with SET LOCAL, and run all
// queries inside that transaction. WithTenant encapsulates that lifecycle.
package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned by lookup helpers when no row matches.
var ErrNotFound = errors.New("store: not found")

// DB wraps the connection pool with tenant-aware helpers. The exposed
// per-table helpers in this package take a *DB so they can opt into either
// a tenant-scoped transaction (RLS active) or a direct pool query (admin /
// bootstrap paths that need to look across tenants).
type DB struct {
	*pgxpool.Pool
}

// New wraps a pgxpool.Pool. The pool MUST connect as a role that does not
// bypass RLS — we deliberately rely on app.tenant_id being set per-tx.
func New(pool *pgxpool.Pool) *DB {
	return &DB{Pool: pool}
}

// WithTenant runs fn inside a transaction with `app.tenant_id` configured
// so RLS scopes every query to that tenant.
func (d *DB) WithTenant(ctx context.Context, tenantID string, fn func(pgx.Tx) error) error {
	tx, err := d.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID); err != nil {
		return fmt.Errorf("set tenant: %w", err)
	}
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	committed = true
	return nil
}

// WithoutTenant runs fn inside a regular transaction with no app.tenant_id
// set. Used for cross-tenant ops (bootstrap, super-admin lookups). RLS will
// still hide rows from any tenant-protected table — caller is expected to
// touch only tables where that's correct.
func (d *DB) WithoutTenant(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := d.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	committed = true
	return nil
}
