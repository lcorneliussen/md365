package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

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
	scanner := bufio.NewScanner(os.Stdin)

	// 1. Ask for account name
	fmt.Print("Account name (short alias like \"work\", \"private\"): ")
	if !scanner.Scan() {
		return fmt.Errorf("failed to read account name")
	}
	accountName := strings.TrimSpace(scanner.Text())
	if accountName == "" {
		return fmt.Errorf("account name cannot be empty")
	}

	// 2. Ask for email hint
	fmt.Print("Email hint (e.g. user@company.com): ")
	if !scanner.Scan() {
		return fmt.Errorf("failed to read email hint")
	}
	emailHint := strings.TrimSpace(scanner.Text())

	// 3. Ask for auth flow
	fmt.Println("Which authentication flow?")
	fmt.Println("  [1] Device Code (default, for most tenants)")
	fmt.Println("  [2] Browser-based (PKCE, for tenants that block device code)")
	fmt.Print("> ")
	if !scanner.Scan() {
		return fmt.Errorf("failed to read auth flow choice")
	}
	flowChoice := strings.TrimSpace(scanner.Text())
	authFlow := "devicecode"
	if flowChoice == "2" {
		authFlow = "authcode"
	}

	// 4. Ask for scopes
	fmt.Println("Select permissions (comma-separated numbers):")
	fmt.Println("  [1] Calendar (read/write)")
	fmt.Println("  [2] Contacts (read/write)")
	fmt.Println("  [3] Mail (send)")
	fmt.Println("  [4] User profile (read)")
	fmt.Print("> ")
	if !scanner.Scan() {
		return fmt.Errorf("failed to read scope choices")
	}
	scopeChoices := strings.TrimSpace(scanner.Text())

	// Map scope choices to Microsoft Graph permissions
	scopeMap := map[string]string{
		"1": "Calendars.ReadWrite",
		"2": "Contacts.ReadWrite",
		"3": "Mail.Send",
		"4": "User.Read",
	}

	var scopes []string
	for _, choice := range strings.Split(scopeChoices, ",") {
		choice = strings.TrimSpace(choice)
		if scope, ok := scopeMap[choice]; ok {
			scopes = append(scopes, scope)
		}
	}

	// Always add offline_access
	scopes = append(scopes, "offline_access")
	scopeStr := strings.Join(scopes, " ")

	// 5. Ask for domains
	fmt.Print("Domains for this account (comma-separated, e.g. company.com,subsidiary.com): ")
	if !scanner.Scan() {
		return fmt.Errorf("failed to read domains")
	}
	domainsInput := strings.TrimSpace(scanner.Text())
	var domains []string
	if domainsInput != "" {
		for _, d := range strings.Split(domainsInput, ",") {
			domain := strings.TrimSpace(d)
			if domain != "" {
				domains = append(domains, domain)
			}
		}
	}

	// 6. Create account and save to config
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

	// 7. Ask to login now
	fmt.Print("\nLogin now? [Y/n] ")
	if !scanner.Scan() {
		return nil // Just exit if we can't read
	}
	loginChoice := strings.ToLower(strings.TrimSpace(scanner.Text()))
	if loginChoice == "" || loginChoice == "y" || loginChoice == "yes" {
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
