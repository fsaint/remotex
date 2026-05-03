// mac/cmd/remotex/main.go
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
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
func newNewCmd() *cobra.Command    { return &cobra.Command{Use: "new <name>", Args: cobra.ExactArgs(1)} }
func newListCmd() *cobra.Command   { return &cobra.Command{Use: "list"} }
func newKillCmd() *cobra.Command   { return &cobra.Command{Use: "kill <name>", Args: cobra.ExactArgs(1)} }
