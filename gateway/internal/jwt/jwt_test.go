package jwt

import (
	"strings"
	"testing"
	"time"
)

func TestSignVerifyRoundtrip(t *testing.T) {
	secret := []byte("supersecret-test-key")
	in := Claims{
		Subject:  "admin-1",
		TenantID: "tenant-1",
		Email:    "alice@acme.test",
		Role:     "super_admin",
	}
	tok, err := Sign(secret, in)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(tok, ".") != 2 {
		t.Fatalf("expected 3 dot-separated segments, got %q", tok)
	}
	out, err := Verify(secret, tok)
	if err != nil {
		t.Fatal(err)
	}
	if out.Subject != in.Subject || out.TenantID != in.TenantID || out.Role != in.Role {
		t.Fatalf("claims mismatch: %#v vs %#v", out, in)
	}
	if out.IssuedAt == 0 || out.Expires == 0 {
		t.Fatal("default iat/exp not set")
	}
}

func TestVerifyRejectsTamperedSignature(t *testing.T) {
	secret := []byte("k")
	tok, _ := Sign(secret, Claims{Subject: "x"})
	parts := strings.Split(tok, ".")
	parts[2] = "AAAA" + parts[2][4:]
	if _, err := Verify(secret, strings.Join(parts, ".")); err == nil {
		t.Fatal("expected signature error")
	}
}

func TestVerifyRejectsWrongSecret(t *testing.T) {
	tok, _ := Sign([]byte("k1"), Claims{Subject: "x"})
	if _, err := Verify([]byte("k2"), tok); err == nil {
		t.Fatal("expected signature error with wrong secret")
	}
}

func TestVerifyRejectsExpired(t *testing.T) {
	secret := []byte("k")
	tok, _ := Sign(secret, Claims{
		Subject: "x",
		Expires: time.Now().Add(-1 * time.Minute).Unix(),
	})
	if _, err := Verify(secret, tok); err != ErrTokenExpired {
		t.Fatalf("expected ErrTokenExpired, got %v", err)
	}
}

func TestVerifyRejectsMalformed(t *testing.T) {
	cases := []string{"", "abc", "abc.def", "a.b.c.d"}
	for _, c := range cases {
		if _, err := Verify([]byte("k"), c); err == nil {
			t.Errorf("expected error for %q", c)
		}
	}
}
