// Package auth handles parsing, hashing, and verifying virtual API keys.
//
// Virtual key format:
//
//	gw_<env>_<8-char-prefix>_<32-char-secret>
//
// where <env> is "live" or "test". The 8-char prefix portion is unique and
// indexed in the database; lookups are O(1) by prefix. The 32-char secret
// is hashed with argon2id and only the hash is stored. The plaintext key is
// returned to the admin once at creation time and never again.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	envLive       = "live"
	envTest       = "test"
	prefixLength  = 8
	secretLength  = 32
	keyPartsCount = 4 // gw, live|test, prefix, secret

	// argon2id parameters tuned for ~50 ms verify on a modern CPU.
	argonTime    = 3
	argonMemory  = 64 * 1024 // 64 MB
	argonThreads = 2
	argonKeyLen  = 32
	argonSaltLen = 16
)

// Errors returned by Parse.
var (
	ErrMalformedKey = errors.New("malformed virtual key")
	ErrWrongEnv     = errors.New("virtual key environment mismatch")
)

// ParsedKey holds the components extracted from a virtual key string.
type ParsedKey struct {
	Env    string // "live" or "test"
	Prefix string // 8-char lookup portion
	Secret string // 32-char secret (never persisted)

	// LookupPrefix is the value stored in virtual_keys.key_prefix:
	// "gw_<env>_<Prefix>".
	LookupPrefix string
}

// Parse decomposes a "gw_<env>_<prefix>_<secret>" key.
func Parse(key string) (*ParsedKey, error) {
	parts := strings.Split(key, "_")
	if len(parts) != keyPartsCount {
		return nil, ErrMalformedKey
	}
	if parts[0] != "gw" {
		return nil, ErrMalformedKey
	}
	env := parts[1]
	if env != envLive && env != envTest {
		return nil, ErrMalformedKey
	}
	prefix, secret := parts[2], parts[3]
	if len(prefix) != prefixLength || len(secret) != secretLength {
		return nil, ErrMalformedKey
	}
	return &ParsedKey{
		Env:          env,
		Prefix:       prefix,
		Secret:       secret,
		LookupPrefix: fmt.Sprintf("gw_%s_%s", env, prefix),
	}, nil
}

// Generate produces a new virtual key. The plaintext is returned once and
// the caller is responsible for surfacing it to the user and immediately
// discarding it.
func Generate(env string) (plaintext string, lookupPrefix string, hash []byte, err error) {
	if env != envLive && env != envTest {
		return "", "", nil, fmt.Errorf("invalid env %q", env)
	}
	prefix, err := randomLowerAlnum(prefixLength)
	if err != nil {
		return "", "", nil, err
	}
	secret, err := randomURLSafe(secretLength)
	if err != nil {
		return "", "", nil, err
	}
	plaintext = fmt.Sprintf("gw_%s_%s_%s", env, prefix, secret)
	lookupPrefix = fmt.Sprintf("gw_%s_%s", env, prefix)

	hash, err = HashSecret(secret)
	if err != nil {
		return "", "", nil, err
	}
	return plaintext, lookupPrefix, hash, nil
}

// HashSecret hashes a secret with argon2id.
//
// Encoding: salt (16 bytes) || derived key (32 bytes) — 48 bytes total,
// stored in the bytea column as-is.
func HashSecret(secret string) ([]byte, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}
	h := argon2.IDKey([]byte(secret), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	out := make([]byte, 0, len(salt)+len(h))
	out = append(out, salt...)
	out = append(out, h...)
	return out, nil
}

// VerifySecret performs a constant-time comparison of the secret against
// a stored argon2id hash.
func VerifySecret(secret string, stored []byte) bool {
	if len(stored) != argonSaltLen+argonKeyLen {
		return false
	}
	salt := stored[:argonSaltLen]
	expected := stored[argonSaltLen:]
	got := argon2.IDKey([]byte(secret), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return subtle.ConstantTimeCompare(expected, got) == 1
}

func randomLowerAlnum(n int) (string, error) {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	buf := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return "", err
	}
	for i := range buf {
		buf[i] = alphabet[int(buf[i])%len(alphabet)]
	}
	return string(buf), nil
}

func randomURLSafe(n int) (string, error) {
	raw := make([]byte, (n*6+7)/8)
	if _, err := io.ReadFull(rand.Reader, raw); err != nil {
		return "", err
	}
	s := base64.RawURLEncoding.EncodeToString(raw)
	if len(s) >= n {
		return s[:n], nil
	}
	for len(s) < n {
		s += "x"
	}
	return s[:n], nil
}
