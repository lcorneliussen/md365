package cmd

import (
	"github.com/larsc/md365/internal/mail"
	"github.com/spf13/cobra"
)

var (
	mailAccount string
	mailTo      string
	mailSubject string
	mailBody    string
	mailForce   bool
)

// mailCmd represents the mail command
var mailCmd = &cobra.Command{
	Use:   "mail",
	Short: "Mail commands",
	Long:  `Send emails via Microsoft Graph API.`,
}

// mailSendCmd represents the mail send command
var mailSendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send email",
	Long:  `Send an email via Microsoft Graph API.`,
	Run: func(cmd *cobra.Command, args []string) {
		if mailAccount == "" || mailTo == "" || mailSubject == "" {
			fatal(cmd.Help())
			return
		}

		if err := mail.Send(cfg, mailAccount, mailTo, mailSubject, mailBody, mailForce); err != nil {
			fatal(err)
		}
	},
}

func init() {
	mailSendCmd.Flags().StringVar(&mailAccount, "account", "", "Account (required)")
	mailSendCmd.Flags().StringVar(&mailTo, "to", "", "Recipient email (required)")
	mailSendCmd.Flags().StringVar(&mailSubject, "subject", "", "Email subject (required)")
	mailSendCmd.Flags().StringVar(&mailBody, "body", "", "Email body")
	mailSendCmd.Flags().BoolVar(&mailForce, "force", false, "Bypass cross-tenant checks")

	mailCmd.AddCommand(mailSendCmd)
}
