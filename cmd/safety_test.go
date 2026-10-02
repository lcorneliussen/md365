package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/lcorneliussen/md365/internal/apierr"
	"github.com/lcorneliussen/md365/internal/commandmeta"
	"github.com/lcorneliussen/md365/internal/config"
	"github.com/lcorneliussen/md365/internal/output"
	"github.com/spf13/cobra"
)

func TestReadOnlyPolicyCoversEveryRunnableCommand(t *testing.T) {
	oldReadOnly, oldNoInput, oldDryRun := readOnlyFlag, noInputFlag, dryRunFlag
	readOnlyFlag, noInputFlag, dryRunFlag = true, false, false
	t.Cleanup(func() { readOnlyFlag, noInputFlag, dryRunFlag = oldReadOnly, oldNoInput, oldDryRun })

	var walk func(*cobra.Command)
	walk = func(command *cobra.Command) {
		if command.Hidden {
			return
		}
		if command.Runnable() {
			path := strings.TrimPrefix(command.CommandPath(), "md365 ")
			policy, ok := commandmeta.Lookup(path)
			if !ok {
				t.Fatalf("%s has no execution policy", command.CommandPath())
			}
			err := enforceExecutionPolicy(command, nil)
			if policy.Mutability == commandmeta.Write {
				if apierr.As(err).Code != apierr.CodePolicy {
					t.Fatalf("%s bypassed --read-only: %v", command.CommandPath(), err)
				}
			} else if err != nil {
				t.Fatalf("read-only command %s was blocked: %v", command.CommandPath(), err)
			}
		}
		for _, child := range command.Commands() {
			walk(child)
		}
	}
	walk(rootCmd)
}

func TestNoInputUsesInvocationAwarePromptPolicy(t *testing.T) {
	conditional := &cobra.Command{Use: "add"}
	conditional.Flags().Bool("interactive", false, "")
	conditional.Flags().Bool("login", false, "")
	policy := commandmeta.Policy{CanPrompt: true, Prompting: commandmeta.Prompting{Mode: "conditional", WhenAnyFlags: []string{"interactive", "login"}}}
	if invocationCanPrompt(conditional, policy) {
		t.Fatal("non-interactive auth add reported prompting")
	}
	if err := conditional.Flags().Set("login", "true"); err != nil {
		t.Fatal(err)
	}
	if !invocationCanPrompt(conditional, policy) {
		t.Fatal("auth add --login did not report prompting")
	}
	if !invocationCanPrompt(conditional, commandmeta.Policy{CanPrompt: true, Prompting: commandmeta.Prompting{Mode: "always"}}) {
		t.Fatal("always-prompting policy did not report prompting")
	}
}

func TestNoInputBlocksMicrosoftEntraLoginBeforeExecution(t *testing.T) {
	oldNoInput, oldReadOnly, oldDryRun := noInputFlag, readOnlyFlag, dryRunFlag
	noInputFlag, readOnlyFlag, dryRunFlag = true, false, false
	t.Cleanup(func() { noInputFlag, readOnlyFlag, dryRunFlag = oldNoInput, oldReadOnly, oldDryRun })
	err := enforceExecutionPolicy(authLoginCmd, nil)
	if apierr.As(err).Code != apierr.CodePolicy {
		t.Fatalf("auth login under --no-input = %v", err)
	}
}

func TestPolicyDenialPrecedesConfigurationLoading(t *testing.T) {
	oldReadOnly, oldNoInput, oldDryRun, oldLoad := readOnlyFlag, noInputFlag, dryRunFlag, loadConfig
	called := false
	loadConfig = func() (*config.Config, error) {
		called = true
		return nil, errors.New("broken config")
	}
	t.Cleanup(func() {
		readOnlyFlag, noInputFlag, dryRunFlag, loadConfig = oldReadOnly, oldNoInput, oldDryRun, oldLoad
	})

	readOnlyFlag, noInputFlag, dryRunFlag = true, false, false
	err := prepareCommand(syncCmd, nil)
	if apierr.As(err).Code != apierr.CodePolicy || called {
		t.Fatalf("read-only preflight = %v, config called = %v", err, called)
	}

	readOnlyFlag, noInputFlag, dryRunFlag = false, true, false
	err = prepareCommand(authLoginCmd, nil)
	if apierr.As(err).Code != apierr.CodePolicy || called {
		t.Fatalf("no-input preflight = %v, config called = %v", err, called)
	}
}

func TestUnsupportedDryRunFailsClosed(t *testing.T) {
	oldDryRun, oldReadOnly, oldNoInput := dryRunFlag, readOnlyFlag, noInputFlag
	dryRunFlag, readOnlyFlag, noInputFlag = true, false, false
	t.Cleanup(func() { dryRunFlag, readOnlyFlag, noInputFlag = oldDryRun, oldReadOnly, oldNoInput })
	err := enforceExecutionPolicy(syncCmd, nil)
	if apierr.As(err).Code != apierr.CodePolicy || !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("sync --dry-run = %v", err)
	}
}

