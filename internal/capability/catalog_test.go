package capability

import (
	"reflect"
	"testing"
)

func TestResolveMinimalScopes(t *testing.T) {
	plan, err := Resolve([]string{"mail list,mail archive", "cal *"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Calendars.ReadWrite", "Mail.ReadWrite", "User.Read", "offline_access"}
	if !reflect.DeepEqual(plan.Scopes, want) {
		t.Fatalf("scopes = %#v, want %#v", plan.Scopes, want)
	}
}

func TestResolveFeature(t *testing.T) {
	plan, err := Resolve(nil, []string{"mail-manage"})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Commands) != 6 {
		t.Fatalf("commands = %d, want 6", len(plan.Commands))
	}
	if !reflect.DeepEqual(plan.Scopes, []string{"Mail.ReadWrite", "User.Read", "offline_access"}) {
		t.Fatalf("unexpected scopes: %#v", plan.Scopes)
	}
}

func TestAllowedAcceptsReadWriteForRead(t *testing.T) {
	command, _ := findCommand("mail list")
	if !Allowed(command, []string{"Mail.ReadWrite"}) {
		t.Fatal("Mail.ReadWrite should satisfy Mail.Read")
	}
}

func TestResolveRejectsUnknownSelector(t *testing.T) {
	if _, err := Resolve([]string{"teams list"}, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestAllowedAcceptsFullyQualifiedGraphScopes(t *testing.T) {
	command, ok := findCommand("mail list")
	if !ok {
		t.Fatal("mail list command missing")
	}
	granted := []string{"https://graph.microsoft.com/Mail.ReadWrite"}
	if !Allowed(command, granted) {
		t.Fatal("Mail.ReadWrite URL scope should satisfy Mail.Read")
	}
}
