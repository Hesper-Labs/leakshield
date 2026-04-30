package auth

import (
	"strings"
	"testing"
)

func TestGenerateParseRoundtrip(t *testing.T) {
	plain, prefix, hash, err := Generate("live")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(plain, "gw_live_") {
		t.Fatalf("expected gw_live_ prefix, got %q", plain)
	}
	if !strings.HasPrefix(prefix, "gw_live_") {
		t.Fatalf("lookup prefix wrong: %q", prefix)
	}

	parsed, err := Parse(plain)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed.Env != "live" {
		t.Fatalf("env mismatch")
	}
	if parsed.LookupPrefix != prefix {
		t.Fatalf("lookup prefix mismatch: %q vs %q", parsed.LookupPrefix, prefix)
	}
	if !VerifySecret(parsed.Secret, hash) {
		t.Fatal("verify failed")
	}
	if VerifySecret("wrong-secret", hash) {
		t.Fatal("verify must fail for wrong secret")
	}
}

func TestParseRejectsMalformed(t *testing.T) {
	cases := []string{
		"",
		"gw_live_xxx",
		"gw_prod_abcdef12_xK9p7Lm5sQwErTyUiOpAsDfGhJkLzXcV", // wrong env
		"sk_live_abcdef12_xK9p7Lm5sQwErTyUiOpAsDfGhJkLzXcV", // wrong scheme
		"gw_live_abc_xK9p7Lm5sQwErTyUiOpAsDfGhJkLzXcV",      // short prefix
		"gw_live_abcdef12_short",                             // short secret
	}
	for _, c := range cases {
		if _, err := Parse(c); err == nil {
			t.Errorf("expected error for %q, got nil", c)
		}
	}
}

func TestVerifyConstantTime(t *testing.T) {
	_, _, hash, _ := Generate("test")
	if VerifySecret("", hash) {
		t.Fatal("empty secret must not verify")
	}
}
