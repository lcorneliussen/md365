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
func WriteEventFile(cfg *config.Config, account string, event *graph.Event) (string, error) {
	calDir := filepath.Join(cfg.DataDir, account, "calendar")
	if err := os.MkdirAll(calDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create calendar directory: %w", err)
	}

	// Parse start date for filename
	startDate := strings.Split(event.Start.DateTime, "T")[0]

	// Generate filename
	slug := auth.Slugify(event.Subject, 60)
	if slug == "" {
		slug = "untitled"
	}
	baseName := fmt.Sprintf("%s-%s", startDate, slug)
	filename := auth.GenerateUniqueFilename(calDir, baseName, ".md")
	filePath := filepath.Join(calDir, filename)

	// Build frontmatter
	fm := map[string]interface{}{
		"id":            event.ID,
		"account":       account,
		"subject":       event.Subject,
		"start":         event.Start.DateTime + "Z",
		"end":           event.End.DateTime + "Z",
		"all_day":       event.IsAllDay,
		"response":      event.ResponseStatus.Response,
		"online_meeting": event.IsOnlineMeeting,
		"sensitivity":   event.Sensitivity,
		"last_modified": event.LastModifiedDateTime,
	}

	if event.Location.DisplayName != "" {
		fm["location"] = event.Location.DisplayName
	}

	if event.Organizer.EmailAddress.Address != "" {
		fm["organizer"] = event.Organizer.EmailAddress.Address
	}

	if len(event.Attendees) > 0 {
		attendees := make([]string, len(event.Attendees))
		for i, a := range event.Attendees {
			attendees[i] = a.EmailAddress.Address
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
	body := graph.HTMLToMarkdown(event.Body.Content)

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

	// Generate filename
	slug := auth.Slugify(contact.DisplayName, 60)
	if slug == "" {
		slug = "unnamed"
	}
	filename := auth.GenerateUniqueFilename(contactDir, slug, ".md")
	filePath := filepath.Join(contactDir, filename)

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

	// Track seen IDs
	seenIDs := make(map[string]bool)

	// Write events
	for _, event := range events {
		seenIDs[event.ID] = true
		if _, err := WriteEventFile(cfg, account, &event); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to write event %s: %v\n", event.ID, err)
		}
	}

	// Delete events not in API response
	deleted := 0
	if err := filepath.Walk(calDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}

		id, err := extractIDFromFile(path)
		if err != nil {
			return nil
		}

		if !seenIDs[id] {
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
