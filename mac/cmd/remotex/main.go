// mac/cmd/remotex/main.go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
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
			if resp.StatusCode != http.StatusCreated {
				return fmt.Errorf("daemon returned %d", resp.StatusCode)
			}

			fmt.Printf("Session %q created (tmux pid %d)\n", name, pid)
			return nil
		},
	}
}
func newListCmd() *cobra.Command   { return &cobra.Command{Use: "list"} }
func newKillCmd() *cobra.Command   { return &cobra.Command{Use: "kill <name>", Args: cobra.ExactArgs(1)} }
