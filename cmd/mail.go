package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/lcorneliussen/md365/internal/mail"
	"github.com/lcorneliussen/md365/internal/output"
	"github.com/spf13/cobra"
)

var (
	mailAccount  string
	mailTo       string
	mailSubject  string
	mailBody     string
	mailForce    bool
	mailSearch   string
	mailFromAddr string
	mailSince    string
	mailUntil    string
	mailFolder   string
	mailLimit    int
	mailUnread   bool
	mailID       string
)

// mailCmd represents the mail command
var mailCmd = &cobra.Command{
	Use:   "mail",
	Short: "Mail commands",
	Long:  `Read and send emails via Microsoft Graph API.`,
}

// mailListCmd lists mailbox messages
var mailListCmd = &cobra.Command{
	Use:   "list",
	Short: "List emails",
	Long:  `List mailbox messages via Microsoft Graph API.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if mailAccount == "" {
			return usageError("--account is required")
		}

		messages, err := mail.List(cfg, mailAccount, mail.ListOptions{
			Search:   mailSearch,
			FromAddr: mailFromAddr,
			Since:    mailSince,
			Until:    mailUntil,
			Unread:   mailUnread,
			Folder:   mailFolder,
			Limit:    mailLimit,
		})
		if err != nil {
			return err
		}
		if writer.IsHuman() {
			printMessages(cmd, messages)
			return nil
		}
		return writeOK(messages,
			output.WithSummary(fmt.Sprintf("%d messages", len(messages))),
			output.WithMeta("source", "graph"),
		)
	},
}

// mailGetCmd prints one message as Markdown
var mailGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get email",
	Long:  `Get a single email as Markdown via Microsoft Graph API.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if mailAccount == "" || mailID == "" {
			return usageError("--account and --id are required")
		}

		msg, err := mail.Get(cfg, mailAccount, mailID)
		if err != nil {
			return err
		}
		if writer.IsHuman() {
			printMessageMarkdown(cmd, msg)
			return nil
		}
		return writeOK(msg,
			output.WithSummary("message read"),
			output.WithMeta("source", "graph"),
			output.WithBreadcrumbs(mailBreadcrumbs(*msg)...),
		)
	},
}

// mailAttachmentsCmd lists attachment metadata for a message.
var mailAttachmentsCmd = &cobra.Command{
	Use:   "attachments",
	Short: "List email attachments",
	Long:  `List attachment metadata for a single email via Microsoft Graph API.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if mailAccount == "" || mailID == "" {
			return usageError("--account and --id are required")
		}

		attachments, err := mail.ListAttachments(cfg, mailAccount, mailID)
		if err != nil {
			return err
		}
		if writer.IsHuman() {
			printAttachments(cmd, attachments)
			return nil
		}
		return writeOK(attachments,
			output.WithSummary(fmt.Sprintf("%d attachments", len(attachments))),
			output.WithMeta("source", "graph"),
			output.WithBreadcrumbs(output.Breadcrumb{
				Action:      "read_message",
				Command:     fmt.Sprintf("md365 mail get --account %s --id %s --json", mailAccount, mailID),
				Description: "Read the message this attachment list belongs to",
			}),
		)
	},
}

// mailSendCmd represents the mail send command
var mailSendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send email",
	Long:  `Send an email via Microsoft Graph API.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if mailAccount == "" || mailTo == "" || mailSubject == "" {
			return usageError("--account, --to, and --subject are required")
		}

		if err := mail.Send(cfg, mailAccount, mailTo, mailSubject, mailBody, mailForce); err != nil {
			return err
		}
		return writeOK(map[string]string{
			"account": mailAccount,
			"to":      mailTo,
			"subject": mailSubject,
		}, output.WithSummary(fmt.Sprintf("Email sent to %s", mailTo)))
	},
}

