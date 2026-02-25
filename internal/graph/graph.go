package graph

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const (
	baseURL = "https://graph.microsoft.com/v1.0"
)

// Client represents a Microsoft Graph API client
type Client struct {
	Token string
}

// NewClient creates a new Graph API client
func NewClient(token string) *Client {
	return &Client{Token: token}
}

// Event represents a calendar event
type Event struct {
	ID                   string         `json:"id,omitempty"`
	Subject              string         `json:"subject"`
	Start                DateTime       `json:"start"`
	End                  DateTime       `json:"end"`
	IsAllDay             bool           `json:"isAllDay,omitempty"`
	Location             *Location      `json:"location,omitempty"`
	Organizer            *Organizer     `json:"organizer,omitempty"`
	Attendees            []Attendee     `json:"attendees,omitempty"`
	ResponseStatus       *Response      `json:"responseStatus,omitempty"`
	IsOnlineMeeting      bool           `json:"isOnlineMeeting,omitempty"`
	OnlineMeeting        *OnlineMeeting `json:"onlineMeeting,omitempty"`
	Categories           []string       `json:"categories,omitempty"`
	Sensitivity          string         `json:"sensitivity,omitempty"`
	LastModifiedDateTime string         `json:"lastModifiedDateTime,omitempty"`
	Body                 *Body          `json:"body,omitempty"`
}

// DateTime represents a date/time
type DateTime struct {
	DateTime string `json:"dateTime"`
	TimeZone string `json:"timeZone"`
}

// Location represents a location
type Location struct {
	DisplayName string `json:"displayName"`
}

// Organizer represents an organizer
type Organizer struct {
	EmailAddress EmailAddress `json:"emailAddress"`
}

// Attendee represents an attendee
type Attendee struct {
	EmailAddress EmailAddress `json:"emailAddress"`
}

// EmailAddress represents an email address
type EmailAddress struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

// Format returns "Name <email>" or just "email" if no name
func (e EmailAddress) Format() string {
	if e.Name != "" && e.Name != e.Address {
		return fmt.Sprintf("%s <%s>", e.Name, e.Address)
	}
	return e.Address
}

// Response represents a response status
type Response struct {
	Response string `json:"response"`
}

// OnlineMeeting represents online meeting details
type OnlineMeeting struct {
	JoinURL string `json:"joinUrl"`
}

// Body represents a body
type Body struct {
	ContentType string `json:"contentType"`
	Content     string `json:"content"`
}

// Contact represents a contact
type Contact struct {
	ID                   string         `json:"id"`
	DisplayName          string         `json:"displayName"`
	GivenName            string         `json:"givenName"`
	Surname              string         `json:"surname"`
	EmailAddresses       []EmailAddress `json:"emailAddresses"`
	BusinessPhones       []string       `json:"businessPhones"`
	HomePhones           []string       `json:"homePhones"`
	MobilePhone          string         `json:"mobilePhone"`
	CompanyName          string         `json:"companyName"`
	JobTitle             string         `json:"jobTitle"`
	Birthday             string         `json:"birthday"`
	LastModifiedDateTime string         `json:"lastModifiedDateTime"`
	Removed              *RemovedMarker `json:"@removed,omitempty"`
}

// RemovedMarker indicates a removed item in delta query
type RemovedMarker struct {
	Reason string `json:"reason"`
}

// ODataResponse represents a paged OData response
type ODataResponse struct {
	Value    json.RawMessage `json:"value"`
	NextLink string          `json:"@odata.nextLink"`
	DeltaLink string         `json:"@odata.deltaLink"`
}

// ErrorResponse represents an error from the Graph API
type ErrorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// GetCalendarView retrieves calendar events in a date range
func (c *Client) GetCalendarView(startDate, endDate time.Time) ([]Event, error) {
	// Format dates in their current timezone (don't convert to UTC)
	start := startDate.Format("2006-01-02T15:04:05")
	end := endDate.Format("2006-01-02T15:04:05")

	url := fmt.Sprintf("%s/me/calendarview?startDateTime=%s&endDateTime=%s", baseURL, start, end)

	var allEvents []Event

	for url != "" {
		resp, err := c.doRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}

		var odataResp ODataResponse
		if err := json.Unmarshal(resp, &odataResp); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}

		var events []Event
		if err := json.Unmarshal(odataResp.Value, &events); err != nil {
			return nil, fmt.Errorf("failed to parse events: %w", err)
		}

		allEvents = append(allEvents, events...)
		url = odataResp.NextLink
	}

	return allEvents, nil
}

