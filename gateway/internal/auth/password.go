package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/argon2"
)

// HashPassword returns an argon2id hash in the standard PHC string format:
//
//	$argon2id$v=19$m=65536,t=3,p=2$<salt-b64>$<key-b64>
//
// This is what we store in admins.password_hash. VerifyPassword can parse
// the same format.
func HashPassword(password string) (string, error) {
	if len(password) < 8 {
		return "", fmt.Errorf("password too short")
	}
	salt := make([]byte, argonSaltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	encoded := fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	)
	return encoded, nil
}

// VerifyPassword performs a constant-time comparison of the candidate
// against a stored argon2id PHC string.
func VerifyPassword(candidate, stored string) bool {
	parts := strings.Split(stored, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false
	}
	var m, t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(candidate), salt, t, m, p, uint32(len(expected)))
	return subtle.ConstantTimeCompare(expected, got) == 1
}
