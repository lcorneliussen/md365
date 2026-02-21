package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/larsc/md365/internal/config"
)

const (
	deviceCodeURL = "https://login.microsoftonline.com/common/oauth2/v2.0/devicecode"
	tokenURL      = "https://login.microsoftonline.com/common/oauth2/v2.0/token"
	tokenBuffer   = 5 * time.Minute // Auto-refresh 5 minutes before expiry
)

// Token represents an OAuth2 token
type Token struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresOn    int64  `json:"expires_on"`
	Scope        string `json:"scope"`
}

// DeviceCodeResponse represents the device code flow response
type DeviceCodeResponse struct {
	UserCode        string `json:"user_code"`
	DeviceCode      string `json:"device_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
	Message         string `json:"message"`
}

// TokenResponse represents the token response
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Error        string `json:"error,omitempty"`
	ErrorDesc    string `json:"error_description,omitempty"`
}

// GetAccessToken returns a valid access token for the account, refreshing if needed
func GetAccessToken(cfg *config.Config, account string) (string, error) {
	tokenFile := getTokenPath(account)

	token, err := loadToken(tokenFile)
	if err != nil {
		return "", fmt.Errorf("no token found for account '%s'. Run: md365 auth login --account %s", account, account)
	}

	// Check if token needs refresh
	if time.Now().Add(tokenBuffer).Unix() >= token.ExpiresOn {
		fmt.Fprintf(os.Stderr, "Refreshing token for account '%s'...\n", account)
		if err := RefreshToken(cfg, account); err != nil {
			return "", fmt.Errorf("failed to refresh token: %w", err)
		}
		// Reload token after refresh
		token, err = loadToken(tokenFile)
		if err != nil {
			return "", err
		}
	}

	return token.AccessToken, nil
}

// RefreshToken refreshes the access token for an account
func RefreshToken(cfg *config.Config, account string) error {
	tokenFile := getTokenPath(account)

	token, err := loadToken(tokenFile)
	if err != nil {
		return fmt.Errorf("no token found for account '%s'", account)
	}

	data := url.Values{
		"client_id":     {cfg.GetClientID(account)},
		"scope":         {token.Scope},
		"refresh_token": {token.RefreshToken},
		"grant_type":    {"refresh_token"},
	}

	resp, err := http.PostForm(tokenURL, data)
	if err != nil {
		return fmt.Errorf("failed to refresh token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if tokenResp.Error != "" {
		return fmt.Errorf("error refreshing token: %s - %s", tokenResp.Error, tokenResp.ErrorDesc)
	}

	// Save new token
	newToken := Token{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresOn:    time.Now().Unix() + int64(tokenResp.ExpiresIn),
		Scope:        token.Scope,
	}

	if err := saveToken(tokenFile, &newToken); err != nil {
		return fmt.Errorf("failed to save token: %w", err)
	}

	fmt.Fprintln(os.Stderr, "Token refreshed successfully")
	return nil
}

// Login performs device code flow authentication
func Login(cfg *config.Config, account string) error {
	acc, err := cfg.GetAccount(account)
	if err != nil {
		return err
	}

	fmt.Printf("Initiating device code flow for account '%s'...\n", account)

	// Start device code flow
	data := url.Values{
		"client_id": {cfg.GetClientID(account)},
		"scope":     {acc.Scope},
	}

	resp, err := http.PostForm(deviceCodeURL, data)
	if err != nil {
		return fmt.Errorf("failed to initiate device code flow: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	var deviceResp DeviceCodeResponse
	if err := json.Unmarshal(body, &deviceResp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	fmt.Println()
	fmt.Println("To sign in, use a web browser to open:")
	fmt.Printf("  %s\n", deviceResp.VerificationURI)
	fmt.Println()
	fmt.Println("And enter the code:")
	fmt.Printf("  %s\n", deviceResp.UserCode)
	fmt.Println()
	if acc.Hint != "" {
		fmt.Printf("Account hint: %s\n", acc.Hint)
		fmt.Println()
	}
	fmt.Println("Waiting for authentication...")

	// Poll for token
	interval := time.Duration(deviceResp.Interval) * time.Second
	if interval == 0 {
		interval = 5 * time.Second
	}
	timeout := time.Now().Add(time.Duration(deviceResp.ExpiresIn) * time.Second)

	for time.Now().Before(timeout) {
		time.Sleep(interval)

		tokenData := url.Values{
			"client_id":   {cfg.GetClientID(account)},
			"device_code": {deviceResp.DeviceCode},
			"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
		}

		tokenResp, err := http.PostForm(tokenURL, tokenData)
		if err != nil {
			return fmt.Errorf("failed to poll for token: %w", err)
		}

		tokenBody, err := io.ReadAll(tokenResp.Body)
		tokenResp.Body.Close()
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}

		var token TokenResponse
		if err := json.Unmarshal(tokenBody, &token); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		switch token.Error {
		case "authorization_pending":
			continue
		case "slow_down":
			interval += 5 * time.Second
			continue
		case "":
			// Success!
			newToken := Token{
				AccessToken:  token.AccessToken,
				RefreshToken: token.RefreshToken,
				ExpiresOn:    time.Now().Unix() + int64(token.ExpiresIn),
				Scope:        acc.Scope,
			}

			tokenFile := getTokenPath(account)
			if err := saveToken(tokenFile, &newToken); err != nil {
				return fmt.Errorf("failed to save token: %w", err)
			}

			fmt.Println()
			fmt.Printf("Successfully authenticated account '%s'\n", account)
			return nil
		default:
			return fmt.Errorf("error: %s - %s", token.Error, token.ErrorDesc)
		}
	}

	return fmt.Errorf("authentication timed out")
}

// Status shows authentication status for all accounts
func Status(cfg *config.Config) {
	fmt.Println("Account authentication status:")
	fmt.Println()

	for _, account := range cfg.ListAccounts() {
		tokenFile := getTokenPath(account)

		token, err := loadToken(tokenFile)
		if err != nil {
			fmt.Printf("  %s: NOT AUTHENTICATED\n", account)
			continue
		}

		if token.ExpiresOn > time.Now().Unix() {
			remaining := time.Duration(token.ExpiresOn-time.Now().Unix()) * time.Second
			hours := int(remaining.Hours())
			fmt.Printf("  %s: Valid (expires in %dh)\n", account, hours)
		} else {
			fmt.Printf("  %s: EXPIRED\n", account)
		}
	}
}

// getTokenPath returns the path to the token file for an account
func getTokenPath(account string) string {
	return filepath.Join(config.GetTokenDir(), account+".json")
}

// loadToken loads a token from disk
func loadToken(path string) (*Token, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var token Token
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, err
	}

	return &token, nil
}

// saveToken saves a token to disk (atomic write)
func saveToken(path string, token *Token) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return err
	}

	// Write to temp file first
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}

	// Atomic rename
	return os.Rename(tmpPath, path)
}

// Slugify converts text to a filename-safe slug
func Slugify(text string, maxLen int) string {
	text = strings.ToLower(text)

	// Replace non-alphanumeric with dashes
	var builder strings.Builder
	for _, r := range text {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
		} else {
			builder.WriteRune('-')
		}
	}

	slug := builder.String()

	// Remove consecutive dashes
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}

	// Trim dashes
	slug = strings.Trim(slug, "-")

	// Truncate
	if len(slug) > maxLen {
		slug = slug[:maxLen]
	}

	return slug
}

// GenerateUniqueFilename generates a unique filename by appending numbers if needed
func GenerateUniqueFilename(dir, baseName, ext string) string {
	filename := baseName + ext
	path := filepath.Join(dir, filename)

	counter := 2
	for {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return filename
		}
		filename = fmt.Sprintf("%s-%d%s", baseName, counter, ext)
		path = filepath.Join(dir, filename)
		counter++
	}
}
