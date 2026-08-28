package cmd

import (
	"os"
	"strings"

	"github.com/lcorneliussen/md365/internal/apierr"
	"github.com/lcorneliussen/md365/internal/config"
	"github.com/lcorneliussen/md365/internal/output"
	"github.com/spf13/cobra"
)

var (
	cfg         *config.Config
	Interactive bool
	writer      *output.Writer
	jsonFlag    bool
	quietFlag   bool
	idsOnlyFlag bool
	countFlag   bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:           "md365",
	Short:         "AI- and human-friendly CLI for Microsoft 365",
	SilenceUsage:  true,
	SilenceErrors: true,
	Long: `md365 - AI- and human-friendly CLI for Microsoft 365

Syncs calendars and contacts as plain Markdown files with YAML frontmatter.
Mail list/get and write operations go through Microsoft Graph API.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		writer = output.New(output.Options{
			Format: outputFormat(),
			Stdout: cmd.OutOrStdout(),
			Stderr: cmd.ErrOrStderr(),
		})
		if err := validateOutputFlags(); err != nil {
			return err
		}

		// Skip config loading for commands that don't need it
		if commandSkipsConfig(cmd) {
			return nil
		}

		var err error
		cfg, err = config.Load()
		if err != nil {
			return apierr.Usage(err.Error())
		}
		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() int {
	if err := rootCmd.Execute(); err != nil {
		if writer == nil {
			writer = output.New(output.Options{Format: outputFormat()})
		}
		writer.Err(err)
		return output.ExitCodeFor(err)
	}
	return 0
}

func outputFormat() output.Format {
	switch {
	case jsonFlag:
		return output.FormatJSON
	case quietFlag:
		return output.FormatQuiet
	case idsOnlyFlag:
		return output.FormatIDs
	case countFlag:
		return output.FormatCount
	default:
		return output.FormatHuman
	}
}

func validateOutputFlags() error {
	selected := []string{}
	if jsonFlag {
		selected = append(selected, "--json")
	}
	if quietFlag {
		selected = append(selected, "--quiet")
	}
	if idsOnlyFlag {
		selected = append(selected, "--ids-only")
	}
	if countFlag {
		selected = append(selected, "--count")
	}
	if len(selected) > 1 {
		return apierr.Usage("choose only one output format: " + strings.Join(selected, ", "))
	}
	return nil
}

func commandSkipsConfig(cmd *cobra.Command) bool {
	parts := strings.Fields(cmd.CommandPath())
	if len(parts) == 0 {
		return true
	}
	if len(parts) == 1 {
		return parts[0] == "md365"
	}
	switch parts[1] {
	case "about", "commands", "help", "skill":
		return true
	case "auth":
		return len(parts) > 2 && parts[2] == "add"
	default:
		return false
	}
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&Interactive, "interactive", "i", false, "Use interactive TUI mode")
	rootCmd.PersistentFlags().BoolVar(&jsonFlag, "json", false, "Output a stable JSON envelope")
	rootCmd.PersistentFlags().BoolVar(&quietFlag, "quiet", false, "Output result data only")
	rootCmd.PersistentFlags().BoolVar(&idsOnlyFlag, "ids-only", false, "Output only result IDs, one per line")
	rootCmd.PersistentFlags().BoolVar(&countFlag, "count", false, "Output only the result count")

	// Add subcommands
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(calCmd)
	rootCmd.AddCommand(contactsCmd)
	rootCmd.AddCommand(mailCmd)
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(aboutCmd)
	rootCmd.AddCommand(commandsCmd)
	rootCmd.AddCommand(skillCmd)
}

// fatal prints an error and exits
func fatal(err error) {
	if writer == nil {
		writer = output.New(output.Options{Format: outputFormat()})
	}
	writer.Err(err)
	os.Exit(output.ExitCodeFor(err))
}

func usageError(message string) error {
	return apierr.Usage(message)
}

func writeOK(data any, opts ...output.ResponseOption) error {
	if writer == nil {
		writer = output.New(output.Options{Format: outputFormat()})
	}
	return writer.OK(data, opts...)
}
