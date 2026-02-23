package cmd

import (
	"github.com/lcorneliussen/md365/internal/auth"
	"github.com/spf13/cobra"
)

var (
	authAccount  string
	authScope    string
	authAddScope []string
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
	Long:  `Authenticate an account using the configured auth flow (devicecode or authcode).`,
	Run: func(cmd *cobra.Command, args []string) {
		if authAccount == "" {
			fatal(cmd.Help())
			return
		}

		if err := auth.DispatchLogin(cfg, authAccount, authScope, authAddScope); err != nil {
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

// authScopesCmd represents the auth scopes command
var authScopesCmd = &cobra.Command{
	Use:   "scopes",
	Short: "Show token scopes",
	Long:  `Display the scopes stored in the current token for an account.`,
	Run: func(cmd *cobra.Command, args []string) {
		if authAccount == "" {
			fatal(cmd.Help())
			return
		}

		if err := auth.ShowScopes(authAccount); err != nil {
			fatal(err)
		}
	},
}

func init() {
	authLoginCmd.Flags().StringVar(&authAccount, "account", "", "Account name (required)")
	authLoginCmd.Flags().StringVar(&authScope, "scope", "", "Override config scope (full scope string)")
	authLoginCmd.Flags().StringSliceVar(&authAddScope, "add-scope", []string{}, "Add scope(s) to existing token scopes")
	authRefreshCmd.Flags().StringVar(&authAccount, "account", "", "Account name (required)")
	authScopesCmd.Flags().StringVar(&authAccount, "account", "", "Account name (required)")

	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authStatusCmd)
	authCmd.AddCommand(authRefreshCmd)
	authCmd.AddCommand(authScopesCmd)
}
