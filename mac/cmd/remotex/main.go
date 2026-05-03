// mac/cmd/remotex/main.go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/fsaint/remotex/internal/config"
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

func newSetupCmd() *cobra.Command  { return &cobra.Command{Use: "setup"} }
func newNewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "new <name>",
		Short: "Create a new tmux session with a mosh-server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

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
				return nil
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("daemon returned status %d", resp.StatusCode)
			}
			if resp.StatusCode != http.StatusCreated {
				return fmt.Errorf("daemon returned %d", resp.StatusCode)
			}

			fmt.Printf("Session %q created (tmux pid %d)\n", name, pid)
			return nil
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
