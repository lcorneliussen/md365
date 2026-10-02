package mail

import (
	"testing"

	"github.com/lcorneliussen/md365/internal/config"
)

func TestValidateWriteEnforcesAccountAndTenantWithoutGraph(t *testing.T) {
	cfg := &config.Config{Accounts: map[string]*config.Account{
		"talendos": {Domains: []string{"talendos.com"}},
		"oms":      {Domains: []string{"oms.example"}},
	}}
	if err := ValidateWrite(cfg, "talendos", "colleague@talendos.com", false); err != nil {
		t.Fatal(err)
	}
	if err := ValidateWrite(cfg, "missing", "person@example.com", true); err == nil {
		t.Fatal("missing account accepted")
	}
	if err := ValidateWrite(cfg, "talendos", "person@oms.example", false); err == nil {
		t.Fatal("cross-tenant recipient accepted")
	}
	if err := ValidateWrite(cfg, "talendos", "person@oms.example", true); err != nil {
		t.Fatalf("explicit force rejected: %v", err)
	}
}
