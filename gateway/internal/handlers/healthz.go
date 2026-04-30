// Package handlers contains the HTTP handlers wired into the gateway's
// chi router.
package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type healthResponse struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks,omitempty"`
}

// Healthz is a liveness probe — returns 200 as long as the process is up.
func Healthz(_ *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
	}
}

// Readyz is a readiness probe — returns 200 when downstream dependencies
// (currently just Postgres) are reachable.
func Readyz(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		checks := map[string]string{}
		status := http.StatusOK
		if err := db.Ping(ctx); err != nil {
			checks["database"] = "fail: " + err.Error()
			status = http.StatusServiceUnavailable
		} else {
			checks["database"] = "ok"
		}
		writeJSON(w, status, healthResponse{
			Status: statusFromCode(status),
			Checks: checks,
		})
	}
}

// NotImplemented is a placeholder for endpoints that exist in the route
// table but have no handler yet.
func NotImplemented(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotImplemented, map[string]string{
			"error":   "not_implemented",
			"message": name + " is not implemented yet",
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func statusFromCode(code int) string {
	if code >= 200 && code < 300 {
		return "ok"
	}
	return "degraded"
}
