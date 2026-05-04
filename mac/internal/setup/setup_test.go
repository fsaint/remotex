package setup_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fsaint/remotex/internal/setup"
)

func TestGenerateKeys(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	if err := setup.GenerateKeys(); err != nil {
		t.Fatalf("GenerateKeys: %v", err)
	}

	privPath := filepath.Join(dir, ".remotex", "id_ed25519")
	pubPath := privPath + ".pub"

	if _, err := os.Stat(privPath); err != nil {
		t.Errorf("private key not created: %v", err)
	}
	if _, err := os.Stat(pubPath); err != nil {
		t.Errorf("public key not created: %v", err)
	}

	// private key should not be world-readable
	info, err := os.Stat(privPath)
	if err == nil {
		if info.Mode().Perm() != 0600 {
			t.Errorf("private key permissions: got %o want 0600", info.Mode().Perm())
		}
	}
}

func TestGenerateAPIKey(t *testing.T) {
	key1, err := setup.GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey: %v", err)
	}
	if len(key1) < 32 {
		t.Errorf("API key too short: %q", key1)
	}

	// Keys should be unique
	key2, _ := setup.GenerateAPIKey()
	if key1 == key2 {
		t.Error("two generated API keys should not be equal")
	}
}