func TestFallbackWriterPreservesContentSafetyOptions(t *testing.T) {
	oldJSON, oldQuiet, oldIDs, oldCount := jsonFlag, quietFlag, idsOnlyFlag, countFlag
	oldWrap, oldSanitize := wrapUntrustedFlag, sanitizeContentFlag
	jsonFlag, quietFlag, idsOnlyFlag, countFlag = false, false, false, false
	wrapUntrustedFlag, sanitizeContentFlag = true, true
	t.Cleanup(func() {
		jsonFlag, quietFlag, idsOnlyFlag, countFlag = oldJSON, oldQuiet, oldIDs, oldCount
		wrapUntrustedFlag, sanitizeContentFlag = oldWrap, oldSanitize
	})

	var stdout, stderr bytes.Buffer
	fallback := newOutputWriter(&stdout, &stderr)
	if fallback.IsHuman() {
		t.Fatal("fallback lost content-safety mode")
	}
	fallback.Err(errors.New("early failure"))
	var response output.ErrorResponse
	if err := json.Unmarshal(stderr.Bytes(), &response); err != nil {
		t.Fatalf("fallback error is not JSON: %v (%s)", err, stderr.String())
	}
	if response.OK || response.Code != apierr.CodeUnknown {
		t.Fatalf("fallback error = %#v", response)
	}
}

func TestPrescanContentSafetyFlagsBeforeUnknownCommand(t *testing.T) {
	oldWrap, oldSanitize := wrapUntrustedFlag, sanitizeContentFlag
	wrapUntrustedFlag, sanitizeContentFlag = false, false
	t.Cleanup(func() { wrapUntrustedFlag, sanitizeContentFlag = oldWrap, oldSanitize })

	prescanContentSafetyFlags([]string{"--wrap-untrusted", "--sanitize-content=true", "unknown", "--wrap-untrusted=false"})
	if !wrapUntrustedFlag || !sanitizeContentFlag {
		t.Fatalf("prescanned flags = wrap:%v sanitize:%v", wrapUntrustedFlag, sanitizeContentFlag)
	}
}

func TestMailSendDryRunIsRedactedAndCompletesWithoutExecution(t *testing.T) {
	oldAccount, oldTo, oldSubject, oldBody, oldForce := mailAccount, mailTo, mailSubject, mailBody, mailForce
	oldDryRun, oldReadOnly, oldNoInput, oldWriter, oldCfg := dryRunFlag, readOnlyFlag, noInputFlag, writer, cfg
	mailAccount, mailTo, mailSubject, mailBody, mailForce = "talendos", "one@example.com,two@example.com", "Quarterly close", "secret body", false
	dryRunFlag, readOnlyFlag, noInputFlag = true, true, false
	cfg = testSafetyConfig()
	var stdout bytes.Buffer
	writer = output.New(output.Options{Format: output.FormatJSON, Stdout: &stdout, Stderr: &stdout})
	t.Cleanup(func() {
		mailAccount, mailTo, mailSubject, mailBody, mailForce = oldAccount, oldTo, oldSubject, oldBody, oldForce
		dryRunFlag, readOnlyFlag, noInputFlag, writer, cfg = oldDryRun, oldReadOnly, oldNoInput, oldWriter, oldCfg
	})

	err := enforceExecutionPolicy(mailSendCmd, nil)
	if !errors.Is(err, errDryRunComplete) {
		t.Fatalf("dry-run result = %v", err)
	}
	if strings.Contains(stdout.String(), "secret body") {
		t.Fatalf("dry-run output leaked body: %s", stdout.String())
	}
	var response output.Response
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(response.Data)
	if !strings.Contains(string(encoded), `"body_length":11`) || !strings.Contains(string(encoded), `"dry_run":true`) {
		t.Fatalf("dry-run preview = %s", encoded)
	}
}

func TestRepresentativeDryRunPlans(t *testing.T) {
	oldMailAccount, oldMailIDs := mailAccount, mailIDs
	oldCalAccount, oldCalID, oldCalFile, oldCfg := calAccount, calID, calFile, cfg
	mailAccount, mailIDs = "talendos", []string{"message-1"}
	calAccount, calID, calFile = "talendos", "event-1", ""
	cfg = testSafetyConfig()
	t.Cleanup(func() {
		mailAccount, mailIDs = oldMailAccount, oldMailIDs
		calAccount, calID, calFile = oldCalAccount, oldCalID, oldCalFile
		cfg = oldCfg
	})

	tests := []struct {
		path      string
		operation string
		methods   []string
	}{
		{"mail mark-read", "mark_message_read", []string{"PATCH"}},
		{"mail archive", "archive_message", []string{"PATCH", "POST"}},
		{"mail delete", "move_message_to_deleted_items", []string{"DELETE"}},
		{"cal delete", "delete_event", []string{"DELETE"}},
	}
	for _, test := range tests {
		policy, _ := commandmeta.Lookup(test.path)
		preview, err := buildDryRunPreview(test.path, nil, policy)
		if err != nil {
			t.Fatalf("%s: %v", test.path, err)
		}
		methods := make([]string, 0, len(preview.Operations))
		for _, operation := range preview.Operations {
			methods = append(methods, operation.Method)
		}
		if preview.Operation != test.operation || !reflect.DeepEqual(methods, test.methods) {
			t.Fatalf("%s preview = %#v", test.path, preview)
		}
	}

	policy, _ := commandmeta.Lookup("mail mark-read")
	preview, err := buildDryRunPreview("mail mark-read", []string{"message-2", "message-1"}, policy)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(preview.Operations[0].Request["message_ids"], []string{"message-1", "message-2"}) {
		t.Fatalf("deduplicated message IDs = %#v", preview.Operations[0].Request["message_ids"])
	}
}

func testSafetyConfig() *config.Config {
	return &config.Config{
		Timezone: "Europe/Berlin",
		Accounts: map[string]*config.Account{
			"talendos": {Domains: []string{"example.com"}},
		},
	}
}
