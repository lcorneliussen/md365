package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/lcorneliussen/md365/internal/apierr"
	"github.com/lcorneliussen/md365/internal/cal"
	"github.com/lcorneliussen/md365/internal/commandmeta"
	"github.com/lcorneliussen/md365/internal/mail"
	"github.com/lcorneliussen/md365/internal/output"
	"github.com/spf13/cobra"
)

var errDryRunComplete = errors.New("dry run complete")

type dryRunPreview struct {
	DryRun     bool              `json:"dry_run"`
	Command    string            `json:"command"`
	Workload   string            `json:"workload"`
	Operation  string            `json:"operation"`
	Account    string            `json:"account,omitempty"`
	Operations []dryRunOperation `json:"operations"`
	Effects    []string          `json:"effects,omitempty"`
	Redactions []string          `json:"redactions,omitempty"`
}

type dryRunOperation struct {
	Method   string         `json:"method"`
	Resource string         `json:"resource"`
	Request  map[string]any `json:"request,omitempty"`
}

func commandPolicy(cmd *cobra.Command) (string, commandmeta.Policy, bool) {
	parts := strings.Fields(cmd.CommandPath())
	if len(parts) < 2 {
		return "", commandmeta.Policy{}, false
	}
	path := strings.Join(parts[1:], " ")
	policy, ok := commandmeta.Lookup(path)
	if !ok || !cmd.Runnable() {
		return path, policy, false
	}
	return path, policy, true
}

func enforcePreConfigPolicy(cmd *cobra.Command) error {
	_, policy, ok := commandPolicy(cmd)
	if !ok {
		return nil
	}
	if noInputFlag && invocationCanPrompt(cmd, policy) {
		return apierr.Policy(fmt.Sprintf("%s may require interactive Microsoft Entra authentication; remove --no-input to continue", cmd.CommandPath()))
	}

	if dryRunFlag {
		if policy.Mutability != commandmeta.Write {
			return apierr.Policy(fmt.Sprintf("--dry-run applies only to mutating commands; %s is read-only", cmd.CommandPath()))
		}
		if !policy.DryRunSupported {
			return apierr.Policy(fmt.Sprintf("--dry-run is not supported for %s", cmd.CommandPath()))
		}
		return nil
	}

	if readOnlyFlag && policy.Mutability == commandmeta.Write {
		return apierr.Policy(fmt.Sprintf("%s is blocked by --read-only", cmd.CommandPath()))
	}
	return nil
}

func executeDryRun(cmd *cobra.Command, args []string) error {
	if !dryRunFlag {
		return nil
	}
	path, policy, ok := commandPolicy(cmd)
	if !ok {
		return nil
	}
	preview, err := buildDryRunPreview(path, args, policy)
	if err != nil {
		return err
	}
	if err := writeOK(preview, output.WithMeta("dry_run", true)); err != nil {
		return err
	}
	return errDryRunComplete
}

func enforceExecutionPolicy(cmd *cobra.Command, args []string) error {
	if err := enforcePreConfigPolicy(cmd); err != nil {
		return err
	}
	return executeDryRun(cmd, args)
}

func invocationCanPrompt(cmd *cobra.Command, policy commandmeta.Policy) bool {
	if !policy.CanPrompt {
		return false
	}
	switch policy.Prompting.Mode {
	case "always":
		return true
	case "conditional":
		for _, name := range policy.Prompting.WhenAnyFlags {
			flag := cmd.Flags().Lookup(name)
			if flag == nil {
				flag = cmd.InheritedFlags().Lookup(name)
			}
			if flag == nil {
				flag = cmd.Root().PersistentFlags().Lookup(name)
			}
			if flag != nil && flag.Value.String() == "true" {
				return true
			}
		}
	}
	return false
}

