package cmd

import (
	"fmt"

	"github.com/larsc/md365/internal/auth"
	"github.com/larsc/md365/internal/sync"
	"github.com/spf13/cobra"
)

var (
	syncAccount string
)

// syncCmd represents the sync command
var syncCmd = &cobra.Command{
	Use:   "sync [all]",
	Short: "Sync calendars and contacts",
	Long:  `Sync calendars and contacts from Microsoft 365 to local Markdown files.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Determine which accounts to sync
		var accounts []string

		if syncAccount == "all" || syncAccount == "" {
			accounts = cfg.ListAccounts()
		} else {
			accounts = []string{syncAccount}
		}

		// Sync each account
		for _, account := range accounts {
			// Get access token
			token, err := auth.GetAccessToken(cfg, account)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Failed to sync '%s': %v\n", account, err)
				continue
			}

			// Sync calendar
			if err := sync.SyncCalendar(cfg, account, token); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Failed to sync calendar for '%s': %v\n", account, err)
			}

			// Sync contacts
			if err := sync.SyncContacts(cfg, account, token); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Failed to sync contacts for '%s': %v\n", account, err)
			}
		}
	},
}

func init() {
	syncCmd.Flags().StringVar(&syncAccount, "account", "", "Account to sync (or 'all' for all accounts)")
}
