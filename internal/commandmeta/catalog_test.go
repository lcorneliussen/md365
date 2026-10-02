package commandmeta

import "testing"

func TestEveryPolicyHasKnownMutability(t *testing.T) {
	for path, policy := range policies {
		if path == "" {
			t.Fatal("empty command path")
		}
		if policy.Mutability != Read && policy.Mutability != Write {
			t.Fatalf("%s mutability = %q", path, policy.Mutability)
		}
	}
}
