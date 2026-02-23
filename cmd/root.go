package cmd

import (
	"fmt"
	"os"

	"github.com/lcorneliussen/md365/internal/config"
	"github.com/spf13/cobra"
)

var (
	cfg         *config.Config
	Interactive bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "md365",
	Short: "Markdown client for Microsoft 365",
	Long: `md365 - Markdown client for Microsoft 365

Syncs calendars and contacts as plain Markdown files with YAML frontmatter.
Write operations go through Microsoft Graph API.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip config loading for commands that don't need it
		if cmd.Name() == "help" || cmd.Name() == "md365" {
			return nil
		}

		var err error
		cfg, err = config.Load()
		if err != nil {
			return err
		}
		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&Interactive, "interactive", "i", false, "Use interactive TUI mode")

	// Add subcommands
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(calCmd)
	rootCmd.AddCommand(contactsCmd)
	rootCmd.AddCommand(mailCmd)
	rootCmd.AddCommand(authCmd)
}

// fatal prints an error and exits
func fatal(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}
