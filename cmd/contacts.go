package cmd

import (
	"github.com/lcorneliussen/md365/internal/contacts"
	"github.com/spf13/cobra"
)

var (
	contactsAccount string
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
	Run: func(cmd *cobra.Command, args []string) {
		query := args[0]

		if err := contacts.Search(cfg, query, contactsAccount); err != nil {
			fatal(err)
		}
	},
}

func init() {
	contactsSearchCmd.Flags().StringVar(&contactsAccount, "account", "", "Filter by account")

	contactsCmd.AddCommand(contactsSearchCmd)
}
