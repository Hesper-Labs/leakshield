// Package observability sets up logging, tracing, and metrics for the gateway.
package observability

import (
	"log/slog"
	"os"
)

// SetupLogger returns a JSON structured slog logger.
//
// TODO(phase1-hardening): wrap with a redacting handler that scrubs secret
// patterns from attributes (Authorization, x-api-key, sk-*, gw_live_*).
func SetupLogger(level slog.Level) *slog.Logger {
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: level <= slog.LevelDebug,
	}
	h := slog.NewJSONHandler(os.Stdout, opts)
	return slog.New(h)
}
