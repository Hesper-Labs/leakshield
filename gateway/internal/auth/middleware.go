package auth

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/Hesper-Labs/leakshield/gateway/internal/store"
)

// VirtualKeyContext is what the chat handler reads after middleware runs.
type VirtualKeyContext struct {
	KeyID            uuid.UUID
	TenantID         uuid.UUID
	UserID           uuid.UUID
	AllowedProviders []string
	AllowedModels    []string
}

type ctxKey struct{}

var virtualKeyCtxKey = ctxKey{}

// FromContext retrieves the virtual key context attached by VirtualKeyMiddleware.
func FromContext(ctx context.Context) (*VirtualKeyContext, bool) {
	v, ok := ctx.Value(virtualKeyCtxKey).(*VirtualKeyContext)
	return v, ok
}

// VirtualKeyVerifier resolves a presented virtual key to a tenant + user.
// Implementations cache verified keys for a short TTL because argon2id is
// intentionally slow.
type VirtualKeyVerifier struct {
	db        *store.DB
	cacheTTL  time.Duration
	negTTL    time.Duration

	mu       sync.RWMutex
	verified map[string]verifiedEntry
}

type verifiedEntry struct {
	ctx       *VirtualKeyContext
	hashedSecret string // sha256 of the secret we verified, for cache safety
	expiresAt time.Time
	notFound  bool
}

// NewVirtualKeyVerifier constructs a verifier with sensible defaults.
func NewVirtualKeyVerifier(db *store.DB) *VirtualKeyVerifier {
	return &VirtualKeyVerifier{
		db:       db,
		cacheTTL: 60 * time.Second,
		negTTL:   5 * time.Second,
		verified: make(map[string]verifiedEntry),
	}
}

// Verify returns the resolved context for a virtual key, or an error.
func (v *VirtualKeyVerifier) Verify(ctx context.Context, presented string) (*VirtualKeyContext, error) {
	parsed, err := Parse(presented)
	if err != nil {
		return nil, err
	}

	// Cache lookup: hashed presented secret to avoid using the secret itself
	// as a map key (defence in depth).
	cacheKey := parsed.LookupPrefix
	v.mu.RLock()
	entry, ok := v.verified[cacheKey]
	v.mu.RUnlock()
	if ok && time.Now().Before(entry.expiresAt) {
		if entry.notFound {
			return nil, store.ErrNotFound
		}
		// Compare stored secret hash to make sure the same secret produced
		// this entry. Different secrets with the same prefix would only
		// happen on a brute-force enumeration attempt; cheap to defend.
		if entry.hashedSecret == hashSecret(parsed.Secret) {
			return entry.ctx, nil
		}
	}

	row, err := v.db.FindVirtualKeyByPrefix(ctx, parsed.LookupPrefix)
	if err != nil {
		v.cacheNotFound(cacheKey)
		return nil, err
	}
	if !VerifySecret(parsed.Secret, row.KeyHash) {
		v.cacheNotFound(cacheKey)
		return nil, store.ErrNotFound
	}
	out := &VirtualKeyContext{
		KeyID:            row.ID,
		TenantID:         row.CompanyID,
		UserID:           row.UserID,
		AllowedProviders: row.AllowedProviders,
		AllowedModels:    row.AllowedModels,
	}
	v.mu.Lock()
	v.verified[cacheKey] = verifiedEntry{
		ctx:          out,
		hashedSecret: hashSecret(parsed.Secret),
		expiresAt:    time.Now().Add(v.cacheTTL),
	}
	v.mu.Unlock()
	// Best-effort last-used update (don't block the hot path).
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = v.db.MarkVirtualKeyUsed(bgCtx, row.ID)
	}()
	return out, nil
}

func (v *VirtualKeyVerifier) cacheNotFound(prefix string) {
	v.mu.Lock()
	v.verified[prefix] = verifiedEntry{
		expiresAt: time.Now().Add(v.negTTL),
		notFound:  true,
	}
	v.mu.Unlock()
}

// Invalidate removes a key (any prefix matches) from the cache.
func (v *VirtualKeyVerifier) Invalidate(prefix string) {
	v.mu.Lock()
	delete(v.verified, prefix)
	v.mu.Unlock()
}

// VirtualKeyMiddleware extracts a key from the request and attaches the
// resolved context. Different providers carry the key in different headers:
//
//	OpenAI:    Authorization: Bearer <key>
//	Anthropic: x-api-key: <key>
//	Google:    ?key=<key> (query param) OR Authorization: Bearer
//	Azure:     api-key: <key>
//
// We accept all of them and the universal /v1/* path uses Authorization.
func (v *VirtualKeyVerifier) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			presented := extractKey(r)
			if presented == "" {
				http.Error(w, "missing virtual key", http.StatusUnauthorized)
				return
			}
			vctx, err := v.Verify(r.Context(), presented)
			if err != nil {
				http.Error(w, "invalid virtual key", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), virtualKeyCtxKey, vctx)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func extractKey(r *http.Request) string {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	if h := r.Header.Get("x-api-key"); h != "" {
		return h
	}
	if h := r.Header.Get("api-key"); h != "" {
		return h
	}
	if q := r.URL.Query().Get("key"); q != "" {
		return q
	}
	return ""
}

func hashSecret(s string) string {
	// Cheap, non-cryptographic: we just want a stable token to detect cache
	// collisions on the SAME prefix from different secrets.
	h := uint64(1469598103934665603)
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return uintHex(h)
}

func uintHex(n uint64) string {
	const hex = "0123456789abcdef"
	out := [16]byte{}
	for i := 15; i >= 0; i-- {
		out[i] = hex[n&0xF]
		n >>= 4
	}
	return string(out[:])
}
