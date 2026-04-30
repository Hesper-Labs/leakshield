package server

import (
	"context"
	"log/slog"
	"time"

	"github.com/Hesper-Labs/leakshield/gateway/internal/config"
)

// RunWorker drives the gateway's background jobs:
//   - daily usage_aggregates rollup
//   - audit_logs partition lifecycle (create next month, detach old)
//   - audit log hash-chain integrity verification (nightly)
//   - cold-tier export (Parquet to S3 / MinIO)
//
// This is a no-op tick logger until the per-job implementations land.
func RunWorker(ctx context.Context, cfg *config.Config, logger *slog.Logger) error {
	logger.Info("worker starting")
	t := time.NewTicker(1 * time.Hour)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			logger.Info("worker shutting down")
			return nil
		case <-t.C:
			logger.Debug("worker tick (TODO: usage rollup, partition mgmt, audit verify)")
		}
	}
}
