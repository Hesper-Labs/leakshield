// Package config loads gateway configuration from environment variables
// (all prefixed LEAKSHIELD_*).
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds the resolved runtime configuration for the gateway.
type Config struct {
	HTTPAddr      string
	AdminAddr     string
	DatabaseURL   string
	RedisURL      string
	InspectorAddr string

	KEKFile      string // local-dev fallback path
	KEKVaultPath string // Vault Transit path (production)
	KMSProvider  string // "vault" | "aws" | "gcp" | "azure" | "local" (default)

	LogLevel  slog.Level
	LogFormat string // "json" | "text"

	OTLPEndpoint   string
	PrometheusAddr string

	ProdMode bool

	InspectorTimeout     time.Duration
	InspectorMaxInflight int

	RateLimitDefaultRPM int
}

// Load reads configuration from the environment and returns a validated Config.
func Load() (*Config, error) {
	c := &Config{
		HTTPAddr:             getenv("LEAKSHIELD_HTTP_ADDR", "0.0.0.0:8080"),
		AdminAddr:            getenv("LEAKSHIELD_ADMIN_ADDR", "0.0.0.0:8090"),
		DatabaseURL:          os.Getenv("LEAKSHIELD_DATABASE_URL"),
		RedisURL:             getenv("LEAKSHIELD_REDIS_URL", "redis://localhost:6379/0"),
		InspectorAddr:        getenv("LEAKSHIELD_INSPECTOR_ADDR", "localhost:50051"),
		KEKFile:              getenv("LEAKSHIELD_KEK_FILE", ""),
		KEKVaultPath:         os.Getenv("LEAKSHIELD_KEK_VAULT_PATH"),
		KMSProvider:          getenv("LEAKSHIELD_KMS_PROVIDER", "local"),
		LogFormat:            getenv("LEAKSHIELD_LOG_FORMAT", "json"),
		OTLPEndpoint:         os.Getenv("LEAKSHIELD_OTLP_ENDPOINT"),
		PrometheusAddr:       getenv("LEAKSHIELD_PROMETHEUS_ADDR", "0.0.0.0:9090"),
		ProdMode:             getenvBool("LEAKSHIELD_PROD", false),
		InspectorTimeout:     getenvDuration("LEAKSHIELD_INSPECTOR_TIMEOUT", 2*time.Second),
		InspectorMaxInflight: getenvInt("LEAKSHIELD_INSPECTOR_MAX_INFLIGHT", 64),
		RateLimitDefaultRPM:  getenvInt("LEAKSHIELD_RATELIMIT_DEFAULT_RPM", 600),
	}

	c.LogLevel = parseLogLevel(getenv("LEAKSHIELD_LOG_LEVEL", "info"))

	if err := c.Validate(); err != nil {
		return nil, err
	}
	return c, nil
}

// Validate checks that required fields are present and that production mode
// has been configured with a real KMS rather than a local file.
func (c *Config) Validate() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("LEAKSHIELD_DATABASE_URL is required")
	}
	if c.ProdMode && c.KMSProvider == "local" && c.KEKFile == "" {
		return fmt.Errorf("--prod mode requires LEAKSHIELD_KMS_PROVIDER (vault|aws|gcp|azure) or an explicit KEK_FILE")
	}
	return nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getenvBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		return v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
	}
	return def
}

func getenvDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func parseLogLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
