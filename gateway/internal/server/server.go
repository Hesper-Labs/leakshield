// Package server wires up the gateway HTTP servers, the worker loop, and
// the migration entry point.
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

// Server is the public-facing gateway: proxy endpoints + health checks.
type Server struct {
	cfg    *config.Config
	logger *slog.Logger
	db     *pgxpool.Pool
	http   *http.Server
}

// New constructs a public gateway server with all routes registered.
// The provider proxy handlers are placeholders until the per-provider
// adapters land in subsequent phases.
func New(ctx context.Context, cfg *config.Config, logger *slog.Logger) (*Server, error) {
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(120 * time.Second))

	// Public health endpoints.
	r.Get("/healthz", handlers.Healthz(pool))
	r.Get("/readyz", handlers.Readyz(pool))

	// Provider native endpoints (filled in by phases 2-4).
	r.Route("/openai/v1", func(r chi.Router) {
		r.Post("/chat/completions", handlers.NotImplemented("openai chat completions"))
		r.Post("/embeddings", handlers.NotImplemented("openai embeddings"))
		r.Get("/models", handlers.NotImplemented("openai models"))
	})
	r.Route("/anthropic/v1", func(r chi.Router) {
		r.Post("/messages", handlers.NotImplemented("anthropic messages"))
	})
	r.Route("/google/v1beta", func(r chi.Router) {
		r.Post("/models/*", handlers.NotImplemented("google generateContent"))
	})
	r.Route("/azure/openai", func(r chi.Router) {
		r.Post("/deployments/*", handlers.NotImplemented("azure deployments"))
	})

	// Universal OpenAI-compatible router (LiteLLM-style, optional).
	r.Route("/v1", func(r chi.Router) {
		r.Post("/chat/completions", handlers.NotImplemented("universal router chat"))
	})

	s := &Server{
		cfg:    cfg,
		logger: logger,
		db:     pool,
		http: &http.Server{
			Addr:              cfg.HTTPAddr,
			Handler:           r,
			ReadHeaderTimeout: 10 * time.Second,
		},
	}
	return s, nil
}

// Run starts the HTTP server and blocks until ctx is canceled.
func (s *Server) Run(ctx context.Context) error {
	s.logger.Info("gateway starting", "addr", s.cfg.HTTPAddr)
	errCh := make(chan error, 1)
	go func() {
		if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		s.logger.Info("shutdown signal received")
		shutCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.http.Shutdown(shutCtx); err != nil {
			return err
		}
		s.db.Close()
		return nil
	case err := <-errCh:
		return err
	}
}
