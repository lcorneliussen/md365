package cmd

import (
	"errors"
	"io"
	"os"
	"strings"

	"github.com/lcorneliussen/md365/internal/apierr"
	"github.com/lcorneliussen/md365/internal/config"
	"github.com/lcorneliussen/md365/internal/output"
	"github.com/spf13/cobra"
)

var (
	cfg                 *config.Config
	Interactive         bool
	writer              *output.Writer
	jsonFlag            bool
	quietFlag           bool
	resultsOnlyFlag     bool
	idsOnlyFlag         bool
	countFlag           bool
	selectFlag          string
	failEmptyFlag       bool
	readOnlyFlag        bool
	noInputFlag         bool
	dryRunFlag          bool
	wrapUntrustedFlag   bool
	sanitizeContentFlag bool
	loadConfig          = config.Load
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:           "md365",
	Short:         "AI- and human-friendly CLI for Microsoft 365",
	SilenceUsage:  true,
	SilenceErrors: true,
	Long: `md365 - AI- and human-friendly CLI for Microsoft 365

Syncs calendars and contacts as plain Markdown files with YAML frontmatter.
Mail, Teams, OneDrive, SharePoint, and write operations use Microsoft Graph API.`,
	PersistentPreRunE: prepareCommand,
}

func prepareCommand(cmd *cobra.Command, args []string) error {
	writer = newOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr())
	if err := validateOutputFlags(); err != nil {
		return err
	}
	if err := enforcePreConfigPolicy(cmd); err != nil {
		return err
	}
	// Skip config loading for commands that don't need it
	if commandSkipsConfig(cmd) {
		return nil
	}

	var err error
	cfg, err = loadConfig()
	if err != nil {
		return apierr.Usage(err.Error())
	}
	return executeDryRun(cmd, args)
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() int {
	prescanAutomationFlags(os.Args[1:])
	if err := rootCmd.Execute(); err != nil {
		if errors.Is(err, errDryRunComplete) {
			return 0
		}
		if writer == nil {
			writer = newOutputWriter(nil, nil)
		}
		writer.Err(err)
		return output.ExitCodeFor(err)
	}
	return 0
}

func prescanAutomationFlags(args []string) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			return
		}
		switch arg {
		case "--json":
			jsonFlag = true
		case "--quiet":
			quietFlag = true
		case "--results-only":
			resultsOnlyFlag = true
		case "--ids-only":
			idsOnlyFlag = true
		case "--count":
			countFlag = true
		case "--fail-empty":
			failEmptyFlag = true
		case "--select":
			if i+1 < len(args) {
				i++
				selectFlag = args[i]
			}
		case "--wrap-untrusted", "--wrap-untrusted=true":
			wrapUntrustedFlag = true
		case "--wrap-untrusted=false":
			wrapUntrustedFlag = false
		case "--sanitize-content", "--sanitize-content=true":
			sanitizeContentFlag = true
		case "--sanitize-content=false":
			sanitizeContentFlag = false
		default:
			if value, ok := strings.CutPrefix(arg, "--select="); ok {
				selectFlag = value
			}
		}
	}
}

func newOutputWriter(stdout, stderr io.Writer) *output.Writer {
	return output.New(output.Options{
		Format:          outputFormat(),
		Stdout:          stdout,
		Stderr:          stderr,
		WrapUntrusted:   wrapUntrustedFlag,
		SanitizeContent: sanitizeContentFlag,
		Select:          selectedFields(),
		FailEmpty:       failEmptyFlag,
	})
}

func outputFormat() output.Format {
	switch {
	case jsonFlag:
		return output.FormatJSON
	case resultsOnlyFlag:
		return output.FormatResultsOnly
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
	if resultsOnlyFlag {
		selected = append(selected, "--results-only")
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
	if strings.TrimSpace(selectFlag) != "" && (idsOnlyFlag || countFlag) {
		return apierr.Usage("--select cannot be combined with --ids-only or --count")
	}
	if selectFlag != "" && len(selectedFields()) == 0 {
		return apierr.Usage("--select requires at least one field")
	}
	if (strings.TrimSpace(selectFlag) != "" || failEmptyFlag) && !(jsonFlag || quietFlag || resultsOnlyFlag) {
		return apierr.Usage("--select and --fail-empty require --json, --results-only, or --quiet")
	}
	return nil
}

func selectedFields() []string {
	seen := map[string]bool{}
	var fields []string
	for _, field := range strings.Split(selectFlag, ",") {
		field = strings.TrimSpace(field)
		if field != "" && !seen[field] {
			seen[field] = true
			fields = append(fields, field)
		}
	}
	return fields
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
	case "about", "commands", "schema", "help", "skill":
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
	rootCmd.PersistentFlags().BoolVar(&resultsOnlyFlag, "results-only", false, "Output result data without the response envelope")
	rootCmd.PersistentFlags().BoolVar(&idsOnlyFlag, "ids-only", false, "Output only result IDs, one per line")
	rootCmd.PersistentFlags().BoolVar(&countFlag, "count", false, "Output only the result count")
	rootCmd.PersistentFlags().StringVar(&selectFlag, "select", "", "Project comma-separated response fields")
	rootCmd.PersistentFlags().BoolVar(&failEmptyFlag, "fail-empty", false, "Return empty_result when a command returns no results")
	rootCmd.PersistentFlags().BoolVar(&readOnlyFlag, "read-only", false, "Block commands that write Microsoft 365 or local state")
	rootCmd.PersistentFlags().BoolVar(&noInputFlag, "no-input", false, "Fail instead of prompting, opening a browser, or waiting for authentication")
	rootCmd.PersistentFlags().BoolVar(&dryRunFlag, "dry-run", false, "Validate and preview a supported mutation without executing it")
	rootCmd.PersistentFlags().BoolVar(&wrapUntrustedFlag, "wrap-untrusted", false, "Wrap remote Microsoft 365 content with structured provenance")
	rootCmd.PersistentFlags().BoolVar(&sanitizeContentFlag, "sanitize-content", false, "Make unsafe control and invisible characters explicit in remote content")

	// Add subcommands
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(calCmd)
	rootCmd.AddCommand(contactsCmd)
	rootCmd.AddCommand(mailCmd)
	rootCmd.AddCommand(teamsCmd)
	rootCmd.AddCommand(oneDriveCmd)
	rootCmd.AddCommand(sharePointCmd)
	rootCmd.AddCommand(filesCmd)
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(aboutCmd)
	rootCmd.AddCommand(commandsCmd)
	rootCmd.AddCommand(schemaCmd)
	rootCmd.AddCommand(skillCmd)
}

// fatal prints an error and exits
func fatal(err error) {
	if writer == nil {
		writer = newOutputWriter(nil, nil)
	}
	writer.Err(err)
	os.Exit(output.ExitCodeFor(err))
}

func usageError(message string) error {
	return apierr.Usage(message)
}

func writeOK(data any, opts ...output.ResponseOption) error {
	if writer == nil {
		writer = newOutputWriter(nil, nil)
	}
	return writer.OK(data, opts...)
}
