package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	gwauth "github.com/Hesper-Labs/leakshield/gateway/internal/auth"
	"github.com/Hesper-Labs/leakshield/gateway/internal/store"
)

// KeysDeps bundles dependencies for virtual-key endpoints.
type KeysDeps struct {
	DB *store.DB
}

type virtualKeyJSON struct {
	ID               string   `json:"id"`
	UserID           string   `json:"user_id"`
	Name             string   `json:"name"`
	Prefix           string   `json:"prefix"`
	AllowedProviders []string `json:"allowed_providers"`
	AllowedModels    []string `json:"allowed_models"`
	IsActive         bool     `json:"is_active"`
}

func vkToJSON(vk *store.VirtualKey) virtualKeyJSON {
	return virtualKeyJSON{
		ID:               vk.ID.String(),
		UserID:           vk.UserID.String(),
		Name:             vk.Name,
		Prefix:           vk.KeyPrefix,
		AllowedProviders: vk.AllowedProviders,
		AllowedModels:    vk.AllowedModels,
		IsActive:         vk.IsActive(),
	}
}

type createKeyRequest struct {
	Name                 string   `json:"name"`
	AllowedProviders     []string `json:"allowed_providers"`
	AllowedModels        []string `json:"allowed_models"`
	MonthlyTokenLimit    *int64   `json:"monthly_token_limit,omitempty"`
	MonthlyUSDMicroLimit *int64   `json:"monthly_usd_micro_limit,omitempty"`
}

// CreateUserKey issues a new virtual key for a user. Returns the plaintext
// key once; subsequent listings only expose the prefix.
func CreateUserKey(deps KeysDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, ok := SessionFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
			return
		}
		userIDStr := chi.URLParam(r, "id")
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user id"})
			return
		}
		var req createKeyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		if req.Name == "" {
			req.Name = "default"
		}
		if len(req.AllowedProviders) == 0 {
			req.AllowedProviders = []string{"openai", "anthropic", "google", "azure"}
		}

		plaintext, lookupPrefix, hash, err := gwauth.Generate("live")
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		vk, err := deps.DB.CreateVirtualKey(r.Context(), s.TenantID, store.CreateVirtualKeyParams{
			UserID:               userID,
			Name:                 req.Name,
			KeyPrefix:            lookupPrefix,
			KeyHash:              hash,
			AllowedProviders:     req.AllowedProviders,
			AllowedModels:        req.AllowedModels,
			MonthlyTokenLimit:    req.MonthlyTokenLimit,
			MonthlyUSDMicroLimit: req.MonthlyUSDMicroLimit,
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusCreated, map[string]any{
			"plaintext": plaintext,
			"key":       vkToJSON(vk),
		})
	}
}

// ListUserKeys returns the keys issued to a user.
func ListUserKeys(deps KeysDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, ok := SessionFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
			return
		}
		userIDStr := chi.URLParam(r, "id")
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user id"})
			return
		}
		rows, err := deps.DB.ListVirtualKeysByUser(r.Context(), s.TenantID, userID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		out := make([]virtualKeyJSON, 0, len(rows))
		for _, vk := range rows {
			out = append(out, vkToJSON(vk))
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": out})
	}
}

// RevokeKey marks a virtual key revoked.
func RevokeKey(deps KeysDeps) http.HandlerFunc {
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
		if err := deps.DB.RevokeVirtualKey(r.Context(), s.TenantID, id); err != nil {
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

// Touch keeps the strings import warning silent if unused in some builds.
var _ = strings.TrimSpace
