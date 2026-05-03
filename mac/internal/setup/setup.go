package setup

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/fsaint/remotex/internal/config"
)

// GenerateKeys creates an Ed25519 keypair in ~/.remotex/ and appends the
// public key to ~/.ssh/authorized_keys.
func GenerateKeys() error {
	dir := config.Dir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("generate key: %w", err)
	}

	// Marshal private key to PEM
	privPEM, err := ssh.MarshalPrivateKey(priv, "remotex")
	if err != nil {
		return fmt.Errorf("marshal private key: %w", err)
	}

	// Write private key
	privPath := filepath.Join(dir, "id_ed25519")
	if err := os.WriteFile(privPath, pem.EncodeToMemory(privPEM), 0600); err != nil {
		return err
	}

	// Write public key
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		return fmt.Errorf("create ssh public key: %w", err)
	}
	pubBytes := ssh.MarshalAuthorizedKey(sshPub)
	if err := os.WriteFile(privPath+".pub", pubBytes, 0644); err != nil {
		return err
	}

	// Append to authorized_keys
	home, _ := os.UserHomeDir()
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return fmt.Errorf("create .ssh dir: %w", err)
	}
	authKeysPath := filepath.Join(sshDir, "authorized_keys")
	f, err := os.OpenFile(authKeysPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("open authorized_keys: %w", err)
	}
	defer f.Close()
	_, err = f.Write(pubBytes)
	return err
}

// GenerateAPIKey returns a random 256-bit base64url-encoded API key.
func GenerateAPIKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// TailscaleHost returns the current machine's Tailscale hostname.
func TailscaleHost() (string, error) {
	out, err := exec.Command("tailscale", "status", "--json").Output()
	if err != nil {
		return "", fmt.Errorf("tailscale status: %w", err)
	}
	var status struct {
		Self struct {
			DNSName string `json:"DNSName"`
		} `json:"Self"`
	}
	if err := json.Unmarshal(out, &status); err != nil {
		return "", fmt.Errorf("parse tailscale status: %w", err)
	}
	// DNSName includes trailing dot — remove it
	return strings.TrimSuffix(status.Self.DNSName, "."), nil
}
