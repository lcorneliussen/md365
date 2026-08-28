package cmd

import (
	"fmt"

	"github.com/lcorneliussen/md365/internal/output"
	"github.com/spf13/cobra"
)

type aboutInfo struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	ReadModel   map[string]string `json:"read_model"`
	Writes      []string          `json:"writes"`
	Storage     map[string]string `json:"storage"`
	Accounts    string            `json:"accounts"`
	Examples    []string          `json:"examples"`
}

var aboutCmd = &cobra.Command{
	Use:   "about",
	Short: "Explain what md365 is and how it is meant to be used",
	Long:  "Explain md365's read model, cache behavior, account model, and agent-friendly conventions.",
	RunE: func(cmd *cobra.Command, args []string) error {
		info := newAboutInfo()
		if !writer.IsHuman() {
			return writeOK(info,
				output.WithSummary("md365 about"),
				output.WithBreadcrumbs(output.Breadcrumb{
					Action:      "sync",
					Command:     "md365 sync --account <name>",
					Description: "Refresh local calendar and contact Markdown files",
				}),
			)
		}

		out := cmd.OutOrStdout()
		fmt.Fprintln(out, "md365 - AI- and human-friendly CLI for Microsoft 365")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "md365 makes calendars, contacts, and mail easier to use from terminals,")
		fmt.Fprintln(out, "scripts, and AI agents. It keeps fast local Markdown read models where")
		fmt.Fprintln(out, "they are useful, and uses Microsoft Graph directly for live reads and writes.")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Read model:")
		fmt.Fprintln(out, "  Calendar and contact list/search commands default to the local Markdown cache.")
		fmt.Fprintln(out, "  Use --no-cache on supported commands to bypass local files and read Graph directly.")
		fmt.Fprintln(out, "  Mail list/get currently reads Microsoft Graph directly.")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Writes:")
		fmt.Fprintln(out, "  Creating/deleting calendar events and sending mail always go through Graph.")
		fmt.Fprintln(out, "  Cross-tenant recipient checks use configured account domains before writes.")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Storage:")
		fmt.Fprintln(out, "  Config: ~/.config/md365/config.yaml")
		fmt.Fprintln(out, "  Tokens: system keyring, service=md365")
		fmt.Fprintln(out, "  Data:   ~/.local/share/md365/<account>/calendar|contacts/*.md")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Accounts:")
		fmt.Fprintln(out, "  Commands use account names from config, not email addresses.")
		fmt.Fprintln(out, "  Examples: --account private, --account dcg, --account talendos, --account oms")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Useful commands:")
		fmt.Fprintln(out, "  md365 auth add -i")
		fmt.Fprintln(out, "  md365 auth login --account <name>")
		fmt.Fprintln(out, "  md365 sync --account <name>")
		fmt.Fprintln(out, "  md365 cal list --account <name> --from 2026-01-01 --to 2026-12-31")
		fmt.Fprintln(out, "  md365 cal list --account <name> --no-cache")
		fmt.Fprintln(out, "  md365 contacts search <query> --account <name>")
		fmt.Fprintln(out, "  md365 contacts search <query> --account <name> --no-cache")
		fmt.Fprintln(out, "  md365 mail list --account <name> --search <query>")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Agent interface:")
		fmt.Fprintln(out, "  Use --json for stable envelopes with ok, data, summary, meta, and breadcrumbs.")
		fmt.Fprintln(out, "  Use --ids-only or --count for compact scripting output.")
		fmt.Fprintln(out, "  mail get --json includes an attachment breadcrumb when attachments exist.")
		return nil
	},
}

func newAboutInfo() aboutInfo {
	return aboutInfo{
		Name:        "md365",
		Description: "AI- and human-friendly CLI for Microsoft 365 calendars, contacts, and mail as Markdown-oriented workflows.",
		ReadModel: map[string]string{
			"calendar": "cache-first; use --no-cache to read Microsoft Graph directly",
			"contacts": "cache-first; use --no-cache to read Microsoft Graph directly",
			"mail":     "live Graph reads today; mail index cache is a planned read model",
		},
		Writes: []string{
			"Calendar create/delete always goes through Microsoft Graph.",
			"Mail send always goes through Microsoft Graph.",
			"Cross-tenant recipient checks use configured account domains before writes.",
		},
		Storage: map[string]string{
			"config": "~/.config/md365/config.yaml",
			"tokens": "system keyring, service=md365",
			"data":   "~/.local/share/md365/<account>/calendar|contacts/*.md",
		},
		Accounts: "Commands use account names from config, not email addresses.",
		Examples: []string{
			"md365 auth add -i",
			"md365 auth login --account <name>",
			"md365 sync --account <name>",
			"md365 cal list --account <name> --from 2026-01-01 --to 2026-12-31",
			"md365 cal list --account <name> --no-cache",
			"md365 contacts search <query> --account <name>",
			"md365 contacts search <query> --account <name> --no-cache",
			"md365 mail list --account <name> --search <query>",
		},
	}
}
