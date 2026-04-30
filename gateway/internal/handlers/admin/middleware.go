// Package admin contains the admin REST API handlers (panel-facing).
package admin

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/Hesper-Labs/leakshield/gateway/internal/jwt"
)

// SessionContext describes the authenticated admin attached by AuthMiddleware.
type SessionContext struct {
	AdminID  uuid.UUID
	TenantID uuid.UUID
	Email    string
	Role     string
	Name     string
}

type ctxKey struct{}

var sessionCtxKey = ctxKey{}

// SessionFromContext returns the authenticated session, if present.
func SessionFromContext(ctx context.Context) (*SessionContext, bool) {
	s, ok := ctx.Value(sessionCtxKey).(*SessionContext)
	return s, ok
}

// JWTMiddleware verifies the Authorization: Bearer <jwt> header and attaches
// a SessionContext to the request context. Endpoints that don't need auth
// (setup/status, auth/bootstrap, auth/login) skip this middleware entirely.
func JWTMiddleware(secret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("Authorization")
			if !strings.HasPrefix(h, "Bearer ") {
				http.Error(w, "missing bearer token", http.StatusUnauthorized)
				return
			}
			tok := strings.TrimPrefix(h, "Bearer ")
			claims, err := jwt.Verify(secret, tok)
			if err != nil {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}
			adminID, err := uuid.Parse(claims.Subject)
			if err != nil {
				http.Error(w, "invalid token subject", http.StatusUnauthorized)
				return
			}
			tenantID, err := uuid.Parse(claims.TenantID)
			if err != nil {
				http.Error(w, "invalid token tenant", http.StatusUnauthorized)
				return
			}
			s := &SessionContext{
				AdminID:  adminID,
				TenantID: tenantID,
				Email:    claims.Email,
				Role:     claims.Role,
				Name:     claims.Name,
			}
			ctx := context.WithValue(r.Context(), sessionCtxKey, s)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole checks that the session role is in the allowed set.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s, ok := SessionFromContext(r.Context())
			if !ok {
				http.Error(w, "unauthenticated", http.StatusUnauthorized)
				return
			}
			for _, want := range roles {
				if s.Role == want {
					next.ServeHTTP(w, r)
					return
				}
			}
			http.Error(w, "forbidden", http.StatusForbidden)
		})
	}
}
