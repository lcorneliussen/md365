package graph

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/lcorneliussen/md365/internal/apierr"
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

// Recipient represents a mail recipient
type Recipient struct {
	EmailAddress EmailAddress `json:"emailAddress"`
}

// Message represents a mail message
type Message struct {
	ID               string      `json:"id,omitempty"`
	Subject          string      `json:"subject"`
	From             *Recipient  `json:"from,omitempty"`
	ToRecipients     []Recipient `json:"toRecipients,omitempty"`
	CcRecipients     []Recipient `json:"ccRecipients,omitempty"`
	ReceivedDateTime string      `json:"receivedDateTime,omitempty"`
	SentDateTime     string      `json:"sentDateTime,omitempty"`
	IsRead           bool        `json:"isRead"`
	HasAttachments   bool        `json:"hasAttachments,omitempty"`
	BodyPreview      string      `json:"bodyPreview,omitempty"`
	ConversationID   string      `json:"conversationId,omitempty"`
	WebLink          string      `json:"webLink,omitempty"`
	Body             *Body       `json:"body,omitempty"`
}

// Attachment represents message attachment metadata.
type Attachment struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name"`
	ContentType string `json:"contentType,omitempty"`
	Size        int    `json:"size,omitempty"`
	IsInline    bool   `json:"isInline,omitempty"`
}

// ListMessagesOptions controls a mailbox query
type ListMessagesOptions struct {
	Search   string
	FromAddr string
	Since    time.Time
	Until    time.Time
	Unread   bool
	Folder   string
	Limit    int
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
	Value     json.RawMessage `json:"value"`
	NextLink  string          `json:"@odata.nextLink"`
	DeltaLink string          `json:"@odata.deltaLink"`
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
			return apierr.Graph(resp.StatusCode, fmt.Sprintf("failed to delete event: %s", errResp.Error.Message))
		}
		return apierr.Graph(resp.StatusCode, "failed to delete event")
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

const messageListSelect = "id,subject,from,toRecipients,receivedDateTime,isRead,hasAttachments,bodyPreview"
const messageGetSelect = "id,subject,from,toRecipients,ccRecipients,receivedDateTime,sentDateTime,isRead,hasAttachments,conversationId,webLink,body"
const attachmentListSelect = "id,name,contentType,size,isInline"

// ListMessages retrieves mailbox messages matching the given options
func (c *Client) ListMessages(opts ListMessagesOptions) ([]Message, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 25
	}

	endpoint := messagesEndpoint(opts.Folder)
	query := url.Values{}
	query.Set("$select", messageListSelect)
	query.Set("$top", strconv.Itoa(min(limit, 50)))

	headers := map[string]string{}
	if search := buildMailSearch(opts); search != "" {
		query.Set("$search", `"`+search+`"`)
		headers["ConsistencyLevel"] = "eventual"
	} else {
		query.Set("$orderby", "receivedDateTime desc")
		if filter := buildMailFilter(opts); filter != "" {
			query.Set("$filter", filter)
		}
	}

	reqURL := endpoint + "?" + query.Encode()
	var all []Message

	for reqURL != "" && len(all) < limit {
		resp, err := c.doRequestHeaders("GET", reqURL, nil, headers)
		if err != nil {
			return nil, err
		}

		var odataResp ODataResponse
		if err := json.Unmarshal(resp, &odataResp); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}

		var messages []Message
		if err := json.Unmarshal(odataResp.Value, &messages); err != nil {
			return nil, fmt.Errorf("failed to parse messages: %w", err)
		}

		all = append(all, messages...)
		reqURL = odataResp.NextLink
	}

	if len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

// GetMessage retrieves a single message including body
func (c *Client) GetMessage(id string) (*Message, error) {
	reqURL := fmt.Sprintf("%s/me/messages/%s?$select=%s", baseURL, url.PathEscape(id), messageGetSelect)
	resp, err := c.doRequestHeaders("GET", reqURL, nil, map[string]string{
		"Prefer": `outlook.body-content-type="text"`,
	})
	if err != nil {
		return nil, err
	}

	var msg Message
	if err := json.Unmarshal(resp, &msg); err != nil {
		return nil, fmt.Errorf("failed to parse message: %w", err)
	}
	return &msg, nil
}

// ListAttachments retrieves metadata for one message's attachments.
func (c *Client) ListAttachments(messageID string) ([]Attachment, error) {
	reqURL := fmt.Sprintf("%s/me/messages/%s/attachments?$select=%s", baseURL, url.PathEscape(messageID), attachmentListSelect)
	var all []Attachment

	for reqURL != "" {
		resp, err := c.doRequestHeaders("GET", reqURL, nil, nil)
		if err != nil {
			return nil, err
		}

		var odataResp ODataResponse
		if err := json.Unmarshal(resp, &odataResp); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}

		var attachments []Attachment
		if err := json.Unmarshal(odataResp.Value, &attachments); err != nil {
			return nil, fmt.Errorf("failed to parse attachments: %w", err)
		}

		all = append(all, attachments...)
		reqURL = odataResp.NextLink
	}

	return all, nil
}

func messagesEndpoint(folder string) string {
	if folder == "" {
		return baseURL + "/me/messages"
	}
	return baseURL + "/me/mailFolders/" + url.PathEscape(folder) + "/messages"
}

func buildMailSearch(opts ListMessagesOptions) string {
	if opts.Search == "" && opts.FromAddr == "" {
		return ""
	}

	var parts []string
	if opts.Search != "" {
		parts = append(parts, "("+opts.Search+")")
	}
	if opts.FromAddr != "" {
		parts = append(parts, "from:"+opts.FromAddr)
	}
	if !opts.Since.IsZero() {
		parts = append(parts, "received>="+opts.Since.Format("2006-01-02"))
	}
	if !opts.Until.IsZero() {
		parts = append(parts, "received<="+opts.Until.Format("2006-01-02"))
	}
	if opts.Unread {
		parts = append(parts, "isread:false")
	}
	return strings.Join(parts, " AND ")
}

func buildMailFilter(opts ListMessagesOptions) string {
	var parts []string
	if !opts.Since.IsZero() {
		parts = append(parts, "receivedDateTime ge "+opts.Since.UTC().Format("2006-01-02T15:04:05Z"))
	}
	if !opts.Until.IsZero() {
		parts = append(parts, "receivedDateTime le "+opts.Until.UTC().Format("2006-01-02T15:04:05Z"))
	}
	if opts.Unread {
		parts = append(parts, "isRead eq false")
	}
	return strings.Join(parts, " and ")
}

// doRequest performs an HTTP request
func (c *Client) doRequest(method, url string, body []byte) ([]byte, error) {
	return c.doRequestHeaders(method, url, body, nil)
}

// doRequestHeaders performs an HTTP request with extra headers
func (c *Client) doRequestHeaders(method, reqURL string, body []byte, headers map[string]string) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, reqURL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
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
			return nil, apierr.Graph(resp.StatusCode, errResp.Error.Message)
		}
		return nil, apierr.Graph(resp.StatusCode, fmt.Sprintf("Microsoft Graph error (HTTP %d)", resp.StatusCode))
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
