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
	DryRun     bool           `json:"dry_run"`
	Command    string         `json:"command"`
	Workload   string         `json:"workload"`
	Operation  string         `json:"operation"`
	Method     string         `json:"method,omitempty"`
	Resource   string         `json:"resource"`
	Account    string         `json:"account,omitempty"`
	Request    map[string]any `json:"request,omitempty"`
	Effects    []string       `json:"effects,omitempty"`
	Redactions []string       `json:"redactions,omitempty"`
}

func enforceExecutionPolicy(cmd *cobra.Command, args []string) error {
	parts := strings.Fields(cmd.CommandPath())
	if len(parts) < 2 {
		return nil
	}
	path := strings.Join(parts[1:], " ")
	policy, ok := commandmeta.Lookup(path)
	if !ok || !cmd.Runnable() {
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
		preview, err := buildDryRunPreview(path, args, policy)
		if err != nil {
			return err
		}
		if err := writeOK(preview, output.WithMeta("dry_run", true)); err != nil {
			return err
		}
		return errDryRunComplete
	}

	if readOnlyFlag && policy.Mutability == commandmeta.Write {
		return apierr.Policy(fmt.Sprintf("%s is blocked by --read-only", cmd.CommandPath()))
	}
	return nil
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
		if err := mail.ValidateWrite(cfg, mailAccount, mailTo, mailForce); err != nil {
			return dryRunPreview{}, err
		}
		preview.Workload = "Exchange Online"
		preview.Method = "POST"
		preview.Resource = "/me/sendMail"
		preview.Operation = "send_message"
		if path == "mail draft" {
			preview.Resource = "/me/messages"
			preview.Operation = "create_draft"
		}
		preview.Request = map[string]any{
			"to":          splitRecipients(mailTo),
			"subject":     mailSubject,
			"body_length": len(mailBody),
			"force":       mailForce,
		}
		preview.Redactions = []string{"body"}
	case "mail mark-read", "mail archive", "mail delete":
		if mailAccount == "" {
			return dryRunPreview{}, usageError("--account is required")
		}
		if _, err := cfg.GetAccount(mailAccount); err != nil {
			return dryRunPreview{}, err
		}
		ids := collectIDs(mailIDs, args)
		if len(ids) == 0 {
			return dryRunPreview{}, usageError("no message IDs provided (use --id or positional arguments)")
		}
		preview.Workload = "Exchange Online"
		preview.Resource = "/me/messages/{id}"
		preview.Request = map[string]any{"message_ids": ids, "message_count": len(ids)}
		switch path {
		case "mail mark-read":
			preview.Method, preview.Operation = "PATCH", "mark_message_read"
			preview.Request["is_read"] = true
		case "mail archive":
			preview.Method, preview.Operation = "POST", "move_message_to_archive"
			preview.Request["destination"] = "archive"
		case "mail delete":
			preview.Method, preview.Operation = "POST", "move_message_to_deleted_items"
			preview.Request["destination"] = "deleteditems"
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
		preview.Method = "POST"
		preview.Resource = "/me/events"
		preview.Operation = "create_event"
		preview.Request = map[string]any{
			"subject":        calSubject,
			"start":          event.Start,
			"end":            event.End,
			"location":       calLocation,
			"attendees":      append([]string(nil), calAttendees...),
			"attendee_count": len(calAttendees),
			"body_length":    len(calBody),
			"force":          calForce,
		}
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
		preview.Method = "DELETE"
		preview.Resource = "/me/events/{id}"
		preview.Operation = "delete_event"
		preview.Request = map[string]any{"event_id": id, "cached_event_file": file}
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

func splitRecipients(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ';' })
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if recipient := strings.TrimSpace(part); recipient != "" {
			result = append(result, recipient)
		}
	}
	return result
}
