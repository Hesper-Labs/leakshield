// Package keys resolves the plaintext master provider key for a tenant + provider.
//
// The resolver wraps two concerns:
//   1. Per-tenant DEK unwrapping (cached in memory with a TTL to bound damage
//      after a process compromise).
//   2. Per-key plaintext decryption.
package keys

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/Hesper-Labs/leakshield/gateway/internal/crypto"
	"github.com/Hesper-Labs/leakshield/gateway/internal/store"
)

// Resolver decrypts master provider keys for a tenant.
type Resolver struct {
	db          *store.DB
	kek         crypto.KEKProvider
	dekCacheTTL time.Duration

	mu    sync.RWMutex
	dek   map[uuid.UUID]dekEntry
}

type dekEntry struct {
	key       []byte
	expiresAt time.Time
}

// NewResolver constructs a Resolver. ttl bounds how long an unwrapped DEK
// stays in memory.
func NewResolver(db *store.DB, kek crypto.KEKProvider, ttl time.Duration) *Resolver {
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	return &Resolver{
		db:          db,
		kek:         kek,
		dekCacheTTL: ttl,
		dek:         make(map[uuid.UUID]dekEntry),
	}
}

// MasterKey returns the plaintext master provider key for a tenant + provider.
func (r *Resolver) MasterKey(ctx context.Context, tenantID uuid.UUID, provider string) (string, *store.MasterProviderKey, error) {
	dek, err := r.dekFor(ctx, tenantID)
	if err != nil {
		return "", nil, err
	}
	mk, err := r.db.FindActiveMasterKey(ctx, tenantID, provider)
	if err != nil {
		return "", nil, err
	}
	plaintext, err := crypto.DecryptWithDEK(dek, append(append([]byte{}, mk.APIKeyNonce...), mk.APIKeyCipher...))
	if err != nil {
		return "", nil, fmt.Errorf("decrypt master key: %w", err)
	}
	return string(plaintext), mk, nil
}

// EncryptForTenant encrypts plaintext under the tenant DEK. Used by the
// admin handler when creating master provider keys.
func (r *Resolver) EncryptForTenant(ctx context.Context, tenantID uuid.UUID, plaintext []byte) (cipher, nonce []byte, err error) {
	dek, err := r.dekFor(ctx, tenantID)
	if err != nil {
		return nil, nil, err
	}
	blob, err := crypto.EncryptWithDEK(dek, plaintext)
	if err != nil {
		return nil, nil, err
	}
	// Convention used elsewhere: nonce (12) || ciphertext.
	return blob[12:], blob[:12], nil
}

// Bootstrap creates a per-tenant DEK, wraps it with the KEK, and returns
// (wrapped_dek, kek_id) so the caller can persist them on the company row.
func (r *Resolver) Bootstrap() (wrappedDEK []byte, kekID string, err error) {
	dek, err := crypto.GenerateDEK()
	if err != nil {
		return nil, "", err
	}
	wrappedDEK, err = r.kek.WrapDEK(dek)
	if err != nil {
		return nil, "", err
	}
	return wrappedDEK, r.kek.ID(), nil
}

func (r *Resolver) dekFor(ctx context.Context, tenantID uuid.UUID) ([]byte, error) {
	r.mu.RLock()
	entry, ok := r.dek[tenantID]
	r.mu.RUnlock()
	if ok && time.Now().Before(entry.expiresAt) {
		return entry.key, nil
	}

	c, err := r.db.FindCompanyByID(ctx, tenantID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, fmt.Errorf("tenant %s not found", tenantID)
		}
		return nil, err
	}
	dek, err := r.kek.UnwrapDEK(c.DEKWrapped)
	if err != nil {
		return nil, fmt.Errorf("unwrap dek: %w", err)
	}
	r.mu.Lock()
	r.dek[tenantID] = dekEntry{key: dek, expiresAt: time.Now().Add(r.dekCacheTTL)}
	r.mu.Unlock()
	return dek, nil
}

// Invalidate drops the cached DEK for a tenant. Called after key rotation.
func (r *Resolver) Invalidate(tenantID uuid.UUID) {
	r.mu.Lock()
	delete(r.dek, tenantID)
	r.mu.Unlock()
}
