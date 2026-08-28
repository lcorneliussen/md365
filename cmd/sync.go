package cmd

import (
	"fmt"

	"github.com/lcorneliussen/md365/internal/auth"
	"github.com/lcorneliussen/md365/internal/output"
	"github.com/lcorneliussen/md365/internal/sync"
	"github.com/spf13/cobra"
)

var (
	syncAccount string
)

type syncAccountResult struct {
	Account  string                   `json:"account"`
	Calendar *sync.CalendarSyncResult `json:"calendar,omitempty"`
	Contacts *sync.ContactsSyncResult `json:"contacts,omitempty"`
	Errors   []string                 `json:"errors,omitempty"`
}

// syncCmd represents the sync command
var syncCmd = &cobra.Command{
	Use:   "sync [all]",
	Short: "Sync calendars and contacts",
	Long:  `Sync calendars and contacts from Microsoft 365 to local Markdown files.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Determine which accounts to sync
		var accounts []string

		if syncAccount == "all" || syncAccount == "" {
			accounts = cfg.ListAccounts()
		} else {
			accounts = []string{syncAccount}
		}

		results := []syncAccountResult{}

		// Sync each account
		for _, account := range accounts {
			result := syncAccountResult{Account: account}
			// Get access token
			token, err := auth.GetAccessToken(cfg, account)
			if err != nil {
				result.Errors = append(result.Errors, err.Error())
				results = append(results, result)
				if writer.IsHuman() {
					fmt.Fprintf(cmd.ErrOrStderr(), "Failed to sync '%s': %v\n", account, err)
				}
				continue
			}

			// Sync calendar
			if calendar, err := sync.SyncCalendar(cfg, account, token); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("calendar: %v", err))
				if writer.IsHuman() {
					fmt.Fprintf(cmd.ErrOrStderr(), "Failed to sync calendar for '%s': %v\n", account, err)
				}
			} else {
				result.Calendar = &calendar
				if writer.IsHuman() {
					fmt.Fprintf(cmd.OutOrStdout(), "Synced %d events for '%s' (deleted %d)\n", calendar.Events, account, calendar.Deleted)
				}
			}

			// Sync contacts
			if contacts, err := sync.SyncContacts(cfg, account, token); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("contacts: %v", err))
				if writer.IsHuman() {
					fmt.Fprintf(cmd.ErrOrStderr(), "Failed to sync contacts for '%s': %v\n", account, err)
				}
			} else {
				result.Contacts = &contacts
				if writer.IsHuman() {
					fmt.Fprintf(cmd.OutOrStdout(), "Synced contacts for '%s' (new/updated: %d, deleted: %d)\n", account, contacts.Updated, contacts.Deleted)
				}
			}

			results = append(results, result)
		}
		if writer.IsHuman() {
			return nil
		}
		return writeOK(results,
			output.WithSummary(fmt.Sprintf("synced %d accounts", len(results))),
			output.WithMeta("source", "graph"),
		)
	},
}

func init() {
	syncCmd.Flags().StringVar(&syncAccount, "account", "", "Account to sync (or 'all' for all accounts)")
}
