package crypto

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRoundTripEnvelope(t *testing.T) {
	dir := t.TempDir()
	kekPath := filepath.Join(dir, "kek")
	if err := GenerateAndWriteKEK(kekPath); err != nil {
		t.Fatalf("generate kek: %v", err)
	}
	provider, err := NewLocalKEKFromFile(kekPath)
	if err != nil {
		t.Fatalf("load kek: %v", err)
	}

	dek, err := GenerateDEK()
	if err != nil {
		t.Fatalf("generate dek: %v", err)
	}
	wrapped, err := provider.WrapDEK(dek)
	if err != nil {
		t.Fatalf("wrap: %v", err)
	}
	dek2, err := provider.UnwrapDEK(wrapped)
	if err != nil {
		t.Fatalf("unwrap: %v", err)
	}
	if !bytes.Equal(dek, dek2) {
		t.Fatal("dek mismatch after roundtrip")
	}

	plaintext := []byte("sk-proj-secret-master-key-test")
	ct, err := EncryptWithDEK(dek, plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if bytes.Equal(ct, plaintext) {
		t.Fatal("ciphertext == plaintext (no encryption?)")
	}
	pt, err := DecryptWithDEK(dek2, ct)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if !bytes.Equal(plaintext, pt) {
		t.Fatalf("plaintext mismatch: got %q want %q", pt, plaintext)
	}
}

func TestRejectsInsecureKEKPerms(t *testing.T) {
	dir := t.TempDir()
	kekPath := filepath.Join(dir, "kek")
	if err := os.WriteFile(kekPath, make([]byte, 32), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewLocalKEKFromFile(kekPath); err == nil {
		t.Fatal("expected error for 0644 kek file, got nil")
	}
}

func TestRejectsWrongSizeKEK(t *testing.T) {
	dir := t.TempDir()
	kekPath := filepath.Join(dir, "kek")
	if err := os.WriteFile(kekPath, []byte("too-short"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewLocalKEKFromFile(kekPath); err == nil {
		t.Fatal("expected error for short kek, got nil")
	}
}

func TestTamperDetection(t *testing.T) {
	dek, _ := GenerateDEK()
	ct, _ := EncryptWithDEK(dek, []byte("hello"))
	// Flip a bit in the ciphertext (after the nonce)
	ct[len(ct)-1] ^= 0x01
	if _, err := DecryptWithDEK(dek, ct); err == nil {
		t.Fatal("expected GCM tag mismatch error, got nil")
	}
}
