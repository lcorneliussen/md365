package cmd

import (
	"fmt"

	"github.com/lcorneliussen/md365/internal/output"
	"github.com/lcorneliussen/md365/internal/storage"
	"github.com/lcorneliussen/md365/internal/teams"
	"github.com/spf13/cobra"
)

var (
	teamsAccount   string
	teamsTeamID    string
	teamsChannelID string
	teamsItemID    string
	teamsLimit     int
)

var teamsCmd = &cobra.Command{
	Use:   "teams",
	Short: "Microsoft Teams commands",
	Long:  "Browse joined teams, channels, and their SharePoint-backed files.",
}

var teamsListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List joined teams",
	RunE: func(cmd *cobra.Command, args []string) error {
		if teamsAccount == "" {
			return usageError("--account is required")
		}
		if teamsLimit <= 0 {
			return usageError("--limit must be greater than zero")
		}
		values, err := teams.List(cfg, teamsAccount, teamsLimit+1)
		if err != nil {
			return err
		}
		values, page := collectionPage(values, teamsLimit)
		if writer.IsHuman() {
			for _, value := range values {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n  %s\n", value.DisplayName, value.ID)
			}
			if len(values) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No teams found")
			}
			return nil
		}
		return writeOK(values,
			output.WithSummary(fmt.Sprintf("%d teams", len(values))),
			output.WithMeta("source", "graph"),
			page,
		)
	},
}

var teamsChannelsCmd = &cobra.Command{
	Use:     "channels",
	Aliases: []string{"channel"},
	Short:   "List channels in a team",
	RunE: func(cmd *cobra.Command, args []string) error {
		if teamsAccount == "" || teamsTeamID == "" {
			return usageError("--account and --team-id are required")
		}
		if teamsLimit <= 0 {
			return usageError("--limit must be greater than zero")
		}
		values, err := teams.ListChannels(cfg, teamsAccount, teamsTeamID, teamsLimit+1)
		if err != nil {
			return err
		}
		values, page := collectionPage(values, teamsLimit)
		if writer.IsHuman() {
			for _, value := range values {
				fmt.Fprintf(cmd.OutOrStdout(), "%s  [%s]\n  %s\n", value.DisplayName, value.MembershipType, value.ID)
			}
			if len(values) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No channels found")
			}
			return nil
		}
		return writeOK(values,
			output.WithSummary(fmt.Sprintf("%d channels", len(values))),
			output.WithMeta("source", "graph"),
			page,
		)
	},
}

var teamsFilesCmd = &cobra.Command{
	Use:   "files",
	Short: "List files in a channel's SharePoint folder",
	RunE: func(cmd *cobra.Command, args []string) error {
		if teamsAccount == "" || teamsTeamID == "" || teamsChannelID == "" {
			return usageError("--account, --team-id, and --channel-id are required")
		}
		if teamsLimit <= 0 {
			return usageError("--limit must be greater than zero")
		}
		values, err := storage.ListChannelFiles(cfg, teamsAccount, teamsTeamID, teamsChannelID, teamsItemID, teamsLimit+1)
		if err != nil {
			return err
		}
		values, page := collectionPage(values, teamsLimit)
		if writer.IsHuman() {
			printStorageItems(cmd, values)
			return nil
		}
		return writeOK(values,
			output.WithSummary(fmt.Sprintf("%d files and folders", len(values))),
			output.WithMeta("source", "graph"),
			output.WithMeta("storage", "sharepoint"),
			page,
		)
	},
}

func init() {
	teamsListCmd.Flags().StringVar(&teamsAccount, "account", "", "Account (required)")
	teamsListCmd.Flags().IntVar(&teamsLimit, "limit", 100, "Maximum teams")

	teamsChannelsCmd.Flags().StringVar(&teamsAccount, "account", "", "Account (required)")
	teamsChannelsCmd.Flags().StringVar(&teamsTeamID, "team-id", "", "Team ID (required)")
	teamsChannelsCmd.Flags().IntVar(&teamsLimit, "limit", 100, "Maximum channels")

	teamsFilesCmd.Flags().StringVar(&teamsAccount, "account", "", "Account (required)")
	teamsFilesCmd.Flags().StringVar(&teamsTeamID, "team-id", "", "Team ID (required)")
	teamsFilesCmd.Flags().StringVar(&teamsChannelID, "channel-id", "", "Channel ID (required)")
	teamsFilesCmd.Flags().StringVar(&teamsItemID, "item-id", "", "Folder item ID (default: channel root)")
	teamsFilesCmd.Flags().IntVar(&teamsLimit, "limit", 100, "Maximum files and folders")

	teamsCmd.AddCommand(teamsListCmd, teamsChannelsCmd, teamsFilesCmd)
}
