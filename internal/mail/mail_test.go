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
