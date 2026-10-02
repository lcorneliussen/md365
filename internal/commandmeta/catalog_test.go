package commandmeta

import "testing"

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

func TestAllReturnsEffectivePolicy(t *testing.T) {
	policy := All()["sharepoint list"]
	if len(policy.OutputModes) == 0 {
		t.Fatal("effective policy is missing default output modes")
	}
	if len(policy.Constraints) != 2 {
		t.Fatalf("sharepoint constraints = %#v", policy.Constraints)
	}
}
