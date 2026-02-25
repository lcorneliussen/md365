package sync

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lcorneliussen/md365/internal/auth"
	"github.com/lcorneliussen/md365/internal/config"
	"github.com/lcorneliussen/md365/internal/graph"
	"gopkg.in/yaml.v3"
)

// SyncState represents the sync state for an account
type SyncState struct {
	LastSync          string `json:"last_sync"`
	ContactsDeltaLink string `json:"contacts_delta_link,omitempty"`
}

// WriteEventFile writes a calendar event to a markdown file
func WriteEventFile(cfg *config.Config, account string, event *graph.Event, timezone string) (string, error) {
	calDir := filepath.Join(cfg.DataDir, account, "calendar")
	if err := os.MkdirAll(calDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create calendar directory: %w", err)
	}

	// Convert start/end times from Graph API format to RFC3339 in configured timezone
	startRFC3339, err := convertGraphTimeToRFC3339(event.Start.DateTime, event.Start.TimeZone, timezone)
	if err != nil {
		return "", fmt.Errorf("failed to convert start time: %w", err)
	}

	endRFC3339, err := convertGraphTimeToRFC3339(event.End.DateTime, event.End.TimeZone, timezone)
	if err != nil {
		return "", fmt.Errorf("failed to convert end time: %w", err)
	}

	// Generate the desired filename based on current event data
	startDate := strings.Split(event.Start.DateTime, "T")[0]
	slug := auth.Slugify(event.Subject, 60)
	if slug == "" {
		slug = "untitled"
	}
	desiredBase := fmt.Sprintf("%s-%s", startDate, slug)

	// Check if a file with this event ID already exists
	existingPath := findFileByID(calDir, event.ID)

	var filePath string
	if existingPath != "" {
		// Check if rename is needed (subject or date changed)
		existingBase := strings.TrimSuffix(filepath.Base(existingPath), ".md")
		if existingBase != desiredBase {
			newFilename := auth.GenerateUniqueFilename(calDir, desiredBase, ".md")
			filePath = filepath.Join(calDir, newFilename)
			os.Rename(existingPath, filePath)
		} else {
			filePath = existingPath
		}
	} else {
		// New event
		filename := auth.GenerateUniqueFilename(calDir, desiredBase, ".md")
		filePath = filepath.Join(calDir, filename)
	}

	// Build frontmatter
	fm := map[string]interface{}{
		"id":            event.ID,
		"account":       account,
		"subject":       event.Subject,
		"start":         startRFC3339,
		"end":           endRFC3339,
		"all_day":       event.IsAllDay,
		"online_meeting": event.IsOnlineMeeting,
		"sensitivity":   event.Sensitivity,
		"last_modified": event.LastModifiedDateTime,
	}

	if event.ResponseStatus != nil {
		fm["response"] = event.ResponseStatus.Response
	}

	if event.Location != nil && event.Location.DisplayName != "" {
		fm["location"] = event.Location.DisplayName
	}

	if event.Organizer != nil && event.Organizer.EmailAddress.Address != "" {
		fm["organizer"] = event.Organizer.EmailAddress.Format()
	}

	if len(event.Attendees) > 0 {
		attendees := make([]string, len(event.Attendees))
		for i, a := range event.Attendees {
			attendees[i] = a.EmailAddress.Format()
		}
		fm["attendees"] = attendees
	}

	if event.IsOnlineMeeting && event.OnlineMeeting != nil && event.OnlineMeeting.JoinURL != "" {
		fm["meeting_url"] = event.OnlineMeeting.JoinURL
	}

	if len(event.Categories) > 0 {
		fm["categories"] = event.Categories
	}

	// Marshal frontmatter
	fmData, err := yaml.Marshal(fm)
	if err != nil {
		return "", fmt.Errorf("failed to marshal frontmatter: %w", err)
	}

	// Convert body HTML to markdown
	var bodyContent string
	if event.Body != nil {
		bodyContent = event.Body.Content
	}
	body := graph.HTMLToMarkdown(bodyContent)

	// Write file
	content := fmt.Sprintf("---\n%s---\n\n# %s\n\n%s\n", string(fmData), event.Subject, body)
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return filePath, nil
}

