package cal

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/lcorneliussen/md365/internal/auth"
	"github.com/lcorneliussen/md365/internal/config"
	"github.com/lcorneliussen/md365/internal/graph"
	"github.com/lcorneliussen/md365/internal/sync"
	"gopkg.in/yaml.v3"
)

// EventInfo represents parsed event information for listing
type EventInfo struct {
	ID       string    `json:"id,omitempty"`
	Start    time.Time `json:"start"`
	End      time.Time `json:"end"`
	Subject  string    `json:"subject"`
	Location string    `json:"location,omitempty"`
	Account  string    `json:"account"`
	FilePath string    `json:"file_path,omitempty"`
}

// List lists calendar events
func List(cfg *config.Config, fromDate, toDate time.Time, search, account string, noCache bool) ([]EventInfo, error) {
	// Determine which accounts to search
	var accounts []string
	if account != "" {
		accounts = []string{account}
	} else {
		accounts = cfg.ListAccounts()
	}

	if noCache {
		return listLive(cfg, fromDate, toDate, search, accounts)
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
			id, _ := fm["id"].(string)

			events = append(events, EventInfo{
				ID:       id,
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
			return nil, fmt.Errorf("failed to walk calendar directory: %w", err)
		}
	}

	sortEvents(events)
	return events, nil
}

func listLive(cfg *config.Config, fromDate, toDate time.Time, search string, accounts []string) ([]EventInfo, error) {
	var events []EventInfo

	for _, acc := range accounts {
		token, err := auth.GetAccessToken(cfg, acc)
		if err != nil {
			return nil, err
		}

		client := graph.NewClient(token)
		graphEvents, err := client.GetCalendarView(fromDate, toDate)
		if err != nil {
			return nil, fmt.Errorf("failed to get calendar view for '%s': %w", acc, err)
		}

		for _, event := range graphEvents {
			if search != "" && !strings.Contains(strings.ToLower(eventSearchText(event)), strings.ToLower(search)) {
				continue
			}

			info, err := eventInfoFromGraph(acc, event)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: skipped event %s: %v\n", event.ID, err)
				continue
			}
			events = append(events, info)
		}
	}

	sortEvents(events)
	return events, nil
}

func eventInfoFromGraph(account string, event graph.Event) (EventInfo, error) {
	start, err := parseGraphDateTime(event.Start)
	if err != nil {
		return EventInfo{}, err
	}

	end, err := parseGraphDateTime(event.End)
	if err != nil {
		return EventInfo{}, err
	}

	location := ""
	if event.Location != nil {
		location = event.Location.DisplayName
	}

	return EventInfo{
		ID:       event.ID,
		Start:    start,
		End:      end,
		Subject:  event.Subject,
		Location: location,
		Account:  account,
	}, nil
}

