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
	"sort"
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
	Scope        string `json:"scope,omitempty"`
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

	// Save new token - use granted scopes from response, fallback to existing if not provided
	grantedScope := tokenResp.Scope
	if grantedScope == "" {
		grantedScope = token.Scope
	}

	newToken := Token{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresOn:    time.Now().Unix() + int64(tokenResp.ExpiresIn),
		Scope:        grantedScope,
	}

	if err := saveToken(account, &newToken); err != nil {
		return fmt.Errorf("failed to save token: %w", err)
	}

	fmt.Fprintln(os.Stderr, "Token refreshed successfully")
	return nil
}

// Login performs device code flow authentication
func Login(cfg *config.Config, account string, scope string) error {
	acc, err := cfg.GetAccount(account)
	if err != nil {
		return err
	}

	fmt.Printf("Initiating device code flow for account '%s'...\n", account)

	// Start device code flow
	data := url.Values{
		"client_id": {cfg.GetClientID(account)},
		"scope":     {scope},
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
			// Success! Use granted scopes from response, fallback to requested if not provided
			grantedScope := token.Scope
			if grantedScope == "" {
				grantedScope = scope
			}

			newToken := Token{
				AccessToken:  token.AccessToken,
				RefreshToken: token.RefreshToken,
				ExpiresOn:    time.Now().Unix() + int64(token.ExpiresIn),
				Scope:        grantedScope,
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
func DispatchLogin(cfg *config.Config, account string, scopeOverride string, addScopes []string) error {
	// Determine final scopes based on priority
	var finalScope string

	if scopeOverride != "" {
		// --scope flag takes precedence over everything
		finalScope = mergeScopes(parseScopes(scopeOverride))
	} else if len(addScopes) > 0 {
		// --add-scope merges with existing token scopes (if any) or config scopes
		var baseScopes []string

		// Try to get existing token scopes
		token, err := loadToken(account)
		if err == nil && token.Scope != "" {
			baseScopes = parseScopes(token.Scope)
		} else {
			// Fall back to config scopes
			acc, err := cfg.GetAccount(account)
			if err != nil {
				return err
			}
			baseScopes = parseScopes(acc.Scope)
		}

		// Parse addScopes in case user passed space-separated scopes in one flag
		var parsedAddScopes []string
		for _, s := range addScopes {
			parsedAddScopes = append(parsedAddScopes, parseScopes(s)...)
		}
		finalScope = mergeScopes(baseScopes, parsedAddScopes)
	} else {
		// No flags: use config scope (current behavior)
		acc, err := cfg.GetAccount(account)
		if err != nil {
			return err
		}
		finalScope = mergeScopes(parseScopes(acc.Scope))
	}

	authFlow := cfg.GetAuthFlow(account)
	switch authFlow {
	case "authcode":
		return LoginAuthCode(cfg, account, finalScope)
	case "devicecode":
		return Login(cfg, account, finalScope)
	default:
		return fmt.Errorf("unknown auth_flow '%s' for account '%s'. Valid values: devicecode, authcode", authFlow, account)
	}
}

// LoginAuthCode performs authorization code flow with PKCE
func LoginAuthCode(cfg *config.Config, account string, scope string) error {
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

	redirectURI := fmt.Sprintf("http://localhost:%d", port)

	// Build authorization URL
	authURL, err := url.Parse(authorizeURL)
	if err != nil {
		return fmt.Errorf("failed to parse authorize URL: %w", err)
	}

	params := url.Values{
		"client_id":             {cfg.GetClientID(account)},
		"response_type":         {"code"},
		"redirect_uri":          {redirectURI},
		"scope":                 {scope},
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
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
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

	// Save token - use granted scopes from response, fallback to requested if not provided
	grantedScope := tokenResp.Scope
	if grantedScope == "" {
		grantedScope = scope
	}

	newToken := Token{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresOn:    time.Now().Unix() + int64(tokenResp.ExpiresIn),
		Scope:        grantedScope,
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
		authFlow := cfg.GetAuthFlow(account)
		token, err := loadToken(account)
		if err != nil {
			fmt.Printf("  %s: NOT AUTHENTICATED [%s]\n", account, authFlow)
			continue
		}

		if token.ExpiresOn > time.Now().Unix() {
			remaining := time.Duration(token.ExpiresOn-time.Now().Unix()) * time.Second
			hours := int(remaining.Hours())
			fmt.Printf("  %s: Valid (expires in %dh) [%s]\n", account, hours, authFlow)
			// Show scopes
			if token.Scope != "" {
				fmt.Printf("    Scopes: %s\n", token.Scope)
			}
		} else {
			fmt.Printf("  %s: EXPIRED [%s]\n", account, authFlow)
			// Show scopes even if expired
			if token.Scope != "" {
				fmt.Printf("    Scopes: %s\n", token.Scope)
			}
		}
	}
}

// tokenFilePath returns the file path for file-based token storage
func tokenFilePath(account string) string {
	xdgConfig := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfig == "" {
		xdgConfig = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(xdgConfig, "md365", "tokens", account+".json")
}

// loadToken loads a token from keyring, falling back to file
func loadToken(account string) (*Token, error) {
	// Try keyring first
	tokenJSON, err := keyring.Get(keyringService, account)
	if err == nil {
		var token Token
		if err := json.Unmarshal([]byte(tokenJSON), &token); err != nil {
			return nil, fmt.Errorf("corrupted token in keyring for '%s': %w", account, err)
		}
		return &token, nil
	}

	// Fall back to file
	data, fileErr := os.ReadFile(tokenFilePath(account))
	if fileErr != nil {
		return nil, fmt.Errorf("no token found for '%s' (keyring: %w)", account, err)
	}

	var token Token
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("corrupted token file for '%s': %w", account, err)
	}
	return &token, nil
}

// saveToken saves a token to keyring, falling back to file
func saveToken(account string, token *Token) error {
	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return err
	}

	// Try keyring first
	if err := keyring.Set(keyringService, account, string(data)); err != nil {
		// Fall back to file storage
		fmt.Fprintf(os.Stderr, "Warning: keyring storage failed, using file fallback: %v\n", err)
		return saveTokenFile(account, data)
	}

	return nil
}

// saveTokenFile saves token data to a file with restricted permissions
func saveTokenFile(account string, data []byte) error {
	path := tokenFilePath(account)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create token directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write token file: %w", err)
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

// parseScopes splits a scope string into individual scopes
func parseScopes(scopeStr string) []string {
	if scopeStr == "" {
		return []string{}
	}
	scopes := strings.Fields(scopeStr)
	result := make([]string, 0, len(scopes))
	for _, s := range scopes {
		s = strings.TrimSpace(s)
		if s != "" {
			result = append(result, s)
		}
	}
	return result
}

// normalizeScope converts scope to lowercase for comparison
func normalizeScope(scope string) string {
	return strings.ToLower(strings.TrimSpace(scope))
}

// mergeScopes merges multiple scope lists, deduplicating (case-insensitive)
// Always ensures offline_access is included
func mergeScopes(scopeLists ...[]string) string {
	seen := make(map[string]string) // normalized -> original
	for _, scopes := range scopeLists {
		for _, scope := range scopes {
			normalized := normalizeScope(scope)
			if _, exists := seen[normalized]; !exists {
				seen[normalized] = scope
			}
		}
	}

	// Ensure offline_access is present
	if _, exists := seen["offline_access"]; !exists {
		seen["offline_access"] = "offline_access"
	}

	// Build result maintaining original case, sorted for deterministic output
	result := make([]string, 0, len(seen))
	for _, original := range seen {
		result = append(result, original)
	}
	sort.Strings(result)

	return strings.Join(result, " ")
}

// ShowScopes displays the scopes for an account
func ShowScopes(account string) error {
	token, err := loadToken(account)
	if err != nil {
		fmt.Printf("No token found for account '%s'\n", account)
		fmt.Printf("Run: md365 auth login --account %s\n", account)
		return nil
	}

	scopes := parseScopes(token.Scope)
	if len(scopes) == 0 {
		fmt.Printf("No scopes stored for account '%s'\n", account)
		return nil
	}

	fmt.Printf("Scopes for account '%s':\n", account)
	for _, scope := range scopes {
		fmt.Printf("  - %s\n", scope)
	}

	return nil
}
