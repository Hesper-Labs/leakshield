package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// MasterProviderKey is a company-owned LLM provider key, stored
// envelope-encrypted under the tenant DEK.
type MasterProviderKey struct {
	ID             uuid.UUID
	CompanyID      uuid.UUID
	Provider       string
	Label          string
	APIKeyCipher   []byte
	APIKeyNonce    []byte
	Config         json.RawMessage
	IsActive       bool
	LastTestedAt   *time.Time
	LastTestStatus *string
	CreatedAt      time.Time
}

// CreateMasterProviderKeyParams holds inputs for the insert.
type CreateMasterProviderKeyParams struct {
	Provider     string
	Label        string
	APIKeyCipher []byte
	APIKeyNonce  []byte
	Config       json.RawMessage
}

// CreateMasterProviderKey inserts a new master provider key. The caller is
// expected to encrypt with the tenant DEK before calling.
func (d *DB) CreateMasterProviderKey(ctx context.Context, tenantID uuid.UUID, p CreateMasterProviderKeyParams) (*MasterProviderKey, error) {
	mk := &MasterProviderKey{}
	if len(p.Config) == 0 {
		p.Config = json.RawMessage("{}")
	}
	err := d.WithTenant(ctx, tenantID.String(), func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			INSERT INTO master_provider_keys (company_id, provider, label, api_key_cipher, api_key_nonce, config)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING id, company_id, provider, label, api_key_cipher, api_key_nonce, config,
			          is_active, last_tested_at, last_test_status, created_at
		`, tenantID, p.Provider, p.Label, p.APIKeyCipher, p.APIKeyNonce, p.Config).Scan(
			&mk.ID, &mk.CompanyID, &mk.Provider, &mk.Label, &mk.APIKeyCipher, &mk.APIKeyNonce,
			&mk.Config, &mk.IsActive, &mk.LastTestedAt, &mk.LastTestStatus, &mk.CreatedAt,
		)
	})
	if err != nil {
		return nil, err
	}
	return mk, nil
}

// FindActiveMasterKey returns the active master key for a tenant + provider.
func (d *DB) FindActiveMasterKey(ctx context.Context, tenantID uuid.UUID, provider string) (*MasterProviderKey, error) {
	mk := &MasterProviderKey{}
	err := d.WithTenant(ctx, tenantID.String(), func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			SELECT id, company_id, provider, label, api_key_cipher, api_key_nonce, config,
			       is_active, last_tested_at, last_test_status, created_at
			FROM master_provider_keys
			WHERE provider = $1 AND is_active
			ORDER BY created_at DESC
			LIMIT 1
		`, provider).Scan(
			&mk.ID, &mk.CompanyID, &mk.Provider, &mk.Label, &mk.APIKeyCipher, &mk.APIKeyNonce,
			&mk.Config, &mk.IsActive, &mk.LastTestedAt, &mk.LastTestStatus, &mk.CreatedAt,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	return mk, nil
}

// ListMasterProviderKeys lists the company's master keys (no plaintext).
func (d *DB) ListMasterProviderKeys(ctx context.Context, tenantID uuid.UUID) ([]*MasterProviderKey, error) {
	out := []*MasterProviderKey{}
	err := d.WithTenant(ctx, tenantID.String(), func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, company_id, provider, label, api_key_cipher, api_key_nonce, config,
			       is_active, last_tested_at, last_test_status, created_at
			FROM master_provider_keys
			ORDER BY created_at DESC
		`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			mk := &MasterProviderKey{}
			if err := rows.Scan(
				&mk.ID, &mk.CompanyID, &mk.Provider, &mk.Label, &mk.APIKeyCipher, &mk.APIKeyNonce,
				&mk.Config, &mk.IsActive, &mk.LastTestedAt, &mk.LastTestStatus, &mk.CreatedAt,
			); err != nil {
				return err
			}
			out = append(out, mk)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// MarkMasterKeyTested records a test-connection result.
func (d *DB) MarkMasterKeyTested(ctx context.Context, tenantID, id uuid.UUID, status string) error {
	return d.WithTenant(ctx, tenantID.String(), func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE master_provider_keys SET last_tested_at = now(), last_test_status = $2 WHERE id = $1`, id, status)
		return err
	})
}

// DeactivateMasterKey marks a master key inactive (used when rotating).
func (d *DB) DeactivateMasterKey(ctx context.Context, tenantID, id uuid.UUID) error {
	return d.WithTenant(ctx, tenantID.String(), func(tx pgx.Tx) error {
		ct, err := tx.Exec(ctx, `UPDATE master_provider_keys SET is_active = false WHERE id = $1`, id)
		if err != nil {
			return err
		}
		if ct.RowsAffected() == 0 {
			return ErrNotFound
		}
		return nil
	})
}
