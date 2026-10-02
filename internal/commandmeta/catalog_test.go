package commandmeta

import (
	"reflect"
	"sort"
	"testing"
)

func TestEveryPolicyHasKnownMutability(t *testing.T) {
	for path, policy := range All() {
		if path == "" {
			t.Fatal("empty command path")
		}
		if policy.Mutability != Read && policy.Mutability != Write {
			t.Fatalf("%s mutability = %q", path, policy.Mutability)
		}
	}
}

func TestDryRunAndPromptingPoliciesAreExplicit(t *testing.T) {
	var dryRun []string
	for path, policy := range All() {
		if policy.DryRunSupported {
			if policy.Mutability != Write {
				t.Fatalf("read-only command %s advertises dry-run", path)
			}
			dryRun = append(dryRun, path)
		}
		if policy.CanPrompt && policy.Prompting.Mode == "" {
			t.Fatalf("prompting command %s has no invocation policy", path)
		}
		if !policy.CanPrompt && policy.Prompting.Mode != "" {
			t.Fatalf("non-prompting command %s has prompting metadata", path)
		}
	}
	sort.Strings(dryRun)
	want := []string{"cal create", "cal delete", "mail archive", "mail delete", "mail draft", "mail mark-read", "mail send"}
	if !reflect.DeepEqual(dryRun, want) {
		t.Fatalf("dry-run commands = %#v, want %#v", dryRun, want)
	}
}

func TestAllReturnsEffectivePolicy(t *testing.T) {
	policy := All()["sharepoint list"]
	if len(policy.OutputModes) == 0 {
		t.Fatal("effective policy is missing default output modes")
	}
	if len(policy.Constraints) != 2 {
		t.Fatalf("sharepoint constraints = %#v", policy.Constraints)
	}
}