func init() {
	mailListCmd.Flags().StringVar(&mailAccount, "account", "", "Account (required)")
	mailListCmd.Flags().StringVar(&mailSearch, "search", "", "KQL/text search (e.g. bauer or from:bauer)")
	mailListCmd.Flags().StringVar(&mailFromAddr, "from-addr", "", "Sender email address")
	mailListCmd.Flags().StringVar(&mailSince, "since", "", "Received on or after (YYYY-MM-DD)")
	mailListCmd.Flags().StringVar(&mailUntil, "until", "", "Received on or before (YYYY-MM-DD)")
	mailListCmd.Flags().StringVar(&mailFolder, "folder", "", "Well-known folder (inbox, sentitems, drafts)")
	mailListCmd.Flags().IntVar(&mailLimit, "limit", 25, "Maximum messages")
	mailListCmd.Flags().BoolVar(&mailUnread, "unread", false, "Only unread messages")

	mailGetCmd.Flags().StringVar(&mailAccount, "account", "", "Account (required)")
	mailGetCmd.Flags().StringVar(&mailID, "id", "", "Message ID (required)")

	mailAttachmentsCmd.Flags().StringVar(&mailAccount, "account", "", "Account (required)")
	mailAttachmentsCmd.Flags().StringVar(&mailID, "id", "", "Message ID (required)")

	mailSendCmd.Flags().StringVar(&mailAccount, "account", "", "Account (required)")
	mailSendCmd.Flags().StringVar(&mailTo, "to", "", "Recipient email (required)")
	mailSendCmd.Flags().StringVar(&mailSubject, "subject", "", "Email subject (required)")
	mailSendCmd.Flags().StringVar(&mailBody, "body", "", "Email body")
	mailSendCmd.Flags().BoolVar(&mailForce, "force", false, "Bypass cross-tenant checks")

	mailCmd.AddCommand(mailListCmd)
	mailCmd.AddCommand(mailGetCmd)
	mailCmd.AddCommand(mailAttachmentsCmd)
	mailCmd.AddCommand(mailSendCmd)
}

func printMessages(cmd *cobra.Command, messages []mail.MessageInfo) {
	loc := configuredLocation()
	for _, msg := range messages {
		received := formatReceivedHuman(msg.ReceivedDateTime, loc)
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

		fmt.Fprintf(cmd.OutOrStdout(), "%s  %s  %s%s\n", received, padHuman(msg.From, 36), msg.Subject, flagStr)
		fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", msg.ID)
	}
	if len(messages) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No messages found")
	}
}

func printMessageMarkdown(cmd *cobra.Command, msg *mail.MessageInfo) {
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "---")
	fmt.Fprintf(out, "id: %s\n", msg.ID)
	fmt.Fprintf(out, "account: %s\n", msg.Account)
	fmt.Fprintf(out, "subject: %s\n", msg.Subject)
	fmt.Fprintf(out, "from: %s\n", msg.From)
	if len(msg.To) > 0 {
		fmt.Fprintf(out, "to: %s\n", strings.Join(msg.To, ", "))
	}
	if len(msg.CC) > 0 {
		fmt.Fprintf(out, "cc: %s\n", strings.Join(msg.CC, ", "))
	}
	if msg.ReceivedDateTime != "" {
		fmt.Fprintf(out, "received: %s\n", msg.ReceivedDateTime)
	}
	fmt.Fprintf(out, "is_read: %v\n", msg.IsRead)
	fmt.Fprintf(out, "has_attachments: %v\n", msg.HasAttachments)
	if msg.ConversationID != "" {
		fmt.Fprintf(out, "conversation_id: %s\n", msg.ConversationID)
	}
	if msg.WebLink != "" {
		fmt.Fprintf(out, "web_link: %s\n", msg.WebLink)
	}
	fmt.Fprintln(out, "---")
	fmt.Fprintln(out)
	if msg.Subject != "" {
		fmt.Fprintf(out, "# %s\n\n", msg.Subject)
	}
	if msg.BodyMarkdown != "" {
		fmt.Fprintln(out, msg.BodyMarkdown)
	}
}

func printAttachments(cmd *cobra.Command, attachments []mail.AttachmentInfo) {
	for _, attachment := range attachments {
		inline := ""
		if attachment.IsInline {
			inline = " inline"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s  %s  %d bytes%s\n", attachment.ID, attachment.Name, attachment.Size, inline)
	}
	if len(attachments) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No attachments found")
	}
}

func mailBreadcrumbs(msg mail.MessageInfo) []output.Breadcrumb {
	breadcrumbs := []output.Breadcrumb{}
	if msg.HasAttachments {
		breadcrumbs = append(breadcrumbs, output.Breadcrumb{
			Action:      "list_attachments",
			Command:     fmt.Sprintf("md365 mail attachments --account %s --id %s --json", msg.Account, msg.ID),
			Description: "List metadata for this message's attachments",
		})
	}
	if msg.WebLink != "" {
		breadcrumbs = append(breadcrumbs, output.Breadcrumb{
			Action:      "open_in_browser",
			Command:     msg.WebLink,
			Description: "Open the message in Microsoft 365",
		})
	}
	return breadcrumbs
}

func configuredLocation() *time.Location {
	if cfg != nil {
		if loc, err := time.LoadLocation(cfg.Timezone); err == nil {
			return loc
		}
	}
	return time.UTC
}

func formatReceivedHuman(value string, loc *time.Location) string {
	if value == "" {
		return "????-??-?? ??:??"
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return value
	}
	return t.In(loc).Format("2006-01-02 15:04")
}

func padHuman(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}
