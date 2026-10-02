package cmd

import (
	"encoding/json"
	"reflect"
	"testing"

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
