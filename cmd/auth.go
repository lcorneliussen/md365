package cmd

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/lcorneliussen/md365/internal/auth"
	"github.com/lcorneliussen/md365/internal/capability"
	"github.com/lcorneliussen/md365/internal/config"
	"github.com/lcorneliussen/md365/internal/output"
	"github.com/spf13/cobra"
)

var (
	authAccount  string
	authScope    string
	authAddScope []string
	authCommands []string
	authFeatures []string

	// flags for auth add
	authAddName     string
	authAddHint     string
	authAddFlow     string
	authAddScopes   string
	authAddCommands []string
	authAddFeatures []string
	authAddDomains  string
	authAddLogin    bool
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
	RunE: func(cmd *cobra.Command, args []string) error {
		if authAccount == "" {
			return usageError("--account is required")
		}
		if !writer.IsHuman() {
			return usageError("auth login is interactive; run without machine-readable output flags")
		}
		if authScope != "" && (len(authCommands) > 0 || len(authFeatures) > 0) {
			return usageError("--scope cannot be combined with --command or --feature")
		}
		if len(authCommands) > 0 || len(authFeatures) > 0 {
			plan, err := capability.Resolve(authCommands, authFeatures)
			if err != nil {
				return usageError(err.Error())
			}
			authAddScope = append(authAddScope, plan.Scopes...)
		}

		if err := auth.DispatchLogin(cfg, authAccount, authScope, authAddScope); err != nil {
			return err
		}
		return nil
	},
}

var authPlanCmd = &cobra.Command{
	Use:   "plan",
	Short: "Plan least-privilege permissions",
	Long:  `Resolve command and feature selections to the minimal delegated Microsoft Graph scopes.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		plan, err := capability.Resolve(authCommands, authFeatures)
		if err != nil {
			return usageError(err.Error())
		}
		if writer.IsHuman() {
			fmt.Fprintln(cmd.OutOrStdout(), "Commands:")
			for _, command := range plan.Commands {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s: %s\n", command.Name, strings.Join(command.Scopes, " "))
			}
			fmt.Fprintln(cmd.OutOrStdout(), "\nRequired scopes:")
			for _, scope := range plan.Scopes {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", scope)
			}
			return nil
		}
		return writeOK(plan, output.WithSummary(fmt.Sprintf("%d commands require %d scopes", len(plan.Commands), len(plan.Scopes))))
	},
}

type authExplainResult struct {
	Account         string               `json:"account"`
	GrantedScopes   []string             `json:"granted_scopes"`
	AllowedCommands []capability.Command `json:"allowed_commands"`
	BlockedCommands []capability.Command `json:"blocked_commands"`
}

var authExplainCmd = &cobra.Command{
	Use:   "explain",
	Short: "Explain what an account can do",
	Long:  `Show which md365 commands are allowed or blocked by the account's current token scopes.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if authAccount == "" {
			return usageError("--account is required")
		}
		scopes, err := auth.Scopes(authAccount)
		if err != nil {
			return err
		}
		result := authExplainResult{Account: authAccount, GrantedScopes: scopes}
		for _, command := range capability.Commands() {
			if capability.Allowed(command, scopes) {
				result.AllowedCommands = append(result.AllowedCommands, command)
			} else {
				result.BlockedCommands = append(result.BlockedCommands, command)
			}
		}
		if writer.IsHuman() {
			fmt.Fprintf(cmd.OutOrStdout(), "Account '%s'\n", authAccount)
			fmt.Fprintf(cmd.OutOrStdout(), "Scopes: %s\n\n", strings.Join(scopes, " "))
			fmt.Fprintln(cmd.OutOrStdout(), "Allowed commands:")
			for _, command := range result.AllowedCommands {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", command.Name)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "\nBlocked commands:")
			for _, command := range result.BlockedCommands {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s (requires %s)\n", command.Name, strings.Join(command.Scopes, " "))
			}
			return nil
		}
		return writeOK(result, output.WithSummary(fmt.Sprintf("%d allowed, %d blocked", len(result.AllowedCommands), len(result.BlockedCommands))))
	},
}

// authStatusCmd represents the auth status command
var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show authentication status",
	Long:  `Show authentication status for all accounts.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		statuses := auth.Status(cfg)
		if writer.IsHuman() {
			fmt.Fprintln(cmd.OutOrStdout(), "Account authentication status:")
			fmt.Fprintln(cmd.OutOrStdout())
			for _, status := range statuses {
				if !status.Authenticated {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s: NOT AUTHENTICATED [%s]\n", status.Account, status.AuthFlow)
					continue
				}
				if status.Expired {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s: EXPIRED [%s]\n", status.Account, status.AuthFlow)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s: Valid (expires in %dh) [%s]\n", status.Account, status.ExpiresInHours, status.AuthFlow)
				}
				if len(status.Scopes) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "    Scopes: %s\n", strings.Join(status.Scopes, " "))
				}
				if len(status.AllowedCommands) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "    Available: %s\n", joinCommandNames(status.AllowedCommands))
				}
				if len(status.BlockedCommands) > 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "    Blocked:")
					for _, command := range status.BlockedCommands {
						fmt.Fprintf(cmd.OutOrStdout(), "      - %s (requires %s)\n", command.Name, strings.Join(command.Scopes, " "))
					}
				}
			}
			return nil
		}
		return writeOK(statuses, output.WithSummary(fmt.Sprintf("%d accounts", len(statuses))))
	},
}

func joinCommandNames(commands []capability.Command) string {
	names := make([]string, 0, len(commands))
	for _, command := range commands {
		names = append(names, command.Name)
	}
	return strings.Join(names, ", ")
}

// authRefreshCmd represents the auth refresh command
var authRefreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Refresh token",
	Long:  `Force refresh the access token for an account.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if authAccount == "" {
			return usageError("--account is required")
		}

		if err := auth.RefreshToken(cfg, authAccount); err != nil {
			return err
		}
		return writeOK(map[string]string{"account": authAccount}, output.WithSummary("Token refreshed successfully"))
	},
}

