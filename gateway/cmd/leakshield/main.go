// Command leakshield is the LeakShield gateway binary. It exposes
// subcommands for the public proxy server, the internal admin API, the
// background worker, and database migrations.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/Hesper-Labs/leakshield/gateway/internal/config"
	"github.com/Hesper-Labs/leakshield/gateway/internal/observability"
	"github.com/Hesper-Labs/leakshield/gateway/internal/server"
)

var version = "dev"

func main() {
	root := &cobra.Command{
		Use:   "leakshield",
		Short: "LeakShield AI Gateway + DLP",
	}

	root.AddCommand(serveCmd())
	root.AddCommand(adminCmd())
	root.AddCommand(workerCmd())
	root.AddCommand(migrateCmd())
	root.AddCommand(versionCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func serveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run the public gateway HTTP server (proxy + DLP)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("config: %w", err)
			}
			logger := observability.SetupLogger(cfg.LogLevel)
			slog.SetDefault(logger)

			ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer cancel()

			s, err := server.New(ctx, cfg, logger)
			if err != nil {
				return fmt.Errorf("server init: %w", err)
			}
			return s.Run(ctx)
		},
	}
}

func adminCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "admin",
		Short: "Run the internal admin REST API",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			logger := observability.SetupLogger(cfg.LogLevel)
			slog.SetDefault(logger)
			ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer cancel()
			s, err := server.NewAdmin(ctx, cfg, logger)
			if err != nil {
				return err
			}
			return s.Run(ctx)
		},
	}
}

func workerCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "worker",
		Short: "Run background workers (usage rollups, retention, audit verification)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			logger := observability.SetupLogger(cfg.LogLevel)
			slog.SetDefault(logger)
			ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer cancel()
			return server.RunWorker(ctx, cfg, logger)
		},
	}
}

func migrateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "migrate [up|down|status|reset]",
		Short: "Run database migrations (goose)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			return server.RunMigrate(context.Background(), cfg, args[0])
		},
	}
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Println("leakshield", version)
		},
	}
}
