// mac/cmd/remotex/main.go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/skip2/go-qrcode"
	"github.com/spf13/cobra"
	"github.com/fsaint/remotex/internal/config"
	"github.com/fsaint/remotex/internal/setup"
	tmuxpkg "github.com/fsaint/remotex/internal/tmux"
)

func main() {
	root := &cobra.Command{
		Use:   "remotex",
		Short: "Manage remote terminal sessions",
	}
	root.AddCommand(newSetupCmd(), newNewCmd(), newListCmd(), newKillCmd())
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newSetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "First-time setup: generate keys, configure daemon, print QR code",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Generating SSH keypair...")
			if err := setup.GenerateKeys(); err != nil {
				return fmt.Errorf("generate keys: %w", err)
			}

			fmt.Println("Generating API key...")
			apiKey, err := setup.GenerateAPIKey()
			if err != nil {
				return fmt.Errorf("generate API key: %w", err)
			}

			fmt.Println("Detecting Tailscale hostname...")
			host, err := setup.TailscaleHost()
			if err != nil {
				return fmt.Errorf("detect tailscale host (is tailscale running?): %w", err)
			}
			fmt.Printf("Tailscale hostname: %s\n", host)

			privKeyPath := filepath.Join(config.Dir(), "id_ed25519")
			privKeyBytes, err := os.ReadFile(privKeyPath)
			if err != nil {
				return fmt.Errorf("read private key: %w", err)
			}

			cfg := &config.Config{
				APIKey:        apiKey,
				TailscaleHost: host,
				DaemonPort:    7654,
				SSHKeyPath:    privKeyPath,
			}
			if err := config.Save(cfg); err != nil {
				return fmt.Errorf("save config: %w", err)
			}

			// Encode pairing payload as JSON for QR code
			payload := map[string]string{
				"host":            host,
				"api_key":         apiKey,
				"ssh_private_key": string(privKeyBytes),
			}
			qrData, err := json.Marshal(payload)
			if err != nil {
				return fmt.Errorf("encode QR payload: %w", err)
			}

			qr, err := qrcode.New(string(qrData), qrcode.Medium)
			if err != nil {
				return fmt.Errorf("generate QR code: %w", err)
			}
			fmt.Print("\nScan this QR code with the RemoteX iOS app:\n\n")
			fmt.Println(qr.ToSmallString(false))

			fmt.Println("Or paste this JSON into the app (for simulator / no-camera pairing):")
			fmt.Println(string(qrData))

			fmt.Println("\nSetup complete!")
			fmt.Println("Start the daemon with: remotex-daemon")
			return nil
		},
	}
}
// inferSessionName returns a session name by checking (in order):
// 1. The GitHub repo name of the current directory
// 2. The current directory's base name
func inferSessionName() string {
	out, err := exec.Command("git", "remote", "get-url", "origin").Output()
	if err == nil {
		url := strings.TrimSpace(string(out))
		// strip .git suffix, then take the last path component
		url = strings.TrimSuffix(url, ".git")
		if idx := strings.LastIndexAny(url, "/:"); idx >= 0 {
			return url[idx+1:]
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "session"
	}
	return filepath.Base(cwd)
}

func newNewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "new [name]",
		Short: "Create a new tmux session with a mosh-server",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var name string
			if len(args) == 1 {
				name = args[0]
			} else {
				name = inferSessionName()
				fmt.Printf("Using session name: %q\n", name)
			}

			if tmuxpkg.SessionExists(name) {
				return fmt.Errorf("session %q already exists", name)
			}

			pid, err := tmuxpkg.NewSession(name)
			if err != nil {
				return fmt.Errorf("create tmux session: %w", err)
			}

			cfg, err := config.Load()
			if err != nil {
				// Config not set up yet — still report session was created
				fmt.Fprintf(os.Stderr, "warn: could not load config (run 'remotex setup' first): %v\n", err)
				fmt.Printf("Session %q created (tmux pid %d) — start daemon to enable remote access\n", name, pid)
				return nil
			}

			body := map[string]interface{}{
				"name":       name,
				"tmux_pid":   pid,
				"started_at": time.Now(),
			}
			data, _ := json.Marshal(body)
			resp, err := http.Post(
				fmt.Sprintf("http://127.0.0.1:%d/internal/sessions", cfg.DaemonPort),
				"application/json",
				bytes.NewReader(data),
			)
			if err != nil {
				// If daemon is not running, still report the session was created
				fmt.Fprintf(os.Stderr, "warn: could not register with daemon: %v\n", err)
				fmt.Printf("Session %q created (tmux pid %d) — start daemon to enable remote access\n", name, pid)
				return attachSession(name)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
				return fmt.Errorf("daemon returned status %d", resp.StatusCode)
			}

			fmt.Printf("Session %q created (tmux pid %d)\n", name, pid)
			return attachSession(name)
		},
	}
}
func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List active sessions",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config (run 'remotex setup' first): %w", err)
			}

			req, _ := http.NewRequest(
				http.MethodGet,
				fmt.Sprintf("http://127.0.0.1:%d/sessions", cfg.DaemonPort),
				nil,
			)
			req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return fmt.Errorf("reach daemon: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("daemon returned status %d", resp.StatusCode)
			}

			var sessions []struct {
				Name      string    `json:"name"`
				Status    string    `json:"status"`
				StartedAt time.Time `json:"started_at"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&sessions); err != nil {
				return fmt.Errorf("decode response: %w", err)
			}

			if len(sessions) == 0 {
				fmt.Println("No active sessions.")
				return nil
			}
			fmt.Printf("%-20s %-6s %s\n", "NAME", "STATUS", "STARTED")
			for _, s := range sessions {
				fmt.Printf("%-20s %-6s %s\n", s.Name, s.Status, s.StartedAt.Format("2006-01-02 15:04:05"))
			}
			return nil
		},
	}
}

// attachSession replaces the current process with `tmux attach-session -t name`
// so the user lands directly inside the session.
func attachSession(name string) error {
	tmuxBin, err := exec.LookPath("tmux")
	if err != nil {
		fmt.Printf("Attach with: tmux attach -t %s\n", name)
		return nil
	}
	return syscall.Exec(tmuxBin, []string{"tmux", "attach-session", "-t", name}, os.Environ())
}

func newKillCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "kill <name>",
		Short: "Kill a tmux session and unregister it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			cfg, err := config.Load()
			if err == nil {
				req, _ := http.NewRequest(
					http.MethodDelete,
					fmt.Sprintf("http://127.0.0.1:%d/internal/sessions/%s", cfg.DaemonPort, name),
					nil,
				)
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warn: could not reach daemon: %v\n", err)
				} else {
					io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				}
			}

			if err := tmuxpkg.KillSession(name); err != nil {
				return fmt.Errorf("kill tmux session: %w", err)
			}

			fmt.Printf("Session %q killed\n", name)
			return nil
		},
	}
}
