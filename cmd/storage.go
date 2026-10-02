package cmd

import (
	"fmt"
	"strings"

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
	storageDriveID string
	storageLimit   int
)

var filesCmd = &cobra.Command{
	Use:   "files",
	Short: "Search files across Microsoft 365",
	Long:  "Search OneDrive and SharePoint content visible to the signed-in account.",
}

var filesSearchCmd = &cobra.Command{
	Use:   "search QUERY",
	Short: "Search OneDrive and SharePoint files",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if storageAccount == "" {
			return usageError("--account is required")
		}
		query := strings.TrimSpace(args[0])
		if query == "" {
			return usageError("search query must not be empty")
		}
		if storageLimit <= 0 {
			return usageError("--limit must be greater than zero")
		}
		values, err := storage.Search(cfg, storageAccount, query, storageLimit)
		if err != nil {
			return err
		}
		if writer.IsHuman() {
			printSearchResults(cmd, values)
			return nil
		}
		return writeOK(values,
			output.WithSummary(fmt.Sprintf("%d files and folders", len(values))),
			output.WithMeta("source", "graph_search"),
			output.WithMeta("query", query),
			output.WithBreadcrumbs(output.Breadcrumb{
				Action:      "browse_library",
				Command:     "md365 sharepoint list --account <name> --drive-id <drive-id> --item-id <folder-item-id>",
				Description: "Browse a matched folder by stable drive and item IDs",
			}),
		)
	},
}

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
	Long:  "Discover and browse SharePoint document libraries for a site or Microsoft Teams team.",
}

var sharePointLibrariesCmd = &cobra.Command{
	Use:     "libraries",
	Aliases: []string{"drives"},
	Short:   "List SharePoint document libraries",
	RunE: func(cmd *cobra.Command, args []string) error {
		if storageAccount == "" {
			return usageError("--account is required")
		}
		if (storageTeamID == "") == (storageSiteID == "") {
			return usageError("choose exactly one of --team-id or --site-id")
		}
		if storageLimit <= 0 {
			return usageError("--limit must be greater than zero")
		}
		var (
			values []storage.LibraryInfo
			err    error
		)
		if storageTeamID != "" {
			values, err = storage.ListTeamLibraries(cfg, storageAccount, storageTeamID, storageLimit)
		} else {
			values, err = storage.ListSiteLibraries(cfg, storageAccount, storageSiteID, storageLimit)
		}
		if err != nil {
			return err
		}
		if writer.IsHuman() {
			printLibraries(cmd, values)
			return nil
		}
		return writeOK(values,
			output.WithSummary(fmt.Sprintf("%d document libraries", len(values))),
			output.WithMeta("source", "graph"),
			output.WithBreadcrumbs(output.Breadcrumb{
				Action:      "browse_library",
				Command:     "md365 sharepoint list --account <name> --drive-id <drive-id>",
				Description: "Browse a document library by its returned drive ID",
			}),
		)
	},
}

var sharePointListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List SharePoint files and folders",
	RunE: func(cmd *cobra.Command, args []string) error {
		if storageAccount == "" {
			return usageError("--account is required")
		}
		selectors := 0
		for _, value := range []string{storageTeamID, storageSiteID, storageDriveID} {
			if value != "" {
				selectors++
			}
		}
		if selectors != 1 {
			return usageError("choose exactly one of --team-id, --site-id, or --drive-id")
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
		if storageDriveID != "" {
			values, err = storage.ListDrive(cfg, storageAccount, storageDriveID, storageItemID, storagePath, storageLimit)
		} else if storageTeamID != "" {
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

func printSearchResults(cmd *cobra.Command, values []storage.SearchResultInfo) {
	for _, value := range values {
		fmt.Fprintf(cmd.OutOrStdout(), "%d. %s  [%s]\n  item: %s", value.Rank, value.Name, value.Type, value.ID)
		if value.DriveID != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "  drive: %s", value.DriveID)
		}
		if value.ParentPath != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "\n  path: %s", value.ParentPath)
		}
		fmt.Fprintln(cmd.OutOrStdout())
	}
	if len(values) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No files or folders found")
	}
}

func printLibraries(cmd *cobra.Command, values []storage.LibraryInfo) {
	for _, value := range values {
		fmt.Fprintf(cmd.OutOrStdout(), "%s  [%s]\n  drive: %s\n", value.Name, value.DriveType, value.ID)
	}
	if len(values) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No document libraries found")
	}
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
	filesSearchCmd.Flags().StringVar(&storageAccount, "account", "", "Account (required)")
	filesSearchCmd.Flags().IntVar(&storageLimit, "limit", 25, "Maximum search results")
	filesCmd.AddCommand(filesSearchCmd)

	addStorageListFlags(oneDriveListCmd)
	oneDriveCmd.AddCommand(oneDriveListCmd)

	addStorageListFlags(sharePointListCmd)
	sharePointListCmd.Flags().StringVar(&storageTeamID, "team-id", "", "Microsoft Teams team/group ID")
	sharePointListCmd.Flags().StringVar(&storageSiteID, "site-id", "", "SharePoint site ID")
	sharePointListCmd.Flags().StringVar(&storageDriveID, "drive-id", "", "SharePoint document library drive ID")

	sharePointLibrariesCmd.Flags().StringVar(&storageAccount, "account", "", "Account (required)")
	sharePointLibrariesCmd.Flags().StringVar(&storageTeamID, "team-id", "", "Microsoft Teams team/group ID")
	sharePointLibrariesCmd.Flags().StringVar(&storageSiteID, "site-id", "", "SharePoint site ID")
	sharePointLibrariesCmd.Flags().IntVar(&storageLimit, "limit", 100, "Maximum document libraries")
	sharePointCmd.AddCommand(sharePointListCmd, sharePointLibrariesCmd)
}
