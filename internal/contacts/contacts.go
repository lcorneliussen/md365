package contacts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/larsc/md365/internal/config"
	"gopkg.in/yaml.v3"
)

// Search searches for contacts matching a query
func Search(cfg *config.Config, query, account string) error {
	// Determine which accounts to search
	var accounts []string
	if account != "" {
		accounts = []string{account}
	} else {
		accounts = cfg.ListAccounts()
	}

	queryLower := strings.ToLower(query)

	for _, acc := range accounts {
		contactDir := filepath.Join(cfg.DataDir, acc, "contacts")
		if _, err := os.Stat(contactDir); os.IsNotExist(err) {
			continue
		}

		err := filepath.Walk(contactDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".md") {
				return nil
			}

			// Read file
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}

			// Search in file content
			if !strings.Contains(strings.ToLower(string(data)), queryLower) {
				return nil
			}

			// Parse frontmatter
			content := string(data)
			parts := strings.SplitN(content, "---", 3)
			if len(parts) < 3 {
				return nil
			}

			var fm map[string]interface{}
			if err := yaml.Unmarshal([]byte(parts[1]), &fm); err != nil {
				return nil
			}

			// Extract fields
			displayName, _ := fm["display_name"].(string)

			// Get first email if available
			email := ""
			if emails, ok := fm["emails"].([]interface{}); ok && len(emails) > 0 {
				if e, ok := emails[0].(string); ok {
					email = e
				}
			}

			// Display contact
			line := fmt.Sprintf("[%s] %s", acc, displayName)
			if email != "" {
				line += fmt.Sprintf(" <%s>", email)
			}

			fmt.Println(line)

			return nil
		})

		if err != nil {
			return fmt.Errorf("failed to walk contacts directory: %w", err)
		}
	}

	return nil
}
