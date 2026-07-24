package secret

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSealOpenRoundTrip(t *testing.T) {
	s, err := NewFromKey([]byte("a-test-key"))
	if err != nil {
		t.Fatalf("new: %v", err)
	}

	for _, plaintext := range []string{"", "hunter2", "a much longer password with spaces and ünïcode"} {
		sealed, err := s.Seal(plaintext)
		if err != nil {
			t.Fatalf("seal: %v", err)
		}
		if plaintext != "" && bytes.Contains(sealed, []byte(plaintext)) {
			t.Errorf("ciphertext contains the plaintext")
		}

		got, err := s.Open(sealed)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		if got != plaintext {
			t.Errorf("got %q, want %q", got, plaintext)
		}
	}
}

// Sealing the same value twice must not produce the same bytes, or an observer
// could tell which servers share a password.
func TestSealIsNondeterministic(t *testing.T) {
	s, _ := NewFromKey([]byte("k"))

	a, _ := s.Seal("same-password")
	b, _ := s.Seal("same-password")

	if bytes.Equal(a, b) {
		t.Error("identical ciphertexts: the nonce is not being randomised")
	}
}

func TestOpenWithWrongKeyFails(t *testing.T) {
	good, _ := NewFromKey([]byte("right-key"))
	bad, _ := NewFromKey([]byte("wrong-key"))

	sealed, _ := good.Seal("hunter2")
	if _, err := bad.Open(sealed); !errors.Is(err, ErrCorrupt) {
		t.Errorf("err = %v, want ErrCorrupt", err)
	}
}

// GCM authenticates, so a flipped bit must be detected rather than silently
// producing garbage that gets sent to a game server as a password.
func TestOpenDetectsTampering(t *testing.T) {
	s, _ := NewFromKey([]byte("k"))
	sealed, _ := s.Seal("hunter2")

	tampered := bytes.Clone(sealed)
	tampered[len(tampered)-1] ^= 0x01

	if _, err := s.Open(tampered); !errors.Is(err, ErrCorrupt) {
		t.Errorf("err = %v, want ErrCorrupt", err)
	}
}

func TestOpenRejectsTruncated(t *testing.T) {
	s, _ := NewFromKey([]byte("k"))
	if _, err := s.Open([]byte{1, 2, 3}); !errors.Is(err, ErrCorrupt) {
		t.Errorf("err = %v, want ErrCorrupt", err)
	}
}

func TestNewFromEmptyKeyFails(t *testing.T) {
	if _, err := NewFromKey(nil); err == nil {
		t.Error("expected an error for an empty key")
	}
}

func TestLoadOrCreateKeyPrefersEnv(t *testing.T) {
	t.Setenv(KeyEnvVar, "key-from-environment")
	dir := t.TempDir()

	key, fromEnv, err := LoadOrCreateKey(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !fromEnv {
		t.Error("fromEnv = false, want true")
	}
	if string(key) != "key-from-environment" {
		t.Errorf("key = %q", key)
	}
	// A deployment's key must not be written into the data directory, or a
	// leaked volume would contain both the sealed passwords and their key.
	if _, err := os.Stat(filepath.Join(dir, "key")); !errors.Is(err, os.ErrNotExist) {
		t.Error("key file was written even though the environment supplied a key")
	}
}

func TestLoadOrCreateKeyGeneratesAndPersists(t *testing.T) {
	t.Setenv(KeyEnvVar, "")
	dir := filepath.Join(t.TempDir(), "nested")

	first, fromEnv, err := LoadOrCreateKey(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if fromEnv {
		t.Error("fromEnv = true, want false")
	}
	if len(first) != 32 {
		t.Errorf("generated key is %d bytes, want 32", len(first))
	}

	// A second call must reuse the key, or every restart would orphan every
	// stored password.
	second, _, err := LoadOrCreateKey(dir)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Error("key changed between calls")
	}

	info, err := os.Stat(filepath.Join(dir, "key"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("key file mode = %o, want 600", perm)
	}
}