func parseGraphDateTime(value graph.DateTime) (time.Time, error) {
	loc := time.UTC
	if value.TimeZone != "" {
		loaded, err := time.LoadLocation(value.TimeZone)
		if err == nil {
			loc = loaded
		}
	}

	for _, layout := range []string{"2006-01-02T15:04:05.0000000", "2006-01-02T15:04:05", time.RFC3339} {
		if t, err := time.ParseInLocation(layout, value.DateTime, loc); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("failed to parse datetime %s", value.DateTime)
}

func eventSearchText(event graph.Event) string {
	var parts []string
	parts = append(parts, event.Subject)

	if event.Location != nil {
		parts = append(parts, event.Location.DisplayName)
	}
	if event.Organizer != nil {
		parts = append(parts, event.Organizer.EmailAddress.Format())
	}
	for _, attendee := range event.Attendees {
		parts = append(parts, attendee.EmailAddress.Format())
	}
	if event.Body != nil {
		parts = append(parts, event.Body.Content)
	}

	return strings.Join(parts, "\n")
}

func sortEvents(events []EventInfo) {
	sort.Slice(events, func(i, j int) bool {
		return events[i].Start.Before(events[j].Start)
	})
}

// parseFlexibleDateTime parses various datetime formats and converts to the configured timezone
func parseFlexibleDateTime(input, timezoneName string) (string, error) {
	loc, err := time.LoadLocation(timezoneName)
	if err != nil {
		return "", fmt.Errorf("failed to load timezone %s: %w", timezoneName, err)
	}

	// Try parsing various formats
	formats := []string{
		time.RFC3339,          // "2026-03-04T10:00:00+01:00"
		"2006-01-02T15:04:05", // "2026-03-04T10:00:00"
		"2006-01-02 15:04",    // "2026-03-04 10:00"
	}

	var parsed time.Time
	for _, format := range formats {
		t, err := time.Parse(format, input)
		if err == nil {
			parsed = t
			break
		}
	}

	if parsed.IsZero() {
		return "", fmt.Errorf("unable to parse datetime: %s", input)
	}

	// Convert to configured timezone
	inZone := parsed.In(loc)

	// Format without offset for Graph API
	return inZone.Format("2006-01-02T15:04:05.0000000"), nil
}

// Create creates a new calendar event
func Create(cfg *config.Config, account, subject, start, end, location, body string, attendees []string, force bool) (string, error) {
	// Check cross-tenant unless force is enabled
	if !force && len(attendees) > 0 {
		if err := cfg.CheckCrossTenant(account, attendees); err != nil {
			return "", err
		}
	}

	// Get access token
	token, err := auth.GetAccessToken(cfg, account)
	if err != nil {
		return "", err
	}

	// Parse and convert datetimes to configured timezone
	startDateTime, err := parseFlexibleDateTime(start, cfg.Timezone)
	if err != nil {
		return "", fmt.Errorf("invalid start datetime: %w", err)
	}

	endDateTime, err := parseFlexibleDateTime(end, cfg.Timezone)
	if err != nil {
		return "", fmt.Errorf("invalid end datetime: %w", err)
	}

	// Create event
	client := graph.NewClient(token)

	event := &graph.Event{
		Subject: subject,
		Start: graph.DateTime{
			DateTime: startDateTime,
			TimeZone: cfg.Timezone,
		},
		End: graph.DateTime{
			DateTime: endDateTime,
			TimeZone: cfg.Timezone,
		},
	}

	if location != "" {
		event.Location = &graph.Location{DisplayName: location}
	}

	if body != "" {
		event.Body = &graph.Body{
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
		return "", err
	}

	// Write to local file
	filePath, err := sync.WriteEventFile(cfg, account, created, cfg.Timezone)
	if err != nil {
		return "", fmt.Errorf("event created but failed to write local file: %w", err)
	}

	return filePath, nil
}

// Delete deletes a calendar event
func Delete(cfg *config.Config, account, id, filePath string) (string, error) {
	// If file provided, extract account and ID
	if filePath != "" {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("failed to read file: %w", err)
		}

		content := string(data)
		parts := strings.SplitN(content, "---", 3)
		if len(parts) < 3 {
			return "", fmt.Errorf("invalid frontmatter in file")
		}

		var fm map[string]interface{}
		if err := yaml.Unmarshal([]byte(parts[1]), &fm); err != nil {
			return "", fmt.Errorf("failed to parse frontmatter: %w", err)
		}

		var ok bool
		account, ok = fm["account"].(string)
		if !ok {
			return "", fmt.Errorf("account not found in frontmatter")
		}

		id, ok = fm["id"].(string)
		if !ok {
			return "", fmt.Errorf("id not found in frontmatter")
		}
	}

	if account == "" || id == "" {
		return "", fmt.Errorf("account and id are required")
	}

	// Get access token
	token, err := auth.GetAccessToken(cfg, account)
	if err != nil {
		return "", err
	}

	// Delete via API
	client := graph.NewClient(token)
	if err := client.DeleteEvent(id); err != nil {
		return "", err
	}

	// Delete local file
	if filePath != "" {
		if err := os.Remove(filePath); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to delete local file: %v\n", err)
		}
		return filePath, nil
	} else {
		// Find and delete file by ID
		calDir := filepath.Join(cfg.DataDir, account, "calendar")

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
					filePath = path
				}
			}

			return nil
		})

		return filePath, nil
	}
}
