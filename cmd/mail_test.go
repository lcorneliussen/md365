package cmd

import (
	"testing"

	"github.com/lcorneliussen/md365/internal/apierr"
)

func TestMailSearchLimitDefaultIsIndependent(t *testing.T) {
	if mailSearchLimit != 25 {
		t.Fatalf("mail search limit = %d, want 25", mailSearchLimit)
	}
	if mailLimit != 25 {
		t.Fatalf("mail list limit = %d, want 25", mailLimit)
	}
}

func TestMailSearchRejectsBlankQueryAsUsageError(t *testing.T) {
	previousAccount, previousLimit := mailAccount, mailSearchLimit
	mailAccount, mailSearchLimit = "work", 25
	t.Cleanup(func() { mailAccount, mailSearchLimit = previousAccount, previousLimit })

	err := mailSearchCmd.RunE(mailSearchCmd, []string{"   "})
	if apierr.As(err).Code != apierr.CodeUsage {
		t.Fatalf("error code = %q, want usage (error: %v)", apierr.As(err).Code, err)
	}
}

func TestMailSearchRejectsInvalidArgumentCountsAsUsageError(t *testing.T) {
	for _, args := range [][]string{nil, {"one", "two"}} {
		err := mailSearchCmd.Args(mailSearchCmd, args)
		if apierr.As(err).Code != apierr.CodeUsage {
			t.Fatalf("args = %#v, error code = %q, want usage", args, apierr.As(err).Code)
		}
	}
}

func TestMailListRejectsNonPositiveLimitBeforeGraphCall(t *testing.T) {
	previousAccount, previousLimit := mailAccount, mailLimit
	mailAccount, mailLimit = "work", -1
	t.Cleanup(func() { mailAccount, mailLimit = previousAccount, previousLimit })

	err := mailListCmd.RunE(mailListCmd, nil)
	if apierr.As(err).Code != apierr.CodeUsage {
		t.Fatalf("error code = %q, want usage (error: %v)", apierr.As(err).Code, err)
	}
}
