package mail

import (
	"fmt"
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

// List lists mailbox messages via Microsoft Graph API
func List(cfg *config.Config, account string, opts ListOptions) error {
	if account == "" {
		return fmt.Errorf("--account is required")
	}

	token, err := auth.GetAccessToken(cfg, account)
	if err != nil {
		return err
	}

	since, err := parseDay(opts.Since, cfg.Timezone, false)
	if err != nil {
		return fmt.Errorf("invalid --since: %w", err)
	}
	until, err := parseDay(opts.Until, cfg.Timezone, true)
	if err != nil {
		return fmt.Errorf("invalid --until: %w", err)
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
		return err
	}

	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		loc = time.UTC
	}

	for _, msg := range messages {
		received := formatReceived(msg.ReceivedDateTime, loc)
		from := ""
		if msg.From != nil {
			from = msg.From.EmailAddress.Format()
		}

		flags := []string{}
		if !msg.IsRead {
			flags = append(flags, "unread")
		}
		if msg.HasAttachments {
			flags = append(flags, "attach")
		}
		flagStr := ""
		if len(flags) > 0 {
			flagStr = "  [" + strings.Join(flags, " ") + "]"
		}

		fmt.Printf("%s  %s  %s%s\n", received, pad(from, 36), msg.Subject, flagStr)
		fmt.Printf("  %s\n", msg.ID)
	}

	if len(messages) == 0 {
		fmt.Println("No messages found")
	}

	return nil
}

// Get prints a single message as Markdown
func Get(cfg *config.Config, account, id string) error {
	if account == "" || id == "" {
		return fmt.Errorf("--account and --id are required")
	}

	token, err := auth.GetAccessToken(cfg, account)
	if err != nil {
		return err
	}

	client := graph.NewClient(token)
	msg, err := client.GetMessage(id)
	if err != nil {
		return err
	}

	from := ""
	if msg.From != nil {
		from = msg.From.EmailAddress.Format()
	}

	fmt.Println("---")
	fmt.Printf("id: %s\n", msg.ID)
	fmt.Printf("account: %s\n", account)
	fmt.Printf("subject: %s\n", msg.Subject)
	fmt.Printf("from: %s\n", from)
	if to := formatRecipients(msg.ToRecipients); to != "" {
		fmt.Printf("to: %s\n", to)
	}
	if cc := formatRecipients(msg.CcRecipients); cc != "" {
		fmt.Printf("cc: %s\n", cc)
	}
	if msg.ReceivedDateTime != "" {
		fmt.Printf("received: %s\n", msg.ReceivedDateTime)
	}
	fmt.Printf("is_read: %v\n", msg.IsRead)
	fmt.Printf("has_attachments: %v\n", msg.HasAttachments)
	if msg.ConversationID != "" {
		fmt.Printf("conversation_id: %s\n", msg.ConversationID)
	}
	if msg.WebLink != "" {
		fmt.Printf("web_link: %s\n", msg.WebLink)
	}
	fmt.Println("---")
	fmt.Println()
	if msg.Subject != "" {
		fmt.Printf("# %s\n\n", msg.Subject)
	}

	if msg.Body != nil && strings.TrimSpace(msg.Body.Content) != "" {
		body := msg.Body.Content
		if strings.EqualFold(msg.Body.ContentType, "html") {
			body = graph.HTMLToMarkdown(body)
		}
		fmt.Println(strings.TrimSpace(body))
	}

	return nil
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

	fmt.Printf("Email sent to %s\n", to)
	return nil
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

func formatRecipients(recipients []graph.Recipient) string {
	parts := make([]string, 0, len(recipients))
	for _, r := range recipients {
		if s := r.EmailAddress.Format(); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, ", ")
}

func pad(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}