// GetContactsDelta retrieves contacts using delta query
func (c *Client) GetContactsDelta(deltaLink string) ([]Contact, string, error) {
	url := deltaLink
	if url == "" {
		url = fmt.Sprintf("%s/me/contacts/delta", baseURL)
	}

	var allContacts []Contact
	var newDeltaLink string

	for url != "" {
		resp, err := c.doRequest("GET", url, nil)
		if err != nil {
			return nil, "", err
		}

		var odataResp ODataResponse
		if err := json.Unmarshal(resp, &odataResp); err != nil {
			return nil, "", fmt.Errorf("failed to parse response: %w", err)
		}

		var contacts []Contact
		if err := json.Unmarshal(odataResp.Value, &contacts); err != nil {
			return nil, "", fmt.Errorf("failed to parse contacts: %w", err)
		}

		allContacts = append(allContacts, contacts...)

		if odataResp.DeltaLink != "" {
			newDeltaLink = odataResp.DeltaLink
			break
		}
		url = odataResp.NextLink
	}

	return allContacts, newDeltaLink, nil
}

// CreateEvent creates a new calendar event
func (c *Client) CreateEvent(event *Event) (*Event, error) {
	url := fmt.Sprintf("%s/me/events", baseURL)

	data, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event: %w", err)
	}

	resp, err := c.doRequest("POST", url, data)
	if err != nil {
		return nil, err
	}

	var created Event
	if err := json.Unmarshal(resp, &created); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &created, nil
}

// DeleteEvent deletes a calendar event
func (c *Client) DeleteEvent(eventID string) error {
	url := fmt.Sprintf("%s/me/events/%s", baseURL, eventID)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		var errResp ErrorResponse
		if json.Unmarshal(body, &errResp) == nil && errResp.Error.Message != "" {
			return fmt.Errorf("failed to delete event (HTTP %d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return fmt.Errorf("failed to delete event (HTTP %d)", resp.StatusCode)
	}

	return nil
}

// SendMail sends an email
func (c *Client) SendMail(to, subject, body string) error {
	url := fmt.Sprintf("%s/me/sendMail", baseURL)

	payload := map[string]interface{}{
		"message": map[string]interface{}{
			"subject": subject,
			"body": map[string]string{
				"contentType": "text",
				"content":     body,
			},
			"toRecipients": []map[string]interface{}{
				{
					"emailAddress": map[string]string{
						"address": to,
					},
				},
			},
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	_, err = c.doRequest("POST", url, data)
	return err
}

// doRequest performs an HTTP request
func (c *Client) doRequest(method, url string, body []byte) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for errors
	if resp.StatusCode >= 400 {
		var errResp ErrorResponse
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Error.Message != "" {
			return nil, fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return nil, fmt.Errorf("API error (HTTP %d)", resp.StatusCode)
	}

	// For methods that return no content
	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusAccepted {
		return nil, nil
	}

	return respBody, nil
}

// HTMLToMarkdown converts HTML to basic markdown
func HTMLToMarkdown(html string) string {
	md := html

	// Convert <br> to newlines
	md = regexp.MustCompile(`<br[^>]*>`).ReplaceAllString(md, "\n")

	// Convert </p> to double newlines
	md = regexp.MustCompile(`</p>`).ReplaceAllString(md, "\n\n")

	// Remove <p> tags
	md = regexp.MustCompile(`<p[^>]*>`).ReplaceAllString(md, "")

	// Convert links
	linkRe := regexp.MustCompile(`<a[^>]*href=["']([^"']*)["'][^>]*>([^<]*)</a>`)
	md = linkRe.ReplaceAllString(md, "[$2]($1)")

	// Convert bold
	md = regexp.MustCompile(`<strong>([^<]*)</strong>`).ReplaceAllString(md, "**$1**")
	md = regexp.MustCompile(`<b>([^<]*)</b>`).ReplaceAllString(md, "**$1**")

	// Convert italic
	md = regexp.MustCompile(`<em>([^<]*)</em>`).ReplaceAllString(md, "*$1*")
	md = regexp.MustCompile(`<i>([^<]*)</i>`).ReplaceAllString(md, "*$1*")

	// Remove all remaining HTML tags
	md = regexp.MustCompile(`<[^>]*>`).ReplaceAllString(md, "")

	// Decode HTML entities
	md = strings.ReplaceAll(md, "&nbsp;", " ")
	md = strings.ReplaceAll(md, "&amp;", "&")
	md = strings.ReplaceAll(md, "&lt;", "<")
	md = strings.ReplaceAll(md, "&gt;", ">")
	md = strings.ReplaceAll(md, "&quot;", "\"")

	return md
}
