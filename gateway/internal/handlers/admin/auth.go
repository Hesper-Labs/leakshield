package admin

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"

	gwauth "github.com/Hesper-Labs/leakshield/gateway/internal/auth"
	"github.com/Hesper-Labs/leakshield/gateway/internal/jwt"
	"github.com/Hesper-Labs/leakshield/gateway/internal/keys"
	"github.com/Hesper-Labs/leakshield/gateway/internal/store"
)

// AuthDeps bundles everything the auth handlers need.
type AuthDeps struct {
	DB       *store.DB
	Resolver *keys.Resolver
	JWTSecret []byte
}

type setupStatusResponse struct {
	HasAdmin         bool `json:"has_admin"`
	AcceptsBootstrap bool `json:"accepts_bootstrap"`
}

// GetSetupStatus reports whether the install has been bootstrapped.
func GetSetupStatus(deps AuthDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		n, err := deps.DB.CountAdmins(r.Context())
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, setupStatusResponse{
			HasAdmin:         n > 0,
			AcceptsBootstrap: n == 0,
		})
	}
}

type bootstrapRequest struct {
	CompanyName string `json:"company_name"`
	CompanySlug string `json:"company_slug"`
	FullName    string `json:"full_name"`
	Email       string `json:"email"`
	Password    string `json:"password"`
}

type authResponse struct {
	Token string         `json:"token"`
	User  authResponseUser `json:"user"`
}

type authResponseUser struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Name     string `json:"name,omitempty"`
	TenantID string `json:"tenant_id"`
	Role     string `json:"role"`
}

// PostBootstrap creates the very first company + admin + default DLP policy.
// Refuses if any admin already exists.
func PostBootstrap(deps AuthDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		n, err := deps.DB.CountAdmins(r.Context())
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
			return
		}
		if n > 0 {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "an admin already exists; use sign-in"})
			return
		}

		var req bootstrapRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		req.Email = strings.TrimSpace(strings.ToLower(req.Email))
		req.CompanyName = strings.TrimSpace(req.CompanyName)
		req.FullName = strings.TrimSpace(req.FullName)
		if req.CompanySlug == "" {
			req.CompanySlug = slugify(req.CompanyName)
		}

		if errs := validateBootstrap(req); len(errs) > 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": strings.Join(errs, "; ")})
			return
		}

		hash, err := gwauth.HashPassword(req.Password)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		wrappedDEK, kekID, err := deps.Resolver.Bootstrap()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "kek wrap failed: " + err.Error()})
			return
		}

		out, err := deps.DB.Bootstrap(r.Context(), store.BootstrapParams{
			CompanyName:    req.CompanyName,
			CompanySlug:    req.CompanySlug,
			DEKWrapped:     wrappedDEK,
			KEKID:          kekID,
			AdminEmail:     req.Email,
			AdminPasswordH: hash,
			AdminRole:      "super_admin",
			AdminFullName:  req.FullName,
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db bootstrap: " + err.Error()})
			return
		}

		token, err := jwt.Sign(deps.JWTSecret, jwt.Claims{
			Subject:  out.AdminID.String(),
			TenantID: out.CompanyID.String(),
			Email:    req.Email,
			Name:     req.FullName,
			Role:     "super_admin",
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "sign token: " + err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, authResponse{
			Token: token,
			User: authResponseUser{
				ID:       out.AdminID.String(),
				Email:    req.Email,
				Name:     req.FullName,
				TenantID: out.CompanyID.String(),
				Role:     "super_admin",
			},
		})
	}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// PostLogin verifies credentials and returns a JWT.
func PostLogin(deps AuthDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		req.Email = strings.TrimSpace(strings.ToLower(req.Email))
		if req.Email == "" || req.Password == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email and password required"})
			return
		}

		admin, err := deps.DB.FindAdminByEmail(r.Context(), req.Email)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
			return
		}
		if !gwauth.VerifyPassword(req.Password, admin.PasswordHash) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
			return
		}
		_ = deps.DB.MarkAdminLogin(r.Context(), admin.ID)

		var tenantID string
		if admin.CompanyID != nil {
			tenantID = admin.CompanyID.String()
		}
		token, err := jwt.Sign(deps.JWTSecret, jwt.Claims{
			Subject:  admin.ID.String(),
			TenantID: tenantID,
			Email:    admin.Email,
			Role:     admin.Role,
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, authResponse{
			Token: token,
			User: authResponseUser{
				ID:       admin.ID.String(),
				Email:    admin.Email,
				TenantID: tenantID,
				Role:     admin.Role,
			},
		})
	}
}

// GetMe returns the authenticated admin profile.
func GetMe(deps AuthDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, ok := SessionFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
			return
		}
		var company *store.Company
		var err error
		if s.TenantID != uuid.Nil {
			company, err = deps.DB.FindCompanyByID(r.Context(), s.TenantID)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"user": authResponseUser{
				ID:       s.AdminID.String(),
				Email:    s.Email,
				Name:     s.Name,
				TenantID: s.TenantID.String(),
				Role:     s.Role,
			},
			"tenant": map[string]string{
				"id":   nilCompanyID(company),
				"name": nilCompanyName(company),
				"slug": nilCompanySlug(company),
			},
		})
	}
}

func nilCompanyID(c *store.Company) string {
	if c == nil {
		return ""
	}
	return c.ID.String()
}

func nilCompanyName(c *store.Company) string {
	if c == nil {
		return ""
	}
	return c.Name
}

func nilCompanySlug(c *store.Company) string {
	if c == nil {
		return ""
	}
	return c.Slug
}

func validateBootstrap(req bootstrapRequest) []string {
	errs := []string{}
	if len(req.CompanyName) < 2 {
		errs = append(errs, "company_name is required")
	}
	if !validSlug(req.CompanySlug) {
		errs = append(errs, "company_slug must be 2-48 chars, lowercase alnum and hyphens")
	}
	if len(req.FullName) < 2 {
		errs = append(errs, "full_name is required")
	}
	if !strings.Contains(req.Email, "@") {
		errs = append(errs, "valid email is required")
	}
	if len(req.Password) < 8 {
		errs = append(errs, "password must be at least 8 characters")
	}
	return errs
}

func validSlug(s string) bool {
	if len(s) < 2 || len(s) > 48 {
		return false
	}
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
			return false
		}
	}
	return true
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	out := strings.Builder{}
	prevHyphen := false
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			out.WriteRune(r)
			prevHyphen = false
		default:
			if !prevHyphen && out.Len() > 0 {
				out.WriteRune('-')
				prevHyphen = true
			}
		}
	}
	result := strings.Trim(out.String(), "-")
	if len(result) > 48 {
		result = result[:48]
	}
	if len(result) < 2 {
		result = "tenant"
	}
	return result
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
