package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Company is a tenant row.
type Company struct {
	ID         uuid.UUID
	Name       string
	Slug       string
	DEKWrapped []byte
	KEKID      string
	CreatedAt  time.Time
}

// FindCompanyByID returns the company matching id.
func (d *DB) FindCompanyByID(ctx context.Context, id uuid.UUID) (*Company, error) {
	c := &Company{}
	err := d.Pool.QueryRow(ctx, `
		SELECT id, name, slug, dek_wrapped, kek_id, created_at
		FROM companies WHERE id = $1
	`, id).Scan(&c.ID, &c.Name, &c.Slug, &c.DEKWrapped, &c.KEKID, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

// FindCompanyBySlug returns the company matching slug.
func (d *DB) FindCompanyBySlug(ctx context.Context, slug string) (*Company, error) {
	c := &Company{}
	err := d.Pool.QueryRow(ctx, `
		SELECT id, name, slug, dek_wrapped, kek_id, created_at
		FROM companies WHERE slug = $1
	`, slug).Scan(&c.ID, &c.Name, &c.Slug, &c.DEKWrapped, &c.KEKID, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

// BootstrapParams holds the inputs for creating the very first admin + company.
type BootstrapParams struct {
	CompanyName    string
	CompanySlug    string
	DEKWrapped     []byte
	KEKID          string
	AdminEmail     string
	AdminPasswordH string
	AdminRole      string // "super_admin" on bootstrap
	AdminFullName  string // optional, stored in users (employee row mirroring the admin)
}

// BootstrapResult holds the IDs of the rows created by Bootstrap.
type BootstrapResult struct {
	CompanyID  uuid.UUID
	AdminID    uuid.UUID
	UserID     uuid.UUID // employee row mirroring the admin (so they can be issued a virtual key)
	DefaultPolicyID uuid.UUID
}

// Bootstrap atomically inserts a company, the first admin, an employee row
// mirroring the admin, and a default DLP policy (mock strategy). Returns the
// IDs so the caller can sign a JWT immediately.
func (d *DB) Bootstrap(ctx context.Context, p BootstrapParams) (*BootstrapResult, error) {
	out := &BootstrapResult{}
	err := d.WithoutTenant(ctx, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `
			INSERT INTO companies (name, slug, dek_wrapped, kek_id)
			VALUES ($1, $2, $3, $4)
			RETURNING id
		`, p.CompanyName, p.CompanySlug, p.DEKWrapped, p.KEKID).Scan(&out.CompanyID); err != nil {
			return err
		}

		// Set tenant_id for the rest of the tx so RLS-protected inserts work.
		if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", out.CompanyID.String()); err != nil {
			return err
		}

		if err := tx.QueryRow(ctx, `
			INSERT INTO admins (company_id, email, password_hash, role)
			VALUES ($1, $2, $3, $4)
			RETURNING id
		`, out.CompanyID, p.AdminEmail, p.AdminPasswordH, p.AdminRole).Scan(&out.AdminID); err != nil {
			return err
		}

		// Mirror the admin as an employee row so they can receive a virtual key.
		name := p.AdminFullName
		if name == "" {
			name = p.AdminEmail
		}
		if err := tx.QueryRow(ctx, `
			INSERT INTO users (company_id, name, email, role, is_active)
			VALUES ($1, $2, $3, 'admin', true)
			RETURNING id
		`, out.CompanyID, name, p.AdminEmail).Scan(&out.UserID); err != nil {
			return err
		}

		// Default DLP policy: mock strategy + fail-closed; admin can swap to hybrid later.
		if err := tx.QueryRow(ctx, `
			INSERT INTO dlp_policies (company_id, name, strategy, config, fail_mode, is_default)
			VALUES ($1, 'Default', 'mock', '{}'::jsonb, 'closed', true)
			RETURNING id
		`, out.CompanyID).Scan(&out.DefaultPolicyID); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