func buildDryRunPreview(path string, args []string, policy commandmeta.Policy) (dryRunPreview, error) {
	preview := dryRunPreview{
		DryRun:  true,
		Command: "md365 " + path,
		Account: commandAccount(path),
		Effects: append([]string(nil), policy.Effects...),
	}
	switch path {
	case "mail send", "mail draft":
		if mailAccount == "" || mailTo == "" || mailSubject == "" {
			return dryRunPreview{}, usageError("--account, --to, and --subject are required")
		}
		recipients, err := mail.PlanWrite(cfg, mailAccount, mailTo, mailForce)
		if err != nil {
			return dryRunPreview{}, err
		}
		preview.Workload = "Exchange Online"
		preview.Operation = "send_message"
		resource := "/me/sendMail"
		if path == "mail draft" {
			resource = "/me/messages"
			preview.Operation = "create_draft"
		}
		preview.Operations = []dryRunOperation{{
			Method: "POST", Resource: resource,
			Request: map[string]any{
				"to":          recipients,
				"subject":     mailSubject,
				"body_length": len(mailBody),
			},
		}}
		preview.Redactions = []string{"body"}
	case "mail mark-read", "mail archive", "mail delete":
		if mailAccount == "" {
			return dryRunPreview{}, usageError("--account is required")
		}
		if err := mail.ValidateAccount(cfg, mailAccount); err != nil {
			return dryRunPreview{}, err
		}
		ids := collectIDs(mailIDs, args)
		if len(ids) == 0 {
			return dryRunPreview{}, usageError("no message IDs provided (use --id or positional arguments)")
		}
		preview.Workload = "Exchange Online"
		request := map[string]any{"message_ids": ids, "message_count": len(ids)}
		switch path {
		case "mail mark-read":
			preview.Operation = "mark_message_read"
			request["is_read"] = true
			preview.Operations = []dryRunOperation{{Method: "PATCH", Resource: "/me/messages/{id}", Request: request}}
		case "mail archive":
			preview.Operation = "archive_message"
			preview.Operations = []dryRunOperation{
				{Method: "PATCH", Resource: "/me/messages/{id}", Request: map[string]any{"message_ids": ids, "message_count": len(ids), "is_read": true}},
				{Method: "POST", Resource: "/me/messages/{id}/move", Request: map[string]any{"message_ids": ids, "message_count": len(ids), "destination_id": "archive"}},
			}
		case "mail delete":
			preview.Operation = "move_message_to_deleted_items"
			preview.Operations = []dryRunOperation{{Method: "DELETE", Resource: "/me/messages/{id}", Request: request}}
		}
	case "cal create":
		if calAccount == "" || calSubject == "" || calStart == "" || calEnd == "" {
			return dryRunPreview{}, usageError("--account, --subject, --start, and --end are required")
		}
		event, err := cal.PlanCreate(cfg, calAccount, calSubject, calStart, calEnd, calLocation, calBody, calAttendees, calForce)
		if err != nil {
			return dryRunPreview{}, err
		}
		preview.Workload = "Outlook calendar"
		preview.Operation = "create_event"
		preview.Operations = []dryRunOperation{{
			Method: "POST", Resource: "/me/events",
			Request: map[string]any{
				"subject":        calSubject,
				"start":          event.Start,
				"end":            event.End,
				"location":       calLocation,
				"attendees":      append([]string(nil), calAttendees...),
				"attendee_count": len(calAttendees),
				"body_length":    len(calBody),
			},
		}}
		preview.Redactions = []string{"body"}
	case "cal delete":
		file := calFile
		if len(args) > 0 {
			file = args[0]
		}
		account, id, err := cal.ResolveDelete(calAccount, calID, file)
		if err != nil {
			return dryRunPreview{}, err
		}
		if _, err := cfg.GetAccount(account); err != nil {
			return dryRunPreview{}, err
		}
		preview.Account = account
		preview.Workload = "Outlook calendar"
		preview.Operation = "delete_event"
		preview.Operations = []dryRunOperation{{Method: "DELETE", Resource: "/me/events/{id}", Request: map[string]any{"event_id": id, "cached_event_file": file}}}
	default:
		return dryRunPreview{}, apierr.Policy(fmt.Sprintf("--dry-run is not implemented for md365 %s", path))
	}
	return preview, nil
}

func commandAccount(path string) string {
	if strings.HasPrefix(path, "mail ") {
		return mailAccount
	}
	if strings.HasPrefix(path, "cal ") {
		return calAccount
	}
	return ""
}
