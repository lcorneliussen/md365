package mail

import (
	"strings"
	"testing"

	"github.com/lcorneliussen/md365/internal/config"
)

func TestPlanWriteNormalizesRecipientsAndEnforcesTenantWithoutGraph(t *testing.T) {
	cfg := &config.Config{Accounts: map[string]*config.Account{
		"talendos": {Domains: []string{"talendos.com"}},
		"oms":      {Domains: []string{"oms.example"}},
	}}
	recipients, err := PlanWrite(cfg, "talendos", " one@talendos.com;two@talendos.com, three@talendos.com ", false)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(recipients, ","); got != "one@talendos.com,two@talendos.com,three@talendos.com" {
		t.Fatalf("normalized recipients = %q", got)
	}
	if _, err := PlanWrite(cfg, "missing", "person@example.com", true); err == nil {
		t.Fatal("missing account accepted")
	}
	if _, err := PlanWrite(cfg, "talendos", "person@oms.example", false); err == nil {
		t.Fatal("cross-tenant recipient accepted")
	}
	if _, err := PlanWrite(cfg, "talendos", "person@oms.example", true); err != nil {
		t.Fatalf("explicit force rejected: %v", err)
	}
}

func TestValidateAccountRejectsRemovedAccountBeforeTokenLookup(t *testing.T) {
	cfg := &config.Config{Accounts: map[string]*config.Account{"talendos": {}}}
	if err := ValidateAccount(cfg, "talendos"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateAccount(cfg, "removed"); err == nil {
		t.Fatal("removed account accepted")
	}
	if _, _, err := MarkRead(cfg, "removed", []string{"message-1"}); err == nil || !strings.Contains(err.Error(), "not found in config") {
		t.Fatalf("mark-read account validation = %v", err)
	}
}
