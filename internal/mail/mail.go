package mail

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/lcorneliussen/md365/internal/auth"
	"github.com/lcorneliussen/md365/internal/config"
	"github.com/lcorneliussen/md365/internal/graph"
)

// ListOptions controls mail list
type ListOptions struct {
	Search   string
	FromAddr string
	Since    string
	Until    string
	Unread   bool
	Folder   string
	Limit    int
}

type MessageInfo struct {
	ID               string   `json:"id"`
	Account          string   `json:"account"`
	Subject          string   `json:"subject"`
	From             string   `json:"from,omitempty"`
	To               []string `json:"to,omitempty"`
	CC               []string `json:"cc,omitempty"`
	ReceivedDateTime string   `json:"received,omitempty"`
	SentDateTime     string   `json:"sent,omitempty"`
	IsRead           bool     `json:"is_read"`
	HasAttachments   bool     `json:"has_attachments"`
	BodyPreview      string   `json:"body_preview,omitempty"`
	ConversationID   string   `json:"conversation_id,omitempty"`
	WebLink          string   `json:"web_link,omitempty"`
	BodyMarkdown     string   `json:"body_markdown,omitempty"`
}

type AttachmentInfo struct {
	ID          string `json:"id"`
	Account     string `json:"account"`
	MessageID   string `json:"message_id"`
	Name        string `json:"name"`
	ContentType string `json:"content_type,omitempty"`
	Size        int    `json:"size,omitempty"`
	IsInline    bool   `json:"is_inline,omitempty"`
}

// List lists mailbox messages via Microsoft Graph API
func List(cfg *config.Config, account string, opts ListOptions) ([]MessageInfo, error) {
	if account == "" {
		return nil, fmt.Errorf("--account is required")
	}

	token, err := auth.GetAccessToken(cfg, account)
	if err != nil {
		return nil, err
	}

	since, err := parseDay(opts.Since, cfg.Timezone, false)
	if err != nil {
		return nil, fmt.Errorf("invalid --since: %w", err)
	}
	until, err := parseDay(opts.Until, cfg.Timezone, true)
	if err != nil {
		return nil, fmt.Errorf("invalid --until: %w", err)
	}

	client := graph.NewClient(token)
	messages, err := client.ListMessages(graph.ListMessagesOptions{
		Search:   opts.Search,
		FromAddr: opts.FromAddr,
		Since:    since,
		Until:    until,
		Unread:   opts.Unread,
		Folder:   opts.Folder,
		Limit:    opts.Limit,
	})
	if err != nil {
		return nil, err
	}

	results := make([]MessageInfo, 0, len(messages))
	for _, msg := range messages {
		results = append(results, messageInfoFromGraph(account, msg, false))
	}
	return results, nil
}

// Get prints a single message as Markdown
func Get(cfg *config.Config, account, id string) (*MessageInfo, error) {
	if account == "" || id == "" {
		return nil, fmt.Errorf("--account and --id are required")
	}

	token, err := auth.GetAccessToken(cfg, account)
	if err != nil {
		return nil, err
	}

	client := graph.NewClient(token)
	msg, err := client.GetMessage(id)
	if err != nil {
		return nil, err
	}

	result := messageInfoFromGraph(account, *msg, true)
	return &result, nil
}

// ListAttachments lists attachment metadata for one message.
func ListAttachments(cfg *config.Config, account, id string) ([]AttachmentInfo, error) {
	if account == "" || id == "" {
		return nil, fmt.Errorf("--account and --id are required")
	}

	token, err := auth.GetAccessToken(cfg, account)
	if err != nil {
		return nil, err
	}

	client := graph.NewClient(token)
	attachments, err := client.ListAttachments(id)
	if err != nil {
		return nil, err
	}

	results := make([]AttachmentInfo, 0, len(attachments))
	for _, attachment := range attachments {
		results = append(results, AttachmentInfo{
			ID:          attachment.ID,
			Account:     account,
			MessageID:   id,
			Name:        attachment.Name,
			ContentType: attachment.ContentType,
			Size:        attachment.Size,
			IsInline:    attachment.IsInline,
		})
	}
	return results, nil
}

