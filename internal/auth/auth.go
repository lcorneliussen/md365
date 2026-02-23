package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/lcorneliussen/md365/internal/config"
	"github.com/zalando/go-keyring"
)

const (
	deviceCodeURL  = "https://login.microsoftonline.com/common/oauth2/v2.0/devicecode"
	authorizeURL   = "https://login.microsoftonline.com/common/oauth2/v2.0/authorize"
	tokenURL       = "https://login.microsoftonline.com/common/oauth2/v2.0/token"
	tokenBuffer    = 5 * time.Minute // Auto-refresh 5 minutes before expiry
	keyringService = "md365"         // Service name for keyring storage
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
	token, err := loadToken(account)
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
		token, err = loadToken(account)
		if err != nil {
			return "", err
		}
	}

	return token.AccessToken, nil
}

// RefreshToken refreshes the access token for an account
func RefreshToken(cfg *config.Config, account string) error {
	token, err := loadToken(account)
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

	if err := saveToken(account, &newToken); err != nil {
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

	// Build direct login URL with pre-filled code
	directURL := fmt.Sprintf("%s?otc=%s", deviceResp.VerificationURI, deviceResp.UserCode)

	fmt.Println()
	fmt.Println("Open this link to sign in (code is pre-filled):")
	fmt.Printf("  %s\n", directURL)
	fmt.Println()
	fmt.Printf("Or go to %s and enter code: %s\n", deviceResp.VerificationURI, deviceResp.UserCode)
	if acc.Hint != "" {
		fmt.Printf("Account hint: %s\n", acc.Hint)
	}
	fmt.Println()
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

			if err := saveToken(account, &newToken); err != nil {
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

// generateCodeVerifier generates a PKCE code verifier (43-128 chars, URL-safe)
func generateCodeVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// generateCodeChallenge generates a PKCE code challenge (SHA256, base64url)
func generateCodeChallenge(verifier string) string {
	h := sha256.New()
	h.Write([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

// getFreePort finds a free port on localhost
func getFreePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

// DispatchLogin performs authentication using the configured flow for the account
func DispatchLogin(cfg *config.Config, account string) error {
	authFlow := cfg.GetAuthFlow(account)
	switch authFlow {
	case "authcode":
		return LoginAuthCode(cfg, account)
	case "devicecode":
		return Login(cfg, account)
	default:
		return fmt.Errorf("unknown auth_flow '%s' for account '%s'. Valid values: devicecode, authcode", authFlow, account)
	}
}

// LoginAuthCode performs authorization code flow with PKCE
func LoginAuthCode(cfg *config.Config, account string) error {
	acc, err := cfg.GetAccount(account)
	if err != nil {
		return err
	}

	fmt.Printf("Initiating authorization code flow for account '%s'...\n", account)

	// Generate PKCE parameters
	codeVerifier, err := generateCodeVerifier()
	if err != nil {
		return fmt.Errorf("failed to generate code verifier: %w", err)
	}
	codeChallenge := generateCodeChallenge(codeVerifier)

	// Get a free port
	port, err := getFreePort()
	if err != nil {
		return fmt.Errorf("failed to find free port: %w", err)
	}

	redirectURI := fmt.Sprintf("http://localhost:%d/callback", port)

	// Build authorization URL
	authURL, err := url.Parse(authorizeURL)
	if err != nil {
		return fmt.Errorf("failed to parse authorize URL: %w", err)
	}

	params := url.Values{
		"client_id":             {cfg.GetClientID(account)},
		"response_type":         {"code"},
		"redirect_uri":          {redirectURI},
		"scope":                 {acc.Scope},
		"code_challenge":        {codeChallenge},
		"code_challenge_method": {"S256"},
	}
	if acc.Hint != "" {
		params.Set("login_hint", acc.Hint)
	}
	authURL.RawQuery = params.Encode()

	// Channel to receive authorization code or error
	resultCh := make(chan string, 1)
	errorCh := make(chan error, 1)

	// Create HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		errParam := r.URL.Query().Get("error")

		if errParam != "" {
			errDesc := r.URL.Query().Get("error_description")
			errorCh <- fmt.Errorf("authorization error: %s - %s", errParam, errDesc)
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, "<html><body><h1>Authentication failed</h1><p>%s: %s</p><p>You can close this tab.</p></body></html>", errParam, errDesc)
			return
		}

		if code == "" {
			errorCh <- fmt.Errorf("no authorization code received")
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, "<html><body><h1>Authentication failed</h1><p>No authorization code received.</p><p>You can close this tab.</p></body></html>")
			return
		}

		resultCh <- code
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, "<html><body><h1>Authentication successful</h1><p>You can close this tab.</p></body></html>")
	})

	server := &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", port),
		Handler: mux,
	}

	// Start server in background
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errorCh <- fmt.Errorf("server error: %w", err)
		}
	}()

	// Ensure server shutdown
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}()

	// Print URL and try to open browser
	fmt.Println()
	fmt.Println("Opening browser for authentication...")
	fmt.Printf("  %s\n", authURL.String())
	fmt.Println()
	fmt.Println("If the browser doesn't open, copy the URL above into your browser.")
	if acc.Hint != "" {
		fmt.Printf("Account hint: %s\n", acc.Hint)
	}
	fmt.Println()
	fmt.Println("Waiting for authentication...")

	// Try to open browser (Linux only, ignore errors)
	exec.Command("xdg-open", authURL.String()).Start()

	// Wait for callback with timeout (~900s to match device code flow)
	timeout := time.After(900 * time.Second)

	var authCode string
	select {
	case authCode = <-resultCh:
		// Success, continue
	case err := <-errorCh:
		return err
	case <-timeout:
		return fmt.Errorf("authentication timed out")
	}

	// Exchange code for token
	tokenData := url.Values{
		"client_id":     {cfg.GetClientID(account)},
		"grant_type":    {"authorization_code"},
		"code":          {authCode},
		"redirect_uri":  {redirectURI},
		"code_verifier": {codeVerifier},
	}

	resp, err := http.PostForm(tokenURL, tokenData)
	if err != nil {
		return fmt.Errorf("failed to exchange code for token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read token response: %w", err)
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return fmt.Errorf("failed to parse token response: %w", err)
	}

	if tokenResp.Error != "" {
		return fmt.Errorf("token error: %s - %s", tokenResp.Error, tokenResp.ErrorDesc)
	}

	// Save token
	newToken := Token{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresOn:    time.Now().Unix() + int64(tokenResp.ExpiresIn),
		Scope:        acc.Scope,
	}

	if err := saveToken(account, &newToken); err != nil {
		return fmt.Errorf("failed to save token: %w", err)
	}

	fmt.Println()
	fmt.Printf("Successfully authenticated account '%s'\n", account)
	return nil
}

// Status shows authentication status for all accounts
func Status(cfg *config.Config) {
	fmt.Println("Account authentication status:")
	fmt.Println()

	for _, account := range cfg.ListAccounts() {
		token, err := loadToken(account)
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

// loadToken loads a token from keyring
func loadToken(account string) (*Token, error) {
	tokenJSON, err := keyring.Get(keyringService, account)
	if err != nil {
		return nil, fmt.Errorf("no token in keyring for '%s': %w", account, err)
	}

	var token Token
	if err := json.Unmarshal([]byte(tokenJSON), &token); err != nil {
		return nil, fmt.Errorf("corrupted token in keyring for '%s': %w", account, err)
	}

	return &token, nil
}

// saveToken saves a token to keyring
func saveToken(account string, token *Token) error {
	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return err
	}

	if err := keyring.Set(keyringService, account, string(data)); err != nil {
		return fmt.Errorf("failed to save token to keyring: %w\n\nEnsure a keyring daemon (e.g. gnome-keyring, kwallet) is running", err)
	}

	return nil
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

// DeleteToken removes a token from keyring
func DeleteToken(account string) error {
	return keyring.Delete(keyringService, account)
}
