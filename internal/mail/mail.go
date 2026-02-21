package mail

import (
	"fmt"

	"github.com/larsc/md365/internal/auth"
	"github.com/larsc/md365/internal/config"
	"github.com/larsc/md365/internal/graph"
)

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
