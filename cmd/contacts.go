package cmd

import (
	"fmt"
	"strings"

	"github.com/lcorneliussen/md365/internal/contacts"
	"github.com/lcorneliussen/md365/internal/output"
	"github.com/spf13/cobra"
)

var (
	contactsAccount string
	contactsNoCache bool
)

// contactsCmd represents the contacts command
var contactsCmd = &cobra.Command{
	Use:   "contacts",
	Short: "Contacts commands",
	Long:  `Manage contacts.`,
}

// contactsSearchCmd represents the contacts search command
var contactsSearchCmd = &cobra.Command{
	Use:   "search QUERY",
	Short: "Search contacts",
	Long:  `Search for contacts matching a query.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]

		results, err := contacts.Search(cfg, query, contactsAccount, contactsNoCache)
		if err != nil {
			return err
		}
		if writer.IsHuman() {
			printContacts(cmd, results)
			return nil
		}
		return writeOK(results,
			output.WithSummary(fmt.Sprintf("%d contacts", len(results))),
			output.WithMeta("source", sourceName(contactsNoCache)),
		)
	},
}

func init() {
	contactsSearchCmd.Flags().StringVar(&contactsAccount, "account", "", "Filter by account")
	contactsSearchCmd.Flags().BoolVar(&contactsNoCache, "no-cache", false, "Read directly from Microsoft Graph instead of local Markdown")

	contactsCmd.AddCommand(contactsSearchCmd)
}

func printContacts(cmd *cobra.Command, results []contacts.ContactInfo) {
	for _, contact := range results {
		line := fmt.Sprintf("[%s] %s", contact.Account, contact.DisplayName)
		if len(contact.Emails) > 0 {
			line += fmt.Sprintf(" <%s>", contact.Emails[0])
		}
		if contact.Company != "" || contact.JobTitle != "" {
			line += "  " + strings.TrimSpace(strings.Join([]string{contact.Company, contact.JobTitle}, " "))
		}
		fmt.Fprintln(cmd.OutOrStdout(), line)
	}
}
