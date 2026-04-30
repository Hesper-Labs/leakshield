// Package jwt is a tiny HS256-only JWT signer/verifier.
//
// We deliberately avoid pulling a third-party library: HS256 is forty
// lines of well-understood code and our use is narrow (admin session
// tokens that we both issue and verify). Anything more elaborate
// (key rotation, JWKS, asymmetric keys) belongs in a Track D phase.
package jwt

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Errors returned by Verify.
var (
	ErrMalformedToken  = errors.New("malformed jwt")
	ErrUnsupportedAlg  = errors.New("unsupported jwt alg")
	ErrSignatureInvalid = errors.New("jwt signature invalid")
	ErrTokenExpired    = errors.New("jwt expired")
)

// Claims is the LeakShield admin session payload.
type Claims struct {
	Subject  string `json:"sub"`
	TenantID string `json:"tid"`
	Email    string `json:"email"`
	Name     string `json:"name,omitempty"`
	Role     string `json:"role"`
	IssuedAt int64  `json:"iat"`
	Expires  int64  `json:"exp"`
}

type header struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

// Sign produces a signed HS256 JWT with the given secret.
func Sign(secret []byte, c Claims) (string, error) {
	if c.IssuedAt == 0 {
		c.IssuedAt = time.Now().Unix()
	}
	if c.Expires == 0 {
		c.Expires = time.Now().Add(24 * time.Hour).Unix()
	}
	headerJSON, _ := json.Marshal(header{Alg: "HS256", Typ: "JWT"})
	payloadJSON, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	signingInput := encodeSegment(headerJSON) + "." + encodeSegment(payloadJSON)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(signingInput))
	sig := mac.Sum(nil)
	return signingInput + "." + encodeSegment(sig), nil
}

// Verify parses and verifies a JWT, returning its claims.
func Verify(secret []byte, token string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrMalformedToken
	}
	headerBytes, err := decodeSegment(parts[0])
	if err != nil {
		return nil, fmt.Errorf("decode header: %w", err)
	}
	var h header
	if err := json.Unmarshal(headerBytes, &h); err != nil {
		return nil, fmt.Errorf("parse header: %w", err)
	}
	if h.Alg != "HS256" {
		return nil, ErrUnsupportedAlg
	}

	signingInput := parts[0] + "." + parts[1]
	expected := hmac.New(sha256.New, secret)
	expected.Write([]byte(signingInput))
	expectedSig := expected.Sum(nil)

	gotSig, err := decodeSegment(parts[2])
	if err != nil {
		return nil, fmt.Errorf("decode signature: %w", err)
	}
	if !hmac.Equal(expectedSig, gotSig) {
		return nil, ErrSignatureInvalid
	}

	payloadBytes, err := decodeSegment(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}
	var c Claims
	if err := json.Unmarshal(payloadBytes, &c); err != nil {
		return nil, fmt.Errorf("parse payload: %w", err)
	}
	if c.Expires > 0 && time.Now().Unix() >= c.Expires {
		return nil, ErrTokenExpired
	}
	return &c, nil
}

func encodeSegment(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeSegment(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}
