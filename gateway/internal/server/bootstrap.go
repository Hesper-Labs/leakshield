package server

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/Hesper-Labs/leakshield/gateway/internal/config"
	"github.com/Hesper-Labs/leakshield/gateway/internal/crypto"
)

// initKEK returns a KEK provider resolved from config.
//
// In production the KEK comes from Vault / AWS KMS / GCP KMS / Azure Key
// Vault. For local-dev a 0600 file is used (auto-generated on first run if
// missing). --prod mode refuses to fall back to local files unless an
// explicit path was given.
func initKEK(cfg *config.Config, logger *slog.Logger) (crypto.KEKProvider, error) {
	switch cfg.KMSProvider {
	case "vault", "aws", "gcp", "azure":
		return nil, fmt.Errorf("KMS provider %q is not yet wired in this build (Track D follow-up); use local in dev", cfg.KMSProvider)
	case "local", "":
		path := cfg.KEKFile
		if path == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, err
			}
			path = filepath.Join(home, ".leakshield", "kek")
		}
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			if cfg.ProdMode {
				return nil, fmt.Errorf("--prod mode requires an existing KEK at %s", path)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				return nil, err
			}
			if err := crypto.GenerateAndWriteKEK(path); err != nil {
				return nil, err
			}
			logger.Warn("auto-generated local KEK; back this file up", "path", path)
		}
		return crypto.NewLocalKEKFromFile(path)
	}
	return nil, fmt.Errorf("unknown KMS provider %q", cfg.KMSProvider)
}

// initJWTSecret returns the secret used to sign admin session JWTs.
//
// Resolution order:
//   1. LEAKSHIELD_JWT_SECRET env var (raw bytes, must be at least 32).
//   2. ~/.leakshield/jwt.secret (0600), auto-generated on first run.
//
// --prod mode refuses to auto-generate unless LEAKSHIELD_JWT_SECRET_FILE is
// explicitly configured.
func initJWTSecret(cfg *config.Config, logger *slog.Logger) ([]byte, error) {
	if v := os.Getenv("LEAKSHIELD_JWT_SECRET"); v != "" {
		if len(v) < 32 {
			return nil, errors.New("LEAKSHIELD_JWT_SECRET must be >= 32 bytes")
		}
		return []byte(v), nil
	}
	path := os.Getenv("LEAKSHIELD_JWT_SECRET_FILE")
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		path = filepath.Join(home, ".leakshield", "jwt.secret")
	}

	if data, err := os.ReadFile(path); err == nil {
		if len(data) < 32 {
			return nil, fmt.Errorf("jwt secret file %s too short", path)
		}
		return data, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	if cfg.ProdMode {
		return nil, fmt.Errorf("--prod mode requires LEAKSHIELD_JWT_SECRET or an existing %s", path)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	secret := make([]byte, 64)
	if _, err := io.ReadFull(rand.Reader, secret); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, secret, 0o600); err != nil {
		return nil, err
	}
	logger.Warn("auto-generated local JWT secret; rotate via env var in production", "path", path)
	return secret, nil
}
