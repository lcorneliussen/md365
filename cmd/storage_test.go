package cmd

import "testing"

func TestStorageLimitDefaultsAreIndependent(t *testing.T) {
	if filesSearchLimit != 25 {
		t.Fatalf("files search limit = %d, want 25", filesSearchLimit)
	}
	if storageLimit != 100 {
		t.Fatalf("storage list limit = %d, want 100", storageLimit)
	}
}
