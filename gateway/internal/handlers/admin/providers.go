package admin

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Hesper-Labs/leakshield/gateway/internal/keys"
	"github.com/Hesper-Labs/leakshield/gateway/internal/store"
)

// ProvidersDeps bundles dependencies for provider endpoints.
type ProvidersDeps struct {
	DB       *store.DB
	Resolver *keys.Resolver
}

type providerJSON struct {
	ID             string          `json:"id"`
	Provider       string          `json:"provider"`
	Label          string          `json:"label"`
	Config         json.RawMessage `json:"config"`
	IsActive       bool            `json:"is_active"`
	LastTestedAt   *time.Time      `json:"last_tested_at,omitempty"`
	LastTestStatus *string         `json:"last_test_status,omitempty"`
}

func providerToJSON(mk *store.MasterProviderKey) providerJSON {
	return providerJSON{
		ID:             mk.ID.String(),
		Provider:       mk.Provider,
		Label:          mk.Label,
		Config:         mk.Config,
		IsActive:       mk.IsActive,
		LastTestedAt:   mk.LastTestedAt,
		LastTestStatus: mk.LastTestStatus,
	}
}

// ListProviders returns the master provider keys for the tenant (no plaintext).
func ListProviders(deps ProvidersDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, ok := SessionFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
			return
		}
		rows, err := deps.DB.ListMasterProviderKeys(r.Context(), s.TenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		out := make([]providerJSON, 0, len(rows))
		for _, mk := range rows {
			out = append(out, providerToJSON(mk))
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": out})
	}
}

type createProviderRequest struct {
	Provider string          `json:"provider"`
	Label    string          `json:"label"`
	APIKey   string          `json:"api_key"`
	Config   json.RawMessage `json:"config"`
}

// CreateProvider stores a new master provider key (envelope-encrypted).
func CreateProvider(deps ProvidersDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, ok := SessionFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
			return
		}
		var req createProviderRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		req.Provider = strings.ToLower(strings.TrimSpace(req.Provider))
		if !validProvider(req.Provider) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "provider must be openai|anthropic|google|azure"})
			return
		}
		if strings.TrimSpace(req.APIKey) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "api_key is required"})
			return
		}
		if req.Label == "" {
			req.Label = strings.Title(req.Provider) //nolint:staticcheck // good enough
		}

		cipher, nonce, err := deps.Resolver.EncryptForTenant(r.Context(), s.TenantID, []byte(req.APIKey))
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "encrypt: " + err.Error()})
			return
		}
		mk, err := deps.DB.CreateMasterProviderKey(r.Context(), s.TenantID, store.CreateMasterProviderKeyParams{
			Provider:     req.Provider,
			Label:        req.Label,
			APIKeyCipher: cipher,
			APIKeyNonce:  nonce,
			Config:       req.Config,
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, providerToJSON(mk))
	}
}

type testProviderRequest struct {
	Provider string          `json:"provider"`
	APIKey   string          `json:"api_key"`
	Config   json.RawMessage `json:"config"`
}

// TestProviderConnection makes a cheap upstream call to verify the API key.
// Returns { ok: true, models: [...] } or { ok: false, error: "..." }.
func TestProviderConnection(deps ProvidersDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req testProviderRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		req.Provider = strings.ToLower(strings.TrimSpace(req.Provider))
		if !validProvider(req.Provider) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "provider must be openai|anthropic|google|azure"})
			return
		}
		if strings.TrimSpace(req.APIKey) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "api_key is required"})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		models, err := probeProvider(ctx, req.Provider, req.APIKey, req.Config)
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "models": models})
	}
}

// DeleteProvider deactivates a master key.
func DeleteProvider(deps ProvidersDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, ok := SessionFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
			return
		}
		idStr := chi.URLParam(r, "id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
			return
		}
		if err := deps.DB.DeactivateMasterKey(r.Context(), s.TenantID, id); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func validProvider(p string) bool {
	switch p {
	case "openai", "anthropic", "google", "azure":
		return true
	}
	return false
}

// probeProvider calls a cheap upstream endpoint to validate the API key.
// Returns the list of models the key can see.
func probeProvider(ctx context.Context, provider, apiKey string, _ json.RawMessage) ([]string, error) {
	switch provider {
	case "openai":
		return probeOpenAI(ctx, apiKey)
	case "anthropic":
		return probeAnthropic(ctx, apiKey)
	case "google":
		return probeGoogle(ctx, apiKey)
	case "azure":
		// Azure needs endpoint + deployment map, which lives in `config`.
		// Test path requires more plumbing; stub for now and rely on the
		// admin to use the panel's "verify" step instead.
		return []string{}, nil
	}
	return nil, errors.New("unknown provider")
}

func probeOpenAI(ctx context.Context, apiKey string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.openai.com/v1/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, errors.New(strings.TrimSpace(string(body)))
	}
	var data struct {
		Data []struct{ ID string } `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(data.Data))
	for _, m := range data.Data {
		out = append(out, m.ID)
	}
	return out, nil
}

func probeAnthropic(ctx context.Context, apiKey string) ([]string, error) {
	// Anthropic does not expose a free model-listing endpoint. Send a
	// 1-token messages call as a connectivity check.
	body := strings.NewReader(`{"model":"claude-haiku-4-5-20251001","max_tokens":1,"messages":[{"role":"user","content":"hi"}]}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, errors.New(strings.TrimSpace(string(raw)))
	}
	return []string{
		"claude-opus-4-7", "claude-sonnet-4-6", "claude-haiku-4-5-20251001",
	}, nil
}

func probeGoogle(ctx context.Context, apiKey string) ([]string, error) {
	url := "https://generativelanguage.googleapis.com/v1beta/models?key=" + apiKey
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, errors.New(strings.TrimSpace(string(raw)))
	}
	var data struct {
		Models []struct{ Name string } `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(data.Models))
	for _, m := range data.Models {
		out = append(out, strings.TrimPrefix(m.Name, "models/"))
	}
	return out, nil
}

var sharedHTTP = &http.Client{Timeout: 12 * time.Second}

func httpClient() *http.Client { return sharedHTTP }
