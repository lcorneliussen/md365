package contacts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lcorneliussen/md365/internal/auth"
	"github.com/lcorneliussen/md365/internal/config"
	"github.com/lcorneliussen/md365/internal/graph"
	"gopkg.in/yaml.v3"
)

type ContactInfo struct {
	ID          string   `json:"id,omitempty"`
	Account     string   `json:"account"`
	DisplayName string   `json:"display_name"`
	Emails      []string `json:"emails,omitempty"`
	Phones      []string `json:"phones,omitempty"`
	Company     string   `json:"company,omitempty"`
	JobTitle    string   `json:"job_title,omitempty"`
	FilePath    string   `json:"file_path,omitempty"`
}

// Search searches for contacts matching a query
func Search(cfg *config.Config, query, account string, noCache bool) ([]ContactInfo, error) {
	// Determine which accounts to search
	var accounts []string
	if account != "" {
		accounts = []string{account}
	} else {
		accounts = cfg.ListAccounts()
	}

	if noCache {
		return searchLive(cfg, query, accounts)
	}

	queryLower := strings.ToLower(query)
	results := []ContactInfo{}

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
			id, _ := fm["id"].(string)
			company, _ := fm["company"].(string)
			jobTitle, _ := fm["job_title"].(string)

			results = append(results, ContactInfo{
				ID:          id,
				Account:     acc,
				DisplayName: displayName,
				Emails:      stringSliceFromFrontmatter(fm["emails"]),
				Phones:      stringSliceFromFrontmatter(fm["phones"]),
				Company:     company,
				JobTitle:    jobTitle,
				FilePath:    path,
			})

			return nil
		})

		if err != nil {
			return nil, fmt.Errorf("failed to walk contacts directory: %w", err)
		}
	}

	return results, nil
}

func searchLive(cfg *config.Config, query string, accounts []string) ([]ContactInfo, error) {
	queryLower := strings.ToLower(query)
	results := []ContactInfo{}

	for _, acc := range accounts {
		token, err := auth.GetAccessToken(cfg, acc)
		if err != nil {
			return nil, err
		}

		client := graph.NewClient(token)
		contacts, _, err := client.GetContactsDelta("")
		if err != nil {
			return nil, fmt.Errorf("failed to get contacts for '%s': %w", acc, err)
		}

		for _, contact := range contacts {
			if contact.Removed != nil {
				continue
			}
			if !strings.Contains(strings.ToLower(contactSearchText(contact)), queryLower) {
				continue
			}

			results = append(results, contactInfoFromGraph(acc, contact))
		}
	}

	return results, nil
}

func contactInfoFromGraph(account string, contact graph.Contact) ContactInfo {
	emails := make([]string, 0, len(contact.EmailAddresses))
	for _, email := range contact.EmailAddresses {
		emails = append(emails, email.Address)
	}
	phones := append([]string{}, contact.BusinessPhones...)
	phones = append(phones, contact.HomePhones...)
	if contact.MobilePhone != "" {
		phones = append(phones, contact.MobilePhone)
	}
	return ContactInfo{
		ID:          contact.ID,
		Account:     account,
		DisplayName: contact.DisplayName,
		Emails:      emails,
		Phones:      phones,
		Company:     contact.CompanyName,
		JobTitle:    contact.JobTitle,
	}
}

func contactSearchText(contact graph.Contact) string {
	parts := []string{
		contact.DisplayName,
		contact.GivenName,
		contact.Surname,
		contact.CompanyName,
		contact.JobTitle,
		contact.Birthday,
		contact.MobilePhone,
	}
	for _, email := range contact.EmailAddresses {
		parts = append(parts, email.Name, email.Address)
	}
	parts = append(parts, contact.BusinessPhones...)
	parts = append(parts, contact.HomePhones...)
	return strings.Join(parts, "\n")
}

func stringSliceFromFrontmatter(value any) []string {
	raw, ok := value.([]interface{})
	if !ok {
		return nil
	}
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}
