package cal

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/larsc/md365/internal/auth"
	"github.com/larsc/md365/internal/config"
	"github.com/larsc/md365/internal/graph"
	"github.com/larsc/md365/internal/sync"
	"gopkg.in/yaml.v3"
)

// EventInfo represents parsed event information for listing
type EventInfo struct {
	Start    time.Time
	End      time.Time
	Subject  string
	Location string
	Account  string
	FilePath string
}

// List lists calendar events
func List(cfg *config.Config, fromDate, toDate time.Time, search, account string) error {
	// Determine which accounts to search
	var accounts []string
	if account != "" {
		accounts = []string{account}
	} else {
		accounts = cfg.ListAccounts()
	}

	// Collect events
	var events []EventInfo

	for _, acc := range accounts {
		calDir := filepath.Join(cfg.DataDir, acc, "calendar")
		if _, err := os.Stat(calDir); os.IsNotExist(err) {
			continue
		}

		err := filepath.Walk(calDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".md") {
				return nil
			}

			// Read file
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}

			// Apply search filter
			if search != "" && !strings.Contains(strings.ToLower(string(data)), strings.ToLower(search)) {
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
			startStr, ok := fm["start"].(string)
			if !ok {
				return nil
			}

			start, err := time.Parse(time.RFC3339, startStr)
			if err != nil {
				return nil
			}

			// Filter by date range
			if start.Before(fromDate) || start.After(toDate) {
				return nil
			}

			endStr, _ := fm["end"].(string)
			end, _ := time.Parse(time.RFC3339, endStr)

			subject, _ := fm["subject"].(string)
			location, _ := fm["location"].(string)

			events = append(events, EventInfo{
				Start:    start,
				End:      end,
				Subject:  subject,
				Location: location,
				Account:  acc,
				FilePath: path,
			})

			return nil
		})

		if err != nil {
			return fmt.Errorf("failed to walk calendar directory: %w", err)
		}
	}

	// Sort by start time
	sort.Slice(events, func(i, j int) bool {
		return events[i].Start.Before(events[j].Start)
	})

	// Display events
	for _, event := range events {
		startDate := event.Start.Format("2006-01-02 Mon")
		startTime := event.Start.Format("15:04")
		endTime := event.End.Format("15:04")

		line := fmt.Sprintf("%s %s-%s %-30s [%s]",
			startDate, startTime, endTime, truncate(event.Subject, 30), event.Account)

		if event.Location != "" {
			line += fmt.Sprintf(" 📍 %s", event.Location)
		}

		fmt.Println(line)
	}

	return nil
}

// Create creates a new calendar event
func Create(cfg *config.Config, account, subject, start, end, location, body string, attendees []string, force bool) error {
	// Check cross-tenant unless force is enabled
	if !force && len(attendees) > 0 {
		if err := cfg.CheckCrossTenant(account, attendees); err != nil {
			return err
		}
	}

	// Get access token
	token, err := auth.GetAccessToken(cfg, account)
	if err != nil {
		return err
	}

	// Create event
	client := graph.NewClient(token)

	event := &graph.Event{
		Subject: subject,
		Start: graph.DateTime{
			DateTime: start,
			TimeZone: "UTC",
		},
		End: graph.DateTime{
			DateTime: end,
			TimeZone: "UTC",
		},
	}

	if location != "" {
		event.Location = graph.Location{DisplayName: location}
	}

	if body != "" {
		event.Body = graph.Body{
			ContentType: "text",
			Content:     body,
		}
	}

	// Add attendees
	if len(attendees) > 0 {
		event.Attendees = make([]graph.Attendee, len(attendees))
		for i, email := range attendees {
			event.Attendees[i] = graph.Attendee{
				EmailAddress: graph.EmailAddress{
					Address: email,
				},
			}
		}
	}

	created, err := client.CreateEvent(event)
	if err != nil {
		return err
	}

	// Write to local file
	filePath, err := sync.WriteEventFile(cfg, account, created)
	if err != nil {
		return fmt.Errorf("event created but failed to write local file: %w", err)
	}

	fmt.Printf("Event created: %s\n", filePath)
	return nil
}

// Delete deletes a calendar event
func Delete(cfg *config.Config, account, id, filePath string) error {
	// If file provided, extract account and ID
	if filePath != "" {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		content := string(data)
		parts := strings.SplitN(content, "---", 3)
		if len(parts) < 3 {
			return fmt.Errorf("invalid frontmatter in file")
		}

		var fm map[string]interface{}
		if err := yaml.Unmarshal([]byte(parts[1]), &fm); err != nil {
			return fmt.Errorf("failed to parse frontmatter: %w", err)
		}

		var ok bool
		account, ok = fm["account"].(string)
		if !ok {
			return fmt.Errorf("account not found in frontmatter")
		}

		id, ok = fm["id"].(string)
		if !ok {
			return fmt.Errorf("id not found in frontmatter")
		}
	}

	if account == "" || id == "" {
		return fmt.Errorf("account and id are required")
	}

	// Get access token
	token, err := auth.GetAccessToken(cfg, account)
	if err != nil {
		return err
	}

	// Delete via API
	client := graph.NewClient(token)
	if err := client.DeleteEvent(id); err != nil {
		return err
	}

	// Delete local file
	if filePath != "" {
		if err := os.Remove(filePath); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to delete local file: %v\n", err)
		}
		fmt.Printf("Event deleted: %s\n", filePath)
	} else {
		// Find and delete file by ID
		calDir := filepath.Join(cfg.DataDir, account, "calendar")
		deleted := false

		filepath.Walk(calDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".md") {
				return nil
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}

			content := string(data)
			parts := strings.SplitN(content, "---", 3)
			if len(parts) < 3 {
				return nil
			}

			var fm map[string]interface{}
			if err := yaml.Unmarshal([]byte(parts[1]), &fm); err != nil {
				return nil
			}

			fileID, ok := fm["id"].(string)
			if ok && fileID == id {
				if err := os.Remove(path); err == nil {
					fmt.Printf("Event deleted: %s\n", path)
					deleted = true
				}
			}

			return nil
		})

		if !deleted {
			fmt.Println("Event deleted (local file not found)")
		}
	}

	return nil
}

// truncate truncates a string to a maximum length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
