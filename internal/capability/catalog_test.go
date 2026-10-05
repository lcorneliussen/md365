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
	if len(plan.Commands) != 8 {
		t.Fatalf("commands = %d, want 8", len(plan.Commands))
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

func TestCacheFirstCommandsExposeConditionalGraphScopes(t *testing.T) {
	for _, name := range []string{"cal list", "contacts search"} {
		command, ok := findCommand(name)
		if !ok {
			t.Fatalf("%s command missing", name)
		}
		if len(command.Scopes) != 0 || len(command.ConditionalScopes) != 1 || command.ConditionalScopes[0].WhenFlag != "no-cache" {
			t.Fatalf("%s scopes = %#v, conditional = %#v", name, command.Scopes, command.ConditionalScopes)
		}
		if !Allowed(command, nil) {
			t.Fatalf("cache-backed %s should be allowed without Graph scopes", name)
		}
	}

	plan, err := Resolve([]string{"cal list", "contacts search"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Calendars.Read", "Contacts.Read", "User.Read", "offline_access"}
	if !reflect.DeepEqual(plan.Scopes, want) {
		t.Fatalf("planned scopes = %#v, want %#v", plan.Scopes, want)
	}
}

func TestResolveRejectsUnknownSelector(t *testing.T) {
	if _, err := Resolve([]string{"teams frobnicate"}, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveTeamsAndFilesFeatures(t *testing.T) {
	plan, err := Resolve(nil, []string{"teams-read", "files-read"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Channel.ReadBasic.All", "Files.Read.All", "Team.ReadBasic.All", "User.Read", "offline_access"}
	if !reflect.DeepEqual(plan.Scopes, want) {
		t.Fatalf("scopes = %#v, want %#v", plan.Scopes, want)
	}
}

func TestResolveFilesFeatureIncludesSharePointScope(t *testing.T) {
	plan, err := Resolve(nil, []string{"files-read"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Files.Read.All", "User.Read", "offline_access"}
	if !reflect.DeepEqual(plan.Scopes, want) {
		t.Fatalf("scopes = %#v, want %#v", plan.Scopes, want)
	}
	if len(plan.Commands) != 5 {
		t.Fatalf("commands = %d, want 5", len(plan.Commands))
	}
}

func TestResolveLoopFeatureUsesFilesReadAll(t *testing.T) {
	plan, err := Resolve(nil, []string{"loop-read"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Files.Read.All", "User.Read", "offline_access"}
	if !reflect.DeepEqual(plan.Scopes, want) {
		t.Fatalf("scopes = %#v, want %#v", plan.Scopes, want)
	}
	if len(plan.Commands) != 1 || plan.Commands[0].Name != "loop search" {
		t.Fatalf("commands = %#v", plan.Commands)
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
