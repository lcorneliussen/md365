package cmd

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/lcorneliussen/md365/internal/capability"
	"github.com/lcorneliussen/md365/internal/commandmeta"
)

func TestSchemaIsDeterministicAndCoversCommandSurface(t *testing.T) {
	first, err := buildSchema(rootCmd)
	if err != nil {
		t.Fatal(err)
	}
	second, err := buildSchema(rootCmd)
	if err != nil {
		t.Fatal(err)
	}
	firstJSON, _ := json.Marshal(first)
	secondJSON, _ := json.Marshal(second)
	if string(firstJSON) != string(secondJSON) {
		for i := range first.Commands {
			if !reflect.DeepEqual(first.Commands[i], second.Commands[i]) {
				t.Fatalf("schema output differs at %s\nfirst: %#v\nsecond: %#v", first.Commands[i].Path, first.Commands[i], second.Commands[i])
			}
		}
		t.Fatalf("schema output is not deterministic: first has %d commands, second has %d", len(first.Commands), len(second.Commands))
	}

	paths := map[string]bool{}
	for _, command := range first.Commands {
		if paths[command.Path] {
			t.Fatalf("duplicate command %q", command.Path)
		}
		paths[command.Path] = true
	}
	for _, command := range commandCatalog(rootCmd) {
		if !paths[command.Path] {
			t.Fatalf("command %q missing from schema", command.Path)
		}
	}
}

func TestSchemaPublishesArgumentsPermissionsAndPolicy(t *testing.T) {
	document, err := buildSchema(rootCmd)
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]schemaCommand{}
	for _, command := range document.Commands {
		byPath[command.Path] = command
	}

	search := byPath["md365 mail search"]
	if !reflect.DeepEqual(search.Arguments, []schemaArgument{{Name: "QUERY", Required: true}}) {
		t.Fatalf("mail search arguments = %#v", search.Arguments)
	}
	if !reflect.DeepEqual(search.DelegatedPermissions, []string{"Mail.Read"}) {
		t.Fatalf("mail search permissions = %#v", search.DelegatedPermissions)
	}
	if search.Mutability != commandmeta.Read || search.CanPrompt {
		t.Fatalf("mail search policy = %#v", search)
	}

	archive := byPath["md365 mail archive"]
	if !reflect.DeepEqual(archive.Arguments, []schemaArgument{{Name: "MESSAGE_ID", Required: false, Variadic: true}}) {
		t.Fatalf("mail archive arguments = %#v", archive.Arguments)
	}

	help := byPath["md365 help"]
	if !reflect.DeepEqual(help.Arguments, []schemaArgument{{Name: "command", Required: false, Variadic: true}}) {
		t.Fatalf("help arguments = %#v", help.Arguments)
	}

	send := byPath["md365 mail send"]
	if send.Mutability != commandmeta.Write || !reflect.DeepEqual(send.DelegatedPermissions, []string{"Mail.Send"}) {
		t.Fatalf("mail send contract = %#v", send)
	}

	get := byPath["md365 mail get"]
	required := map[string]bool{}
	for _, flag := range get.Flags {
		if flag.Required {
			required[flag.Name] = true
		}
	}
	if !required["account"] || !required["id"] {
		t.Fatalf("required flags = %#v", required)
	}

	authAdd := byPath["md365 auth add"]
	for _, flag := range authAdd.Flags {
		if flag.Name == "name" {
			if flag.Required || !reflect.DeepEqual(flag.RequiredUnless, []string{"interactive"}) {
				t.Fatalf("auth add name requirement = %#v", flag)
			}
			if flag.Default != "" {
				t.Fatalf("auth add name default = %q, want explicit empty string", flag.Default)
			}
		}
	}

	contacts := byPath["md365 contacts search"]
	if len(contacts.DelegatedPermissions) != 0 || len(contacts.ConditionalPermissions) != 1 || contacts.ConditionalPermissions[0].WhenFlag != "no-cache" {
		t.Fatalf("contacts permissions = %#v, conditional = %#v", contacts.DelegatedPermissions, contacts.ConditionalPermissions)
	}

	for _, command := range document.Commands {
		hasHelp := false
		for _, flag := range command.Flags {
			if flag.Name == "help" {
				hasHelp = true
			}
			if strings.Contains(strings.ToLower(flag.Usage), "(required)") && !flag.Required && len(flag.RequiredUnless) == 0 {
				t.Fatalf("%s --%s has required help text but no explicit requirement metadata", command.Path, flag.Name)
			}
		}
		if !hasHelp {
			t.Fatalf("%s omits its default --help flag", command.Path)
		}
	}
}

func TestSchemaPublishesKnownAliases(t *testing.T) {
	document, err := buildSchema(rootCmd)
	if err != nil {
		t.Fatal(err)
	}
	aliases := map[string][]string{}
	for _, command := range document.Commands {
		if len(command.Aliases) > 0 {
			aliases[command.Path] = command.Aliases
		}
	}
	want := map[string][]string{
		"md365 onedrive list":        {"ls"},
		"md365 sharepoint libraries": {"drives"},
		"md365 sharepoint list":      {"ls"},
		"md365 teams channels":       {"channel"},
		"md365 teams list":           {"ls"},
	}
	if !reflect.DeepEqual(aliases, want) {
		t.Fatalf("aliases = %#v, want %#v", aliases, want)
	}
}

