package cmd

import (
	"os"

	"github.com/lcorneliussen/md365/internal/mail"
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
	Run: func(cmd *cobra.Command, args []string) {
		if mailAccount == "" {
			cmd.Help()
			os.Exit(1)
			return
		}

		if err := mail.List(cfg, mailAccount, mail.ListOptions{
			Search:   mailSearch,
			FromAddr: mailFromAddr,
			Since:    mailSince,
			Until:    mailUntil,
			Unread:   mailUnread,
			Folder:   mailFolder,
			Limit:    mailLimit,
		}); err != nil {
			fatal(err)
		}
	},
}

// mailGetCmd prints one message as Markdown
var mailGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get email",
	Long:  `Get a single email as Markdown via Microsoft Graph API.`,
	Run: func(cmd *cobra.Command, args []string) {
		if mailAccount == "" || mailID == "" {
			cmd.Help()
			os.Exit(1)
			return
		}

		if err := mail.Get(cfg, mailAccount, mailID); err != nil {
			fatal(err)
		}
	},
}

// mailSendCmd represents the mail send command
var mailSendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send email",
	Long:  `Send an email via Microsoft Graph API.`,
	Run: func(cmd *cobra.Command, args []string) {
		if mailAccount == "" || mailTo == "" || mailSubject == "" {
			cmd.Help()
			os.Exit(1)
			return
		}

		if err := mail.Send(cfg, mailAccount, mailTo, mailSubject, mailBody, mailForce); err != nil {
			fatal(err)
		}
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

	mailSendCmd.Flags().StringVar(&mailAccount, "account", "", "Account (required)")
	mailSendCmd.Flags().StringVar(&mailTo, "to", "", "Recipient email (required)")
	mailSendCmd.Flags().StringVar(&mailSubject, "subject", "", "Email subject (required)")
	mailSendCmd.Flags().StringVar(&mailBody, "body", "", "Email body")
	mailSendCmd.Flags().BoolVar(&mailForce, "force", false, "Bypass cross-tenant checks")

	mailCmd.AddCommand(mailListCmd)
	mailCmd.AddCommand(mailGetCmd)
	mailCmd.AddCommand(mailSendCmd)
}
