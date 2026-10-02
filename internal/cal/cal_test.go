package cal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lcorneliussen/md365/internal/config"
)

func TestPlanCreateValidatesWithoutGraph(t *testing.T) {
	cfg := &config.Config{
		Timezone: "Europe/Berlin",
		Accounts: map[string]*config.Account{
			"talendos": {Domains: []string{"talendos.com"}},
			"oms":      {Domains: []string{"oms.example"}},
		},
	}
	event, err := PlanCreate(cfg, "talendos", "Review", "2026-10-03 10:00", "2026-10-03 11:00", "Teams", "private body", []string{"colleague@talendos.com"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if event.Subject != "Review" || event.Start.DateTime == "" || event.End.DateTime == "" || len(event.Attendees) != 1 {
		t.Fatalf("planned event = %#v", event)
	}
	if _, err := PlanCreate(cfg, "talendos", "Review", "invalid", "2026-10-03 11:00", "", "", nil, false); err == nil {
		t.Fatal("invalid start accepted")
	}
	if _, err := PlanCreate(cfg, "talendos", "Review", "2026-10-03 10:00", "2026-10-03 11:00", "", "", []string{"person@oms.example"}, false); err == nil {
		t.Fatal("cross-tenant attendee accepted")
	}
}

func TestResolveDeleteReadsCachedEventWithoutDeletingIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "event.md")
	content := "---\naccount: talendos\nid: event-123\n---\n\n# Review\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	account, id, err := ResolveDelete("", "", path)
	if err != nil {
		t.Fatal(err)
	}
	if account != "talendos" || id != "event-123" {
		t.Fatalf("resolved locator = %q/%q", account, id)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("planning removed cached event: %v", err)
	}
}