// WriteContactFile writes a contact to a markdown file
func WriteContactFile(cfg *config.Config, account string, contact *graph.Contact) (string, error) {
	contactDir := filepath.Join(cfg.DataDir, account, "contacts")
	if err := os.MkdirAll(contactDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create contacts directory: %w", err)
	}

	// Check if a file with this contact ID already exists — update in place
	filePath := findFileByID(contactDir, contact.ID)

	if filePath == "" {
		// New contact — generate filename
		slug := auth.Slugify(contact.DisplayName, 60)
		if slug == "" {
			slug = "unnamed"
		}
		filename := auth.GenerateUniqueFilename(contactDir, slug, ".md")
		filePath = filepath.Join(contactDir, filename)
	}

	// Build frontmatter
	fm := map[string]interface{}{
		"id":            contact.ID,
		"account":       account,
		"display_name":  contact.DisplayName,
		"last_modified": contact.LastModifiedDateTime,
	}

	if contact.GivenName != "" {
		fm["given_name"] = contact.GivenName
	}

	if contact.Surname != "" {
		fm["surname"] = contact.Surname
	}

	if len(contact.EmailAddresses) > 0 {
		emails := make([]string, len(contact.EmailAddresses))
		for i, e := range contact.EmailAddresses {
			emails[i] = e.Address
		}
		fm["emails"] = emails
	}

	// Collect all phone numbers
	var phones []string
	phones = append(phones, contact.BusinessPhones...)
	phones = append(phones, contact.HomePhones...)
	if contact.MobilePhone != "" {
		phones = append(phones, contact.MobilePhone)
	}
	if len(phones) > 0 {
		fm["phones"] = phones
	}

	if contact.CompanyName != "" {
		fm["company"] = contact.CompanyName
	}

	if contact.JobTitle != "" {
		fm["job_title"] = contact.JobTitle
	}

	if contact.Birthday != "" {
		fm["birthday"] = contact.Birthday
	}

	// Marshal frontmatter
	fmData, err := yaml.Marshal(fm)
	if err != nil {
		return "", fmt.Errorf("failed to marshal frontmatter: %w", err)
	}

	// Build body
	var bodyLines []string
	bodyLines = append(bodyLines, fmt.Sprintf("# %s", contact.DisplayName))
	bodyLines = append(bodyLines, "")

	if len(contact.EmailAddresses) > 0 {
		emails := make([]string, len(contact.EmailAddresses))
		for i, e := range contact.EmailAddresses {
			emails[i] = e.Address
		}
		bodyLines = append(bodyLines, fmt.Sprintf("📧 %s", strings.Join(emails, ", ")))
	}

	for _, phone := range phones {
		bodyLines = append(bodyLines, fmt.Sprintf("📱 %s", phone))
	}

	if contact.CompanyName != "" || contact.JobTitle != "" {
		line := "🏢 "
		if contact.CompanyName != "" {
			line += contact.CompanyName
		}
		if contact.JobTitle != "" {
			if contact.CompanyName != "" {
				line += " — "
			}
			line += contact.JobTitle
		}
		bodyLines = append(bodyLines, line)
	}

	body := strings.Join(bodyLines, "\n")

	// Write file
	content := fmt.Sprintf("---\n%s---\n\n%s\n", string(fmData), body)
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return filePath, nil
}

// SyncCalendar syncs calendar events for an account
func SyncCalendar(cfg *config.Config, account string, token string) error {
	client := graph.NewClient(token)
	calDir := filepath.Join(cfg.DataDir, account, "calendar")

	fmt.Printf("Syncing calendar for account '%s'...\n", account)

	// Calculate date range: -30 days to +90 days
	startDate := time.Now().AddDate(0, 0, -30)
	endDate := time.Now().AddDate(0, 0, 90)

	events, err := client.GetCalendarView(startDate, endDate)
	if err != nil {
		return fmt.Errorf("failed to get calendar view: %w", err)
	}

	// Track which file path was written for each event ID
	writtenPaths := make(map[string]string)

	// Write events
	for _, event := range events {
		path, err := WriteEventFile(cfg, account, &event, cfg.Timezone)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to write event %s: %v\n", event.ID, err)
			continue
		}
		writtenPaths[event.ID] = path
	}

	// Delete files that are not the canonical path for any event
	// This removes both stale events and duplicates
	deleted := 0
	if err := filepath.Walk(calDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}

		id, err := extractIDFromFile(path)
		if err != nil {
			return nil
		}

		canonicalPath, seen := writtenPaths[id]
		if !seen || path != canonicalPath {
			if err := os.Remove(path); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to delete %s: %v\n", path, err)
			} else {
				deleted++
			}
		}

		return nil
	}); err != nil {
		return fmt.Errorf("failed to walk calendar directory: %w", err)
	}

	// Update sync state
	if err := updateSyncState(cfg.DataDir, account, "", ""); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to update sync state: %v\n", err)
	}

	fmt.Printf("Synced %d events for '%s' (deleted %d)\n", len(events), account, deleted)
	return nil
}

