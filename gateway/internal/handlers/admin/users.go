package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Hesper-Labs/leakshield/gateway/internal/store"
)

// UsersDeps bundles dependencies for user endpoints.
type UsersDeps struct {
	DB *store.DB
}

type userJSON struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Surname    string `json:"surname,omitempty"`
	Email      string `json:"email"`
	Phone      string `json:"phone,omitempty"`
	Department string `json:"department,omitempty"`
	Role       string `json:"role"`
	IsActive   bool   `json:"is_active"`
}

func userToJSON(u *store.User) userJSON {
	out := userJSON{
		ID:       u.ID.String(),
		Name:     u.Name,
		Email:    u.Email,
		Role:     u.Role,
		IsActive: u.IsActive,
	}
	if u.Surname != nil {
		out.Surname = *u.Surname
	}
	if u.Phone != nil {
		out.Phone = *u.Phone
	}
	if u.Department != nil {
		out.Department = *u.Department
	}
	return out
}

// ListUsers returns the company's users.
func ListUsers(deps UsersDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, ok := SessionFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
			return
		}
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		rows, err := deps.DB.ListUsers(r.Context(), s.TenantID, limit, offset)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		out := make([]userJSON, 0, len(rows))
		for _, u := range rows {
			out = append(out, userToJSON(u))
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": out})
	}
}

type createUserRequest struct {
	Name       string `json:"name"`
	Surname    string `json:"surname"`
	Email      string `json:"email"`
	Phone      string `json:"phone"`
	Department string `json:"department"`
	Role       string `json:"role"`
}

// CreateUser inserts a new employee.
func CreateUser(deps UsersDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, ok := SessionFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
			return
		}
		var req createUserRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		req.Email = strings.TrimSpace(strings.ToLower(req.Email))
		req.Name = strings.TrimSpace(req.Name)
		if req.Name == "" || !strings.Contains(req.Email, "@") {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name and valid email required"})
			return
		}
		if req.Role == "" {
			req.Role = "member"
		}
		u, err := deps.DB.CreateUser(r.Context(), s.TenantID, store.CreateUserParams{
			Name:       req.Name,
			Surname:    req.Surname,
			Email:      req.Email,
			Phone:      req.Phone,
			Department: req.Department,
			Role:       req.Role,
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, userToJSON(u))
	}
}

// GetUser loads a user by id.
func GetUser(deps UsersDeps) http.HandlerFunc {
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
		u, err := deps.DB.FindUserByID(r.Context(), s.TenantID, id)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, userToJSON(u))
	}
}
