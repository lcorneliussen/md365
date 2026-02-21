package cmd

import (
	"github.com/lcorneliussen/md365/internal/auth"
	"github.com/spf13/cobra"
)

var (
	authAccount string
)

// authCmd represents the auth command
var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authentication commands",
	Long:  `Manage OAuth2 authentication with Microsoft 365.`,
}

// authLoginCmd represents the auth login command
var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to account",
	Long:  `Initiate device code flow to authenticate an account.`,
	Run: func(cmd *cobra.Command, args []string) {
		if authAccount == "" {
			fatal(cmd.Help())
			return
		}

		if err := auth.Login(cfg, authAccount); err != nil {
			fatal(err)
		}
	},
}

// authStatusCmd represents the auth status command
var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show authentication status",
	Long:  `Show authentication status for all accounts.`,
	Run: func(cmd *cobra.Command, args []string) {
		auth.Status(cfg)
	},
}

// authRefreshCmd represents the auth refresh command
var authRefreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Refresh token",
	Long:  `Force refresh the access token for an account.`,
	Run: func(cmd *cobra.Command, args []string) {
		if authAccount == "" {
			fatal(cmd.Help())
			return
		}

		if err := auth.RefreshToken(cfg, authAccount); err != nil {
			fatal(err)
		}
	},
}

// authImportCmd represents the auth import command
var authImportCmd = &cobra.Command{
	Use:   "import",
	Short: "Import tokens from files to keyring",
	Long: `Import authentication tokens from JSON files to system keyring.
Reads existing JSON token files from ~/.config/md365/tokens/<account>.json,
imports them into the system keyring, then renames the files to <account>.json.bak.

Use --account to import a specific account, or omit to import all accounts.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := auth.ImportTokens(cfg, authAccount); err != nil {
			fatal(err)
		}
	},
}

// authExportCmd represents the auth export command
var authExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export tokens from keyring to files",
	Long: `Export authentication tokens from system keyring to JSON files.
Writes tokens to ~/.config/md365/tokens/<account>.json for backup or migration.

Use --account to export a specific account, or omit to export all accounts.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := auth.ExportTokens(cfg, authAccount); err != nil {
			fatal(err)
		}
	},
}

func init() {
	authLoginCmd.Flags().StringVar(&authAccount, "account", "", "Account name (required)")
	authRefreshCmd.Flags().StringVar(&authAccount, "account", "", "Account name (required)")
	authImportCmd.Flags().StringVar(&authAccount, "account", "", "Account to import (or omit for all)")
	authExportCmd.Flags().StringVar(&authAccount, "account", "", "Account to export (or omit for all)")

	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authStatusCmd)
	authCmd.AddCommand(authRefreshCmd)
	authCmd.AddCommand(authImportCmd)
	authCmd.AddCommand(authExportCmd)
}
