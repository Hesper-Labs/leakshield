package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Admin is a panel-login row.
type Admin struct {
	ID           uuid.UUID
	CompanyID    *uuid.UUID
	Email        string
	PasswordHash string
	Role         string
	Name         string // pulled from companies.settings or user table — best-effort
	CreatedAt    time.Time
	LastLoginAt  *time.Time
}

// CountAdmins returns the total number of panel admin rows. Used by the
// setup-status endpoint to report whether the install has been bootstrapped.
func (d *DB) CountAdmins(ctx context.Context) (int, error) {
	var n int
	err := d.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM admins").Scan(&n)
	if err != nil {
		return 0, err
	}
	return n, nil
}

// FindAdminByEmail loads an admin by email. Returns ErrNotFound when no row matches.
func (d *DB) FindAdminByEmail(ctx context.Context, email string) (*Admin, error) {
	a := &Admin{}
	err := d.Pool.QueryRow(ctx, `
		SELECT id, company_id, email, password_hash, role, created_at, last_login_at
		FROM admins
		WHERE email = $1
	`, email).Scan(&a.ID, &a.CompanyID, &a.Email, &a.PasswordHash, &a.Role, &a.CreatedAt, &a.LastLoginAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return a, nil
}

// MarkAdminLogin updates last_login_at on the admin row.
func (d *DB) MarkAdminLogin(ctx context.Context, id uuid.UUID) error {
	_, err := d.Pool.Exec(ctx, "UPDATE admins SET last_login_at = now() WHERE id = $1", id)
	return err
}
