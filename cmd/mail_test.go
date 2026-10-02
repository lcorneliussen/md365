package cmd

import "testing"

func TestMailSearchLimitDefaultIsIndependent(t *testing.T) {
	if mailSearchLimit != 25 {
		t.Fatalf("mail search limit = %d, want 25", mailSearchLimit)
	}
	if mailLimit != 25 {
		t.Fatalf("mail list limit = %d, want 25", mailLimit)
	}
}