// authScopesCmd represents the auth scopes command
var authScopesCmd = &cobra.Command{
	Use:   "scopes",
	Short: "Show token scopes",
	Long:  `Display the scopes stored in the current token for an account.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if authAccount == "" {
			return usageError("--account is required")
		}

		scopes, err := auth.Scopes(authAccount)
		if err != nil {
			return err
		}
		if writer.IsHuman() {
			if len(scopes) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No scopes stored for account '%s'\n", authAccount)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Scopes for account '%s':\n", authAccount)
			for _, scope := range scopes {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", scope)
			}
			return nil
		}
		return writeOK(scopes, output.WithSummary(fmt.Sprintf("%d scopes", len(scopes))))
	},
}

// authAddCmd represents the auth add command
var authAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new account",
	Long: `Add a new account with authentication configuration.

Requires --name flag. Use --interactive for a guided TUI setup.

Examples:
  md365 auth add --name work --hint user@company.com --flow authcode --scopes "Calendars.ReadWrite,User.Read"
  md365 auth add --interactive`,
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

	if !Interactive && authAddName == "" {
		return fmt.Errorf("--name is required. Use --interactive for guided setup.\n\nExample: md365 auth add --name work --hint user@company.com")
	}

	if !Interactive {
		// Non-interactive mode: use flags
		accountName = strings.TrimSpace(authAddName)

		// Validate account name (used in file paths / keyring keys)
		if !regexp.MustCompile(`^[a-zA-Z0-9_-]+$`).MatchString(accountName) {
			return fmt.Errorf("account name must contain only letters, numbers, dashes, and underscores")
		}

		emailHint = strings.TrimSpace(authAddHint)

		// Validate and set auth flow
		authFlow = authAddFlow
		if authFlow == "" {
			authFlow = "devicecode"
		}
		if authFlow != "devicecode" && authFlow != "authcode" {
			return fmt.Errorf("invalid --flow: must be 'devicecode' or 'authcode'")
		}

		if authAddScopes != "" && (len(authAddCommands) > 0 || len(authAddFeatures) > 0) {
			return fmt.Errorf("--scopes cannot be combined with --command or --feature")
		}

		// Parse scopes from flags or resolve command/feature selections.
		if authAddScopes != "" {
			for _, s := range strings.Split(authAddScopes, ",") {
				scope := strings.TrimSpace(s)
				if scope != "" {
					scopeChoices = append(scopeChoices, scope)
				}
			}
		} else if len(authAddCommands) > 0 || len(authAddFeatures) > 0 {
			plan, err := capability.Resolve(authAddCommands, authAddFeatures)
			if err != nil {
				return err
			}
			scopeChoices = plan.Scopes
		} else {
			// Default scopes if not specified
			scopeChoices = []string{"Calendars.ReadWrite", "User.Read"}
		}

		domainsInput = authAddDomains
		loginNow = authAddLogin
	} else {
		// Interactive mode: show huh form
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
						huh.NewOption("Mail (read)", "Mail.Read"),
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
	}

	// Build scopes list
	var scopes []string
	scopes = append(scopes, scopeChoices...)
	// Always add offline_access
	scopes = append(scopes, "offline_access")
	scopes = capability.MinimalScopes(scopes)
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
	authLoginCmd.Flags().StringSliceVar(&authCommands, "command", []string{}, "Add permissions required by command(s), e.g. 'mail archive'")
	authLoginCmd.Flags().StringSliceVar(&authFeatures, "feature", []string{}, "Add permissions required by feature(s), e.g. mail-manage")
	authRefreshCmd.Flags().StringVar(&authAccount, "account", "", "Account name (required)")
	authScopesCmd.Flags().StringVar(&authAccount, "account", "", "Account name (required)")
	authPlanCmd.Flags().StringSliceVar(&authCommands, "command", []string{}, "Command selector(s), e.g. 'mail archive' or 'mail *'")
	authPlanCmd.Flags().StringSliceVar(&authFeatures, "feature", []string{}, "Feature bundle(s), e.g. mail-manage,calendar")
	authExplainCmd.Flags().StringVar(&authAccount, "account", "", "Account name (required)")

	// Flags for auth add (non-interactive mode)
	authAddCmd.Flags().StringVar(&authAddName, "name", "", "Account name (required)")
	authAddCmd.Flags().StringVar(&authAddHint, "hint", "", "Email hint (e.g., user@company.com)")
	authAddCmd.Flags().StringVar(&authAddFlow, "flow", "devicecode", "Auth flow: devicecode or authcode")
	authAddCmd.Flags().StringVar(&authAddScopes, "scopes", "", "Comma-separated scopes (e.g., Calendars.ReadWrite,User.Read)")
	authAddCmd.Flags().StringSliceVar(&authAddCommands, "command", []string{}, "Commands whose permissions should be configured")
	authAddCmd.Flags().StringSliceVar(&authAddFeatures, "feature", []string{}, "Feature bundles whose permissions should be configured")
	authAddCmd.Flags().StringVar(&authAddDomains, "domains", "", "Comma-separated domains (e.g., company.com,subsidiary.com)")
	authAddCmd.Flags().BoolVar(&authAddLogin, "login", false, "Auto-login after creating account")

	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authStatusCmd)
	authCmd.AddCommand(authRefreshCmd)
	authCmd.AddCommand(authScopesCmd)
	authCmd.AddCommand(authAddCmd)
	authCmd.AddCommand(authPlanCmd)
	authCmd.AddCommand(authExplainCmd)
}