func messageInfoFromGraph(account string, msg graph.Message, includeBody bool) MessageInfo {
	from := ""
	if msg.From != nil {
		from = msg.From.EmailAddress.Format()
	}

	body := ""
	if msg.Body != nil && strings.TrimSpace(msg.Body.Content) != "" {
		body = msg.Body.Content
		if strings.EqualFold(msg.Body.ContentType, "html") {
			body = graph.HTMLToMarkdown(body)
		}
	}

	info := MessageInfo{
		ID:               msg.ID,
		Account:          account,
		Subject:          msg.Subject,
		From:             from,
		To:               recipientStrings(msg.ToRecipients),
		CC:               recipientStrings(msg.CcRecipients),
		ReceivedDateTime: msg.ReceivedDateTime,
		SentDateTime:     msg.SentDateTime,
		IsRead:           msg.IsRead,
		HasAttachments:   msg.HasAttachments,
		BodyPreview:      msg.BodyPreview,
		ConversationID:   msg.ConversationID,
		WebLink:          msg.WebLink,
	}
	if includeBody {
		info.BodyMarkdown = strings.TrimSpace(body)
	}
	return info
}

// Send sends an email
func Send(cfg *config.Config, account, to, subject, body string, force bool) error {
	// Check cross-tenant unless force is enabled
	if !force {
		if err := cfg.CheckCrossTenant(account, []string{to}); err != nil {
			return err
		}
	}

	// Get access token
	token, err := auth.GetAccessToken(cfg, account)
	if err != nil {
		return err
	}

	// Send email
	client := graph.NewClient(token)
	if err := client.SendMail(to, subject, body); err != nil {
		return err
	}

	return nil
}

// Draft creates an email draft without sending it.
func Draft(cfg *config.Config, account, to, subject, body string, force bool) (*MessageInfo, error) {
	if !force {
		if err := cfg.CheckCrossTenant(account, []string{to}); err != nil {
			return nil, err
		}
	}

	token, err := auth.GetAccessToken(cfg, account)
	if err != nil {
		return nil, err
	}

	created, err := graph.NewClient(token).CreateDraft(to, subject, body)
	if err != nil {
		return nil, err
	}
	result := messageInfoFromGraph(account, *created, true)
	return &result, nil
}

// MarkRead marks messages as read
func MarkRead(cfg *config.Config, account string, ids []string) (int, int, error) {
	token, err := auth.GetAccessToken(cfg, account)
	if err != nil {
		return 0, 0, err
	}
	client := graph.NewClient(token)
	success := 0
	failed := 0
	for _, id := range ids {
		if err := client.UpdateMessage(id, map[string]interface{}{"isRead": true}); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to mark read: %v\n", err)
			failed++
			continue
		}
		success++
	}
	return success, failed, nil
}

// Archive marks messages as read and moves them to the archive folder
func Archive(cfg *config.Config, account string, ids []string) (int, int, error) {
	token, err := auth.GetAccessToken(cfg, account)
	if err != nil {
		return 0, 0, err
	}
	client := graph.NewClient(token)
	success := 0
	failed := 0
	for _, id := range ids {
		if err := client.UpdateMessage(id, map[string]interface{}{"isRead": true}); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to mark read: %v\n", err)
			failed++
			continue
		}
		if err := client.MoveMessage(id, "archive"); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to archive: %v\n", err)
			failed++
			continue
		}
		success++
	}
	return success, failed, nil
}

// Delete deletes messages (moves to Deleted Items)
func Delete(cfg *config.Config, account string, ids []string) (int, int, error) {
	token, err := auth.GetAccessToken(cfg, account)
	if err != nil {
		return 0, 0, err
	}
	client := graph.NewClient(token)
	success := 0
	failed := 0
	for _, id := range ids {
		if err := client.DeleteMessage(id); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to delete: %v\n", err)
			failed++
			continue
		}
		success++
	}
	return success, failed, nil
}

func parseDay(value, timezone string, endOfDay bool) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to load timezone %s: %w", timezone, err)
	}

	t, err := time.ParseInLocation("2006-01-02", value, loc)
	if err != nil {
		return time.Time{}, err
	}
	if endOfDay {
		t = t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
	}
	return t, nil
}

func formatReceived(value string, loc *time.Location) string {
	if value == "" {
		return "????-??-?? ??:??"
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return value
	}
	return t.In(loc).Format("2006-01-02 15:04")
}

func recipientStrings(recipients []graph.Recipient) []string {
	parts := make([]string, 0, len(recipients))
	for _, r := range recipients {
		if s := r.EmailAddress.Format(); s != "" {
			parts = append(parts, s)
		}
	}
	return parts
}

func pad(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}