func TestFormatCommandPermissionsIncludesConditions(t *testing.T) {
	command, ok := capability.CommandByName("contacts search")
	if !ok {
		t.Fatal("contacts search command missing")
	}
	if got, want := formatCommandPermissions(command), "--no-cache: Contacts.Read"; got != want {
		t.Fatalf("permissions = %q, want %q", got, want)
	}
}

func TestSchemaPublishesCompleteObservableEffects(t *testing.T) {
	document, err := buildSchema(rootCmd)
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]schemaCommand{}
	for _, command := range document.Commands {
		byPath[command.Path] = command
	}
	for path, want := range map[string][]string{
		"md365 auth add":   {"authentication", "browser", "keyring_write", "local_write"},
		"md365 cal create": {"external_communication", "local_write", "microsoft_graph_write"},
		"md365 cal delete": {"external_communication", "local_read", "local_write", "microsoft_graph_write"},
	} {
		if !reflect.DeepEqual(byPath[path].Effects, want) {
			t.Fatalf("%s effects = %#v, want %#v", path, byPath[path].Effects, want)
		}
	}
}

func TestSchemaPublishesOutputModesConstraintsAndFrameworkPolicy(t *testing.T) {
	document, err := buildSchema(rootCmd)
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]schemaCommand{}
	for _, command := range document.Commands {
		byPath[command.Path] = command
	}

	if !reflect.DeepEqual(byPath["md365 mail search"].OutputModes, []string{"human", "json", "quiet", "ids", "count"}) {
		t.Fatalf("mail search output modes = %#v", byPath["md365 mail search"].OutputModes)
	}
	if !reflect.DeepEqual(byPath["md365 schema"].OutputModes, []string{"human", "json", "quiet"}) {
		t.Fatalf("schema output modes = %#v", byPath["md365 schema"].OutputModes)
	}
	for _, path := range []string{"md365 auth add", "md365 auth login", "md365 skill"} {
		if !reflect.DeepEqual(byPath[path].OutputModes, []string{"human"}) {
			t.Fatalf("%s output modes = %#v", path, byPath[path].OutputModes)
		}
	}
	if byPath["md365 help"].Mutability != commandmeta.Read || !reflect.DeepEqual(byPath["md365 help"].OutputModes, []string{"human"}) {
		t.Fatalf("help contract = %#v", byPath["md365 help"])
	}

	sharepoint := byPath["md365 sharepoint list"].Constraints
	if len(sharepoint) != 2 || sharepoint[0].Kind != "exactly_one" || sharepoint[1].Kind != "mutually_exclusive" {
		t.Fatalf("sharepoint list constraints = %#v", sharepoint)
	}
	for _, path := range []string{"md365 cal delete", "md365 mail mark-read", "md365 mail archive", "md365 mail delete"} {
		constraints := byPath[path].Constraints
		if len(constraints) != 1 || constraints[0].Kind != "at_least_one" {
			t.Fatalf("%s constraints = %#v", path, constraints)
		}
	}
	authPlan := byPath["md365 auth plan"].Constraints
	if len(authPlan) != 1 || authPlan[0].Kind != "at_least_one" {
		t.Fatalf("auth plan constraints = %#v", authPlan)
	}
	for _, path := range []string{"md365 auth add", "md365 auth login"} {
		authConstraints := byPath[path].Constraints
		if len(authConstraints) != 2 {
			t.Fatalf("%s constraints = %#v", path, authConstraints)
		}
		for _, constraint := range authConstraints {
			if constraint.Kind != "mutually_exclusive" || len(constraint.Options) != 2 || len(constraint.Options[0].Flags) != 1 || len(constraint.Options[1].Flags) != 1 {
				t.Fatalf("%s constraint is not pairwise: %#v", path, constraint)
			}
		}
	}
}

func TestSchemaPublishesOnlyEmittedExitStatuses(t *testing.T) {
	document, err := buildSchema(rootCmd)
	if err != nil {
		t.Fatal(err)
	}
	want := []schemaExitStatus{
		{Code: "ok", Status: 0},
		{Code: "usage", Status: 1},
		{Code: "unknown", Status: 1},
		{Code: "auth", Status: 3},
		{Code: "graph", Status: 7},
	}
	if !reflect.DeepEqual(document.ExitStatuses, want) {
		t.Fatalf("exit statuses = %#v, want %#v", document.ExitStatuses, want)
	}
}

func TestGraphCapabilityValidationRequiresDelegatedPermissions(t *testing.T) {
	if err := validateGraphCapability("md365 example", capability.Command{}, false); err == nil || !strings.Contains(err.Error(), "no capability metadata") {
		t.Fatalf("missing capability error = %v", err)
	}
	if err := validateGraphCapability("md365 example", capability.Command{Name: "example"}, true); err == nil || !strings.Contains(err.Error(), "no delegated permissions") {
		t.Fatalf("empty capability error = %v", err)
	}
	conditional := capability.Command{Name: "example", ConditionalScopes: []capability.ConditionalScopes{{WhenFlag: "live", Scopes: []string{"User.Read"}}}}
	if err := validateGraphCapability("md365 example", conditional, true); err != nil {
		t.Fatalf("conditional capability rejected: %v", err)
	}
}

func TestEveryExecutionPolicyAppearsInSchema(t *testing.T) {
	document, err := buildSchema(rootCmd)
	if err != nil {
		t.Fatal(err)
	}
	paths := map[string]bool{}
	for _, command := range document.Commands {
		paths[command.Path] = true
	}
	for path := range commandmeta.All() {
		if !paths["md365 "+path] {
			t.Fatalf("policy command %q missing from schema", path)
		}
	}
}
