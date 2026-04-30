package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Hesper-Labs/leakshield/gateway/internal/config"
	"github.com/Hesper-Labs/leakshield/gateway/internal/handlers"
)

// AdminServer is the internal-only admin API: company/user/key/policy CRUD,
// audit and analytics endpoints. It is exposed on a separate address so it
// can be locked down behind a VPN / SSO layer at the ingress.
type AdminServer struct {
	cfg    *config.Config
	logger *slog.Logger
	db     *pgxpool.Pool
	http   *http.Server
}

// NewAdmin constructs an admin server with placeholder routes.
func NewAdmin(ctx context.Context, cfg *config.Config, logger *slog.Logger) (*AdminServer, error) {
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	r.Get("/healthz", handlers.Healthz(pool))

	r.Route("/admin/v1", func(r chi.Router) {
		// Filled in by later phases:
		//   r.Post("/auth/login", ...)
		//   r.Post("/auth/bootstrap", ...)
		//   r.Route("/companies", ...)
		//   r.Route("/providers", ...)
		//   r.Route("/users", ...)
		//   r.Route("/keys", ...)
		//   r.Route("/policies", ...)
		//   r.Route("/audit", ...)
		//   r.Route("/analytics", ...)
		r.Get("/", handlers.NotImplemented("admin api"))
	})

	s := &AdminServer{
		cfg:    cfg,
		logger: logger,
		db:     pool,
		http: &http.Server{
			Addr:              cfg.AdminAddr,
			Handler:           r,
			ReadHeaderTimeout: 10 * time.Second,
		},
	}
	return s, nil
}

// Run starts the admin server and blocks until ctx is canceled.
func (s *AdminServer) Run(ctx context.Context) error {
	s.logger.Info("admin server starting", "addr", s.cfg.AdminAddr)
	errCh := make(chan error, 1)
	go func() {
		if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = s.http.Shutdown(shutCtx)
		s.db.Close()
		return nil
	case err := <-errCh:
		return err
	}
}
