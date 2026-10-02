package graph

import "testing"

func TestGraphRecipientsPreservesNormalizedAddresses(t *testing.T) {
	recipients := graphRecipients([]string{"one@example.com", "two@example.com"})
	if len(recipients) != 2 || recipients[0].EmailAddress.Address != "one@example.com" || recipients[1].EmailAddress.Address != "two@example.com" {
		t.Fatalf("Graph recipients = %#v", recipients)
	}
}
