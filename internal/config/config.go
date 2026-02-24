package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// DefaultClientID is the official md365 app registration
const DefaultClientID = "98a465bc-fdca-4ea6-a3b9-a4b819e50a86"

// Config represents the application configuration
type Config struct {
	ClientID string              `yaml:"client_id"`
	DataDir  string              `yaml:"data_dir"`
	Timezone string              `yaml:"timezone"`
	Accounts map[string]*Account `yaml:"accounts"`
}

// Account represents an account configuration
type Account struct {
	ClientID string   `yaml:"client_id"`
	AuthFlow string   `yaml:"auth_flow"`
	Hint     string   `yaml:"hint"`
	Scope    string   `yaml:"scope"`
	Domains  []string `yaml:"domains"`
}

// GetClientID returns the account-specific client_id, falling back to global
func (c *Config) GetClientID(accountName string) string {
	if acc, ok := c.Accounts[accountName]; ok && acc.ClientID != "" {
		return acc.ClientID
	}
	return c.ClientID
}

// GetAuthFlow returns the auth_flow for an account (default: "devicecode")
func (c *Config) GetAuthFlow(accountName string) string {
	if acc, ok := c.Accounts[accountName]; ok && acc.AuthFlow != "" {
		return acc.AuthFlow
	}
	return "devicecode"
}

var (
	configDir  string
	configFile string
	dataDir    string
)

func init() {
	// Set up config directory
	xdgConfig := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfig == "" {
		xdgConfig = filepath.Join(os.Getenv("HOME"), ".config")
	}
	configDir = filepath.Join(xdgConfig, "md365")
	configFile = filepath.Join(configDir, "config.yaml")

	// Set up data directory
	xdgData := os.Getenv("XDG_DATA_HOME")
	if xdgData == "" {
		xdgData = filepath.Join(os.Getenv("HOME"), ".local", "share")
	}
	dataDir = filepath.Join(xdgData, "md365")
}

// Load reads and parses the configuration file
func Load() (*Config, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("configuration file not found: %s\n\nCreate %s with:\n"+
				"client_id: YOUR_CLIENT_ID\n"+
				"timezone: Europe/Berlin\n"+
				"accounts:\n"+
				"  myaccount:\n"+
				"    hint: user@example.com\n"+
				"    scope: Calendars.ReadWrite Contacts.ReadWrite User.Read Mail.Send",
				configFile, configFile)
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Default to official md365 app registration if no client_id configured
	if cfg.ClientID == "" {
		cfg.ClientID = DefaultClientID
	}

	// Set default timezone
	if cfg.Timezone == "" {
		cfg.Timezone = "UTC"
	}

	// Expand data_dir if custom
	if cfg.DataDir != "" {
		cfg.DataDir = expandTilde(cfg.DataDir)
	} else {
		cfg.DataDir = dataDir
	}

	return &cfg, nil
}

// expandTilde expands ~ to home directory
func expandTilde(path string) string {
	if strings.HasPrefix(path, "~") {
		return filepath.Join(os.Getenv("HOME"), path[1:])
	}
	return path
}

// GetConfigDir returns the configuration directory path
func GetConfigDir() string {
	return configDir
}

// GetDataDir returns the default data directory path
func GetDataDir() string {
	return dataDir
}

// ListAccounts returns a list of account names
func (c *Config) ListAccounts() []string {
	accounts := make([]string, 0, len(c.Accounts))
	for name := range c.Accounts {
		accounts = append(accounts, name)
	}
	return accounts
}

// GetAccount returns an account by name
func (c *Config) GetAccount(name string) (*Account, error) {
	acc, ok := c.Accounts[name]
	if !ok {
		return nil, fmt.Errorf("account '%s' not found in config", name)
	}
	return acc, nil
}

// CheckCrossTenant validates recipient emails against account domains
// Returns error if recipient belongs to another account's domain
// Returns warning (but allows) if domain is unknown
func (c *Config) CheckCrossTenant(account string, recipientEmails []string) error {
	if len(recipientEmails) == 0 {
		return nil
	}

	currentAccount, err := c.GetAccount(account)
	if err != nil {
		return err
	}

	for _, email := range recipientEmails {
		domain := extractDomain(email)
		if domain == "" {
			continue
		}

		// Check if domain belongs to current account
		if containsDomain(currentAccount.Domains, domain) {
			continue
		}

		// Check if domain belongs to another account
		for otherAccount, otherConfig := range c.Accounts {
			if otherAccount == account {
				continue
			}

			if containsDomain(otherConfig.Domains, domain) {
				return fmt.Errorf("recipient %s belongs to context '%s'. Use --account %s instead. Override with --force",
					email, otherAccount, otherAccount)
			}
		}

		// Unknown domain - warn but proceed
		fmt.Fprintf(os.Stderr, "Warning: Unknown domain %s for %s. Proceeding with account '%s'.\n",
			domain, email, account)
	}

	return nil
}

// extractDomain extracts domain from email address
func extractDomain(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(parts[1]))
}

// containsDomain checks if domain is in the list (case-insensitive)
func containsDomain(domains []string, domain string) bool {
	for _, d := range domains {
		if strings.EqualFold(d, domain) {
			return true
		}
	}
	return false
}

// SaveAccount adds or updates an account in the configuration file
func SaveAccount(name string, account *Account) error {
	// Load existing config or create minimal one
	cfg, err := Load()
	if err != nil {
		// Create a new config if file doesn't exist
		cfg = &Config{
			ClientID: DefaultClientID,
			Timezone: "Europe/Berlin",
			Accounts: make(map[string]*Account),
		}
	}

	// Initialize accounts map if needed
	if cfg.Accounts == nil {
		cfg.Accounts = make(map[string]*Account)
	}

	// Add or update account
	cfg.Accounts[name] = account

	// Ensure config directory exists
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal to YAML
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(configFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
