package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/lcorneliussen/md365/internal/auth"
	"github.com/lcorneliussen/md365/internal/config"
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

// authAddCmd represents the auth add command
var authAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Interactively add a new account",
	Long:  `Interactively set up a new account with authentication configuration.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runAuthAdd(); err != nil {
			fatal(err)
		}
	},
}

func runAuthAdd() error {
	var (
		accountName  string
		emailHint    string
		authFlow     string
		scopeChoices []string
		domainsInput string
		loginNow     bool
	)

	// Create the interactive form
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Account name").
				Description("Short alias like \"work\", \"private\"").
				Value(&accountName).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("account name cannot be empty")
					}
					return nil
				}),

			huh.NewInput().
				Title("Email hint").
				Description("e.g. user@company.com").
				Value(&emailHint),
		),

		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Authentication flow").
				Options(
					huh.NewOption("Device Code (default, for most tenants)", "devicecode"),
					huh.NewOption("Browser-based (PKCE, for tenants that block device code)", "authcode"),
				).
				Value(&authFlow),
		),

		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Select permissions").
				Description("Choose one or more scopes").
				Options(
					huh.NewOption("Calendar (read/write)", "Calendars.ReadWrite"),
					huh.NewOption("Contacts (read/write)", "Contacts.ReadWrite"),
					huh.NewOption("Mail (send)", "Mail.Send"),
					huh.NewOption("User profile (read)", "User.Read"),
				).
				Value(&scopeChoices),
		),

		huh.NewGroup(
			huh.NewInput().
				Title("Domains").
				Description("Comma-separated, e.g. company.com,subsidiary.com (optional)").
				Value(&domainsInput),
		),

		huh.NewGroup(
			huh.NewConfirm().
				Title("Login now?").
				Value(&loginNow),
		),
	)

	// Run the form
	if err := form.Run(); err != nil {
		return fmt.Errorf("form cancelled or failed: %w", err)
	}

	// Process the collected data
	accountName = strings.TrimSpace(accountName)
	emailHint = strings.TrimSpace(emailHint)

	// Build scopes list
	var scopes []string
	scopes = append(scopes, scopeChoices...)
	// Always add offline_access
	scopes = append(scopes, "offline_access")
	scopeStr := strings.Join(scopes, " ")

	// Process domains
	var domains []string
	if domainsInput != "" {
		for _, d := range strings.Split(domainsInput, ",") {
			domain := strings.TrimSpace(d)
			if domain != "" {
				domains = append(domains, domain)
			}
		}
	}

	// Create account and save to config
	account := &config.Account{
		AuthFlow: authFlow,
		Hint:     emailHint,
		Scope:    scopeStr,
		Domains:  domains,
	}

	if err := config.SaveAccount(accountName, account); err != nil {
		return fmt.Errorf("failed to save account: %w", err)
	}

	fmt.Printf("\nAccount '%s' created successfully!\n", accountName)
	fmt.Printf("  Auth flow: %s\n", authFlow)
	fmt.Printf("  Email hint: %s\n", emailHint)
	fmt.Printf("  Scopes: %s\n", scopeStr)
	if len(domains) > 0 {
		fmt.Printf("  Domains: %s\n", strings.Join(domains, ", "))
	}

	// Login if confirmed
	if loginNow {
		// Reload config to get the new account
		newCfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to reload config: %w", err)
		}
		fmt.Println()
		return auth.DispatchLogin(newCfg, accountName, "", nil)
	}

	return nil
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
	authCmd.AddCommand(authAddCmd)
}
