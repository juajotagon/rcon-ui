// Package secret seals RCON passwords so they are never stored in plaintext.
//
// Threat model, stated plainly so the guarantees are not overread: this
// protects passwords at rest -- a leaked database file, a backup, a snapshot of
// a Kubernetes volume. It does not protect against an attacker who can already
// read the process's memory or its key material. Single-user, single-instance
// deployment is assumed throughout.
package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrCorrupt means the ciphertext failed authentication: it was truncated,
// tampered with, or sealed under a different key. The most common real cause is
// a changed or missing RCON_UI_KEY, so the message says so.
var ErrCorrupt = errors.New("secret: cannot decrypt (wrong key, or data was modified)")

// Sealer encrypts and decrypts credentials.
type Sealer interface {
	Seal(plaintext string) ([]byte, error)
	Open(ciphertext []byte) (string, error)
}

// aesSealer uses AES-256-GCM, which authenticates as well as encrypts, so
// tampering is detected rather than silently yielding garbage.
type aesSealer struct {
	aead cipher.AEAD
}

// NewFromKey builds a Sealer from raw key material of any length. The key is
// hashed to 32 bytes, so a human-typed passphrase and a generated 32-byte key
// are both acceptable inputs.
func NewFromKey(key []byte) (Sealer, error) {
	if len(key) == 0 {
		return nil, errors.New("secret: empty key")
	}

	sum := sha256.Sum256(key)
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, fmt.Errorf("secret: new cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("secret: new gcm: %w", err)
	}
	return &aesSealer{aead: aead}, nil
}

func (s *aesSealer) Seal(plaintext string) ([]byte, error) {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("secret: read nonce: %w", err)
	}
	// The nonce is prepended to the ciphertext; it is not secret, only unique.
	return s.aead.Seal(nonce, nonce, []byte(plaintext), nil), nil
}

func (s *aesSealer) Open(ciphertext []byte) (string, error) {
	n := s.aead.NonceSize()
	if len(ciphertext) < n {
		return "", ErrCorrupt
	}

	plaintext, err := s.aead.Open(nil, ciphertext[:n], ciphertext[n:], nil)
	if err != nil {
		return "", ErrCorrupt
	}
	return string(plaintext), nil
}

// KeyEnvVar names the environment variable holding the sealing key. On
// Kubernetes this is populated from a Secret, which is what makes the ArgoCD
// workflow natural: the key lives in the cluster's secret store, not in the
// database or the container image.
const KeyEnvVar = "RCON_UI_KEY"

// LoadOrCreateKey resolves key material, preferring $RCON_UI_KEY and otherwise
// falling back to a generated key file beside the database.
//
// The fallback exists so that running the binary locally works with no setup.
// It is deliberately NOT used when the environment variable is set, because a
// server deployment must keep its key outside the data volume -- otherwise a
// leaked volume contains both the sealed passwords and the key that opens them,
// and the sealing has bought nothing.
//
// The returned bool reports whether the key came from the environment, so
// callers can warn about the weaker local mode.
func LoadOrCreateKey(dir string) (key []byte, fromEnv bool, err error) {
	if v := os.Getenv(KeyEnvVar); v != "" {
		// Accept base64 for generated keys, but fall back to raw bytes so a
		// hand-written passphrase also works.
		if decoded, decErr := base64.StdEncoding.DecodeString(v); decErr == nil && len(decoded) >= 16 {
			return decoded, true, nil
		}
		return []byte(v), true, nil
	}

	path := filepath.Join(dir, "key")
	switch data, readErr := os.ReadFile(path); {
	case readErr == nil:
		return data, false, nil
	case !errors.Is(readErr, os.ErrNotExist):
		return nil, false, fmt.Errorf("secret: read key file: %w", readErr)
	}

	generated := make([]byte, 32)
	if _, err := rand.Read(generated); err != nil {
		return nil, false, fmt.Errorf("secret: generate key: %w", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, false, fmt.Errorf("secret: create data dir: %w", err)
	}
	// 0600: readable only by the owner. On a shared host this is the only
	// thing standing between another user and every stored RCON password.
	if err := os.WriteFile(path, generated, 0o600); err != nil {
		return nil, false, fmt.Errorf("secret: write key file: %w", err)
	}
	return generated, false, nil
}
