package admin

import (
	"github.com/go-chi/chi/v5"

	"github.com/Hesper-Labs/leakshield/gateway/internal/keys"
	"github.com/Hesper-Labs/leakshield/gateway/internal/store"
)

// MountDeps bundles everything the admin routes need.
type MountDeps struct {
	DB        *store.DB
	Resolver  *keys.Resolver
	JWTSecret []byte
}

// Mount attaches the admin REST endpoints under /admin/v1.
//
// The router contract is: setup-status, auth/bootstrap, and auth/login are
// public (no JWT required); everything else sits behind JWTMiddleware.
func Mount(r chi.Router, deps MountDeps) {
	authDeps := AuthDeps{DB: deps.DB, Resolver: deps.Resolver, JWTSecret: deps.JWTSecret}
	provDeps := ProvidersDeps{DB: deps.DB, Resolver: deps.Resolver}
	userDeps := UsersDeps{DB: deps.DB}
	keyDeps := KeysDeps{DB: deps.DB}

	r.Route("/admin/v1", func(r chi.Router) {
		// Public endpoints.
		r.Get("/setup/status", GetSetupStatus(authDeps))
		r.Post("/auth/bootstrap", PostBootstrap(authDeps))
		r.Post("/auth/login", PostLogin(authDeps))

		// Authenticated endpoints.
		r.Group(func(r chi.Router) {
			r.Use(JWTMiddleware(deps.JWTSecret))

			r.Get("/me", GetMe(authDeps))

			r.Get("/providers", ListProviders(provDeps))
			r.Post("/providers", CreateProvider(provDeps))
			r.Post("/providers/test", TestProviderConnection(provDeps))
			r.Delete("/providers/{id}", DeleteProvider(provDeps))

			r.Get("/users", ListUsers(userDeps))
			r.Post("/users", CreateUser(userDeps))
			r.Get("/users/{id}", GetUser(userDeps))
			r.Get("/users/{id}/keys", ListUserKeys(keyDeps))
			r.Post("/users/{id}/keys", CreateUserKey(keyDeps))
			r.Delete("/keys/{id}", RevokeKey(keyDeps))
		})
	})
}