// SyncContacts syncs contacts for an account
func SyncContacts(cfg *config.Config, account string, token string) error {
	client := graph.NewClient(token)
	contactDir := filepath.Join(cfg.DataDir, account, "contacts")

	fmt.Printf("Syncing contacts for account '%s'...\n", account)

	// Load sync state
	state, err := loadSyncState(cfg.DataDir, account)
	if err != nil {
		state = &SyncState{}
	}

	// Get contacts using delta query
	contacts, newDeltaLink, err := client.GetContactsDelta(state.ContactsDeltaLink)
	if err != nil {
		return fmt.Errorf("failed to get contacts: %w", err)
	}

	newCount := 0
	deletedCount := 0

	// Process contacts
	for _, contact := range contacts {
		if contact.Removed != nil {
			// Delete contact
			if err := deleteContactByID(contactDir, contact.ID); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to delete contact %s: %v\n", contact.ID, err)
			} else {
				deletedCount++
			}
		} else {
			// New or updated contact
			if _, err := WriteContactFile(cfg, account, &contact); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to write contact %s: %v\n", contact.ID, err)
			} else {
				newCount++
			}
		}
	}

	// Update sync state
	if err := updateSyncState(cfg.DataDir, account, newDeltaLink, ""); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to update sync state: %v\n", err)
	}

	fmt.Printf("Synced contacts for '%s' (new/updated: %d, deleted: %d)\n", account, newCount, deletedCount)
	return nil
}

// findFileByID finds an existing markdown file with the given ID in its frontmatter
func findFileByID(dir, id string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		fileID, err := extractIDFromFile(path)
		if err == nil && fileID == id {
			return path
		}
	}
	return ""
}

// extractIDFromFile extracts the ID from a markdown file's frontmatter
func extractIDFromFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	// Parse frontmatter
	content := string(data)
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return "", fmt.Errorf("invalid frontmatter")
	}

	var fm map[string]interface{}
	if err := yaml.Unmarshal([]byte(parts[1]), &fm); err != nil {
		return "", err
	}

	id, ok := fm["id"].(string)
	if !ok {
		return "", fmt.Errorf("id not found in frontmatter")
	}

	return id, nil
}

// deleteContactByID deletes a contact file by ID
func deleteContactByID(contactDir, id string) error {
	return filepath.Walk(contactDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}

		fileID, err := extractIDFromFile(path)
		if err != nil {
			return nil
		}

		if fileID == id {
			return os.Remove(path)
		}

		return nil
	})
}

// loadSyncState loads the sync state for an account
func loadSyncState(dataDir, account string) (*SyncState, error) {
	syncDir := filepath.Join(dataDir, ".sync")
	syncFile := filepath.Join(syncDir, account+".json")

	data, err := os.ReadFile(syncFile)
	if err != nil {
		return nil, err
	}

	var state SyncState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	return &state, nil
}

// updateSyncState updates the sync state for an account
func updateSyncState(dataDir, account, deltaLink, lastSync string) error {
	syncDir := filepath.Join(dataDir, ".sync")
	if err := os.MkdirAll(syncDir, 0755); err != nil {
		return err
	}

	syncFile := filepath.Join(syncDir, account+".json")

	// Load existing state
	state, err := loadSyncState(dataDir, account)
	if err != nil {
		state = &SyncState{}
	}

	// Update fields
	if deltaLink != "" {
		state.ContactsDeltaLink = deltaLink
	}
	if lastSync == "" {
		lastSync = time.Now().UTC().Format(time.RFC3339)
	}
	state.LastSync = lastSync

	// Save state
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(syncFile, data, 0644)
}

// convertGraphTimeToRFC3339 converts a Graph API DateTime+TimeZone pair to RFC3339 in the target timezone
// Graph API format: "2026-02-28T19:15:00.0000000" with separate "Europe/Berlin" timezone field
func convertGraphTimeToRFC3339(dateTimeStr, sourceTimeZone, targetTimeZone string) (string, error) {
	// Load source timezone
	sourceLoc, err := time.LoadLocation(sourceTimeZone)
	if err != nil {
		return "", fmt.Errorf("invalid source timezone %s: %w", sourceTimeZone, err)
	}

	// Load target timezone
	targetLoc, err := time.LoadLocation(targetTimeZone)
	if err != nil {
		return "", fmt.Errorf("invalid target timezone %s: %w", targetTimeZone, err)
	}

	// Parse the Graph API DateTime (without timezone info)
	// Graph returns format like "2026-02-28T19:15:00.0000000"
	t, err := time.ParseInLocation("2006-01-02T15:04:05.0000000", dateTimeStr, sourceLoc)
	if err != nil {
		// Try without fractional seconds
		t, err = time.ParseInLocation("2006-01-02T15:04:05", dateTimeStr, sourceLoc)
		if err != nil {
			return "", fmt.Errorf("failed to parse datetime %s: %w", dateTimeStr, err)
		}
	}

	// Convert to target timezone
	targetTime := t.In(targetLoc)

	// Format as RFC3339
	return targetTime.Format(time.RFC3339), nil
}
