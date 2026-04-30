package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// VirtualKey is the per-employee gateway-issued credential.
type VirtualKey struct {
	ID                   uuid.UUID
	CompanyID            uuid.UUID
	UserID               uuid.UUID
	Name                 string
	KeyPrefix            string
	KeyHash              []byte
	AllowedProviders     []string
	AllowedModels        []string
	RPMLimit             *int32
	TPMLimit             *int32
	MonthlyTokenLimit    *int64
	MonthlyUSDMicroLimit *int64
	ExpiresAt            *time.Time
	RevokedAt            *time.Time
	LastUsedAt           *time.Time
	IsActive             bool
	CreatedAt            time.Time
}

// CreateVirtualKeyParams holds the inputs for issuing a key.
type CreateVirtualKeyParams struct {
	UserID               uuid.UUID
	Name                 string
	KeyPrefix            string
	KeyHash              []byte
	AllowedProviders     []string
	AllowedModels        []string
	RPMLimit             *int32
	TPMLimit             *int32
	MonthlyTokenLimit    *int64
	MonthlyUSDMicroLimit *int64
	ExpiresAt            *time.Time
}

// CreateVirtualKey inserts a virtual key row.
func (d *DB) CreateVirtualKey(ctx context.Context, tenantID uuid.UUID, p CreateVirtualKeyParams) (*VirtualKey, error) {
	vk := &VirtualKey{}
	err := d.WithTenant(ctx, tenantID.String(), func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			INSERT INTO virtual_keys (
				company_id, user_id, name, key_prefix, key_hash,
				allowed_providers, allowed_models, rpm_limit, tpm_limit,
				monthly_token_limit, monthly_usd_micro_limit, expires_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			RETURNING id, company_id, user_id, name, key_prefix, key_hash,
			          allowed_providers, allowed_models, rpm_limit, tpm_limit,
			          monthly_token_limit, monthly_usd_micro_limit,
			          expires_at, revoked_at, last_used_at, is_active, created_at
		`,
			tenantID, p.UserID, p.Name, p.KeyPrefix, p.KeyHash,
			p.AllowedProviders, p.AllowedModels,
			p.RPMLimit, p.TPMLimit,
			p.MonthlyTokenLimit, p.MonthlyUSDMicroLimit,
			p.ExpiresAt,
		).Scan(
			&vk.ID, &vk.CompanyID, &vk.UserID, &vk.Name, &vk.KeyPrefix, &vk.KeyHash,
			&vk.AllowedProviders, &vk.AllowedModels, &vk.RPMLimit, &vk.TPMLimit,
			&vk.MonthlyTokenLimit, &vk.MonthlyUSDMicroLimit,
			&vk.ExpiresAt, &vk.RevokedAt, &vk.LastUsedAt, &vk.IsActive, &vk.CreatedAt,
		)
	})
	if err != nil {
		return nil, err
	}
	return vk, nil
}

// FindVirtualKeyByPrefix loads an active virtual key by its lookup prefix
// across tenants. The hot-path proxy auth uses this; RLS does not apply
// because the prefix is globally unique and the caller doesn't have a
// tenant context yet (that's exactly what we're trying to discover).
func (d *DB) FindVirtualKeyByPrefix(ctx context.Context, prefix string) (*VirtualKey, error) {
	vk := &VirtualKey{}
	err := d.Pool.QueryRow(ctx, `
		SELECT id, company_id, user_id, name, key_prefix, key_hash,
		       allowed_providers, allowed_models, rpm_limit, tpm_limit,
		       monthly_token_limit, monthly_usd_micro_limit,
		       expires_at, revoked_at, last_used_at, is_active, created_at
		FROM virtual_keys
		WHERE key_prefix = $1 AND revoked_at IS NULL
	`, prefix).Scan(
		&vk.ID, &vk.CompanyID, &vk.UserID, &vk.Name, &vk.KeyPrefix, &vk.KeyHash,
		&vk.AllowedProviders, &vk.AllowedModels, &vk.RPMLimit, &vk.TPMLimit,
		&vk.MonthlyTokenLimit, &vk.MonthlyUSDMicroLimit,
		&vk.ExpiresAt, &vk.RevokedAt, &vk.LastUsedAt, &vk.IsActive, &vk.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return vk, nil
}

// MarkVirtualKeyUsed updates last_used_at; called from the audit-log writer
// hot path as a best-effort, fire-and-forget update.
func (d *DB) MarkVirtualKeyUsed(ctx context.Context, id uuid.UUID) error {
	_, err := d.Pool.Exec(ctx, "UPDATE virtual_keys SET last_used_at = now() WHERE id = $1", id)
	return err
}

// ListVirtualKeysByUser returns the virtual keys for a user inside the tenant context.
func (d *DB) ListVirtualKeysByUser(ctx context.Context, tenantID, userID uuid.UUID) ([]*VirtualKey, error) {
	out := []*VirtualKey{}
	err := d.WithTenant(ctx, tenantID.String(), func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, company_id, user_id, name, key_prefix, key_hash,
			       allowed_providers, allowed_models, rpm_limit, tpm_limit,
			       monthly_token_limit, monthly_usd_micro_limit,
			       expires_at, revoked_at, last_used_at, is_active, created_at
			FROM virtual_keys
			WHERE user_id = $1
			ORDER BY created_at DESC
		`, userID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			vk := &VirtualKey{}
			if err := rows.Scan(
				&vk.ID, &vk.CompanyID, &vk.UserID, &vk.Name, &vk.KeyPrefix, &vk.KeyHash,
				&vk.AllowedProviders, &vk.AllowedModels, &vk.RPMLimit, &vk.TPMLimit,
				&vk.MonthlyTokenLimit, &vk.MonthlyUSDMicroLimit,
				&vk.ExpiresAt, &vk.RevokedAt, &vk.LastUsedAt, &vk.IsActive, &vk.CreatedAt,
			); err != nil {
				return err
			}
			out = append(out, vk)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// RevokeVirtualKey marks the key as revoked.
func (d *DB) RevokeVirtualKey(ctx context.Context, tenantID, id uuid.UUID) error {
	return d.WithTenant(ctx, tenantID.String(), func(tx pgx.Tx) error {
		ct, err := tx.Exec(ctx, `UPDATE virtual_keys SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL`, id)
		if err != nil {
			return err
		}
		if ct.RowsAffected() == 0 {
			return ErrNotFound
		}
		return nil
	})
}
