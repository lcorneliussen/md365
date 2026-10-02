package cmd

import (
	"fmt"

	"github.com/lcorneliussen/md365/internal/output"
	"github.com/lcorneliussen/md365/internal/storage"
	"github.com/spf13/cobra"
)

var (
	storageAccount string
	storagePath    string
	storageItemID  string
	storageTeamID  string
	storageSiteID  string
	storageLimit   int
)

var oneDriveCmd = &cobra.Command{
	Use:   "onedrive",
	Short: "OneDrive commands",
	Long:  "Browse files and folders in the signed-in user's OneDrive.",
}

var oneDriveListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List OneDrive files and folders",
	RunE: func(cmd *cobra.Command, args []string) error {
		if storageAccount == "" {
			return usageError("--account is required")
		}
		if storagePath != "" && storageItemID != "" {
			return usageError("--path and --item-id are mutually exclusive")
		}
		if storageLimit <= 0 {
			return usageError("--limit must be greater than zero")
		}
		values, err := storage.ListOneDrive(cfg, storageAccount, storageItemID, storagePath, storageLimit)
		if err != nil {
			return err
		}
		return writeStorageItems(cmd, values, "onedrive")
	},
}

var sharePointCmd = &cobra.Command{
	Use:   "sharepoint",
	Short: "SharePoint document library commands",
	Long:  "Browse a SharePoint site's or Microsoft Teams team's default document library.",
}

var sharePointListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List SharePoint files and folders",
	RunE: func(cmd *cobra.Command, args []string) error {
		if storageAccount == "" {
			return usageError("--account is required")
		}
		if (storageTeamID == "") == (storageSiteID == "") {
			return usageError("choose exactly one of --team-id or --site-id")
		}
		if storagePath != "" && storageItemID != "" {
			return usageError("--path and --item-id are mutually exclusive")
		}
		if storageLimit <= 0 {
			return usageError("--limit must be greater than zero")
		}
		var (
			values []storage.ItemInfo
			err    error
		)
		if storageTeamID != "" {
			values, err = storage.ListTeamDrive(cfg, storageAccount, storageTeamID, storageItemID, storagePath, storageLimit)
		} else {
			values, err = storage.ListSiteDrive(cfg, storageAccount, storageSiteID, storageItemID, storagePath, storageLimit)
		}
		if err != nil {
			return err
		}
		return writeStorageItems(cmd, values, "sharepoint")
	},
}

func addStorageListFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&storageAccount, "account", "", "Account (required)")
	cmd.Flags().StringVar(&storagePath, "path", "", "Folder path from drive root")
	cmd.Flags().StringVar(&storageItemID, "item-id", "", "Folder item ID")
	cmd.Flags().IntVar(&storageLimit, "limit", 100, "Maximum files and folders")
}

func writeStorageItems(cmd *cobra.Command, values []storage.ItemInfo, source string) error {
	if writer.IsHuman() {
		printStorageItems(cmd, values)
		return nil
	}
	return writeOK(values,
		output.WithSummary(fmt.Sprintf("%d files and folders", len(values))),
		output.WithMeta("source", "graph"),
		output.WithMeta("storage", source),
	)
}

func printStorageItems(cmd *cobra.Command, values []storage.ItemInfo) {
	for _, value := range values {
		detail := value.Type
		if value.Type == "file" && value.Size > 0 {
			detail = fmt.Sprintf("%s, %d bytes", detail, value.Size)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s  [%s]\n  item: %s", value.Name, detail, value.ID)
		if value.DriveID != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "  drive: %s", value.DriveID)
		}
		fmt.Fprintln(cmd.OutOrStdout())
	}
	if len(values) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No files or folders found")
	}
}

func init() {
	addStorageListFlags(oneDriveListCmd)
	oneDriveCmd.AddCommand(oneDriveListCmd)

	addStorageListFlags(sharePointListCmd)
	sharePointListCmd.Flags().StringVar(&storageTeamID, "team-id", "", "Microsoft Teams team/group ID")
	sharePointListCmd.Flags().StringVar(&storageSiteID, "site-id", "", "SharePoint site ID")
	sharePointCmd.AddCommand(sharePointListCmd)
}
