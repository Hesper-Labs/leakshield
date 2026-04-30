// Package crypto provides envelope encryption (KEK ⊃ DEK ⊃ data) for the
// gateway.
//
//	KEK  - Key Encryption Key. Lives in a KMS (Vault, AWS, GCP, Azure) or,
//	       for local development, in a 0600 file. Never persisted in the DB.
//	DEK  - Data Encryption Key. One per tenant; wrapped by the KEK and
//	       stored in companies.dek_wrapped.
//	data - master provider keys, audit_logs.prompt_encrypted, etc., all
//	       encrypted with the tenant DEK using AES-256-GCM.
//
// Only the local KEK provider is implemented in this package. KMS-backed
// providers (Vault Transit, AWS/GCP/Azure KMS) are added in subsequent
// hardening phases.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
)

const (
	keySize   = 32 // AES-256
	nonceSize = 12 // GCM standard
)

// KEKProvider wraps and unwraps DEKs. Implementations:
//   - LocalKEKProvider:  reads a 32-byte key from a 0600 file (default for dev)
//   - VaultKEKProvider:  HashiCorp Vault Transit (TODO)
//   - AWSKMSProvider:    AWS KMS GenerateDataKey / Decrypt (TODO)
//   - GCPKMSProvider, AzureKVProvider (TODO)
type KEKProvider interface {
	// ID returns a stable identifier for the KEK source (used for rotation).
	ID() string
	// WrapDEK encrypts a plaintext DEK with the KEK.
	WrapDEK(plaintextDEK []byte) ([]byte, error)
	// UnwrapDEK decrypts a wrapped DEK with the KEK.
	UnwrapDEK(wrappedDEK []byte) ([]byte, error)
}

// LocalKEKProvider reads a KEK from a local file. The file must have
// permission 0600; otherwise NewLocalKEKFromFile refuses to load it.
type LocalKEKProvider struct {
	id  string
	kek []byte
}

// NewLocalKEKFromFile loads a 32-byte KEK from the given path. It rejects
// files with insecure permissions or the wrong size.
func NewLocalKEKFromFile(path string) (*LocalKEKProvider, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("kek file %s: %w", path, err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("kek file %s has insecure permissions %o; expected 0600",
			path, info.Mode().Perm())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read kek file: %w", err)
	}
	if len(data) != keySize {
		return nil, fmt.Errorf("kek must be %d bytes, got %d", keySize, len(data))
	}
	return &LocalKEKProvider{
		id:  "local:" + path,
		kek: data,
	}, nil
}

// GenerateAndWriteKEK creates a 32-byte random KEK and writes it to path
// with 0600 permissions. It refuses to overwrite an existing file: clobbering
// a KEK means losing all data encrypted under it.
func GenerateAndWriteKEK(path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("kek file %s already exists; refusing to overwrite", path)
	}
	kek := make([]byte, keySize)
	if _, err := io.ReadFull(rand.Reader, kek); err != nil {
		return err
	}
	return os.WriteFile(path, kek, 0o600)
}

// ID returns the KEK identifier.
func (p *LocalKEKProvider) ID() string { return p.id }

// WrapDEK encrypts a DEK with the KEK.
func (p *LocalKEKProvider) WrapDEK(dek []byte) ([]byte, error) {
	if len(dek) != keySize {
		return nil, fmt.Errorf("dek must be %d bytes", keySize)
	}
	return seal(p.kek, dek)
}

// UnwrapDEK decrypts a wrapped DEK.
func (p *LocalKEKProvider) UnwrapDEK(wrapped []byte) ([]byte, error) {
	dek, err := open(p.kek, wrapped)
	if err != nil {
		return nil, err
	}
	if len(dek) != keySize {
		return nil, fmt.Errorf("unwrapped dek wrong size: %d", len(dek))
	}
	return dek, nil
}

// GenerateDEK returns a fresh 32-byte random DEK.
func GenerateDEK() ([]byte, error) {
	dek := make([]byte, keySize)
	if _, err := io.ReadFull(rand.Reader, dek); err != nil {
		return nil, err
	}
	return dek, nil
}

// EncryptWithDEK seals plaintext under the DEK. Output layout:
// nonce || ciphertext || tag.
func EncryptWithDEK(dek, plaintext []byte) ([]byte, error) {
	return seal(dek, plaintext)
}

// DecryptWithDEK reverses EncryptWithDEK.
func DecryptWithDEK(dek, blob []byte) ([]byte, error) {
	return open(dek, blob)
}

func seal(key, plaintext []byte) ([]byte, error) {
	if len(key) != keySize {
		return nil, errors.New("key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ct := aead.Seal(nil, nonce, plaintext, nil)
	out := make([]byte, 0, len(nonce)+len(ct))
	out = append(out, nonce...)
	out = append(out, ct...)
	return out, nil
}

func open(key, blob []byte) ([]byte, error) {
	if len(key) != keySize {
		return nil, errors.New("key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(blob) < aead.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}
	nonce := blob[:aead.NonceSize()]
	ct := blob[aead.NonceSize():]
	return aead.Open(nil, nonce, ct, nil)
}
