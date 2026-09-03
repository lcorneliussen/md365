package capability

import (
	"fmt"
	"sort"
	"strings"
)

// Command describes the delegated Microsoft Graph scopes required by a CLI command.
type Command struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Scopes      []string `json:"scopes"`
}

// Feature is a convenient bundle of related commands.
type Feature struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Commands    []string `json:"commands"`
}

// Plan is the resolved, least-privilege authorization request.
type Plan struct {
	Commands []Command `json:"commands"`
	Features []string  `json:"features,omitempty"`
	Scopes   []string  `json:"scopes"`
}

var commands = []Command{
	{Name: "mail list", Description: "List messages", Scopes: []string{"Mail.Read"}},
	{Name: "mail get", Description: "Read a message", Scopes: []string{"Mail.Read"}},
	{Name: "mail attachments", Description: "List message attachments", Scopes: []string{"Mail.Read"}},
	{Name: "mail mark-read", Description: "Mark messages as read", Scopes: []string{"Mail.ReadWrite"}},
	{Name: "mail archive", Description: "Archive messages", Scopes: []string{"Mail.ReadWrite"}},
	{Name: "mail delete", Description: "Move messages to Deleted Items", Scopes: []string{"Mail.ReadWrite"}},
	{Name: "mail draft", Description: "Create message drafts", Scopes: []string{"Mail.ReadWrite"}},
	{Name: "mail send", Description: "Send messages", Scopes: []string{"Mail.Send"}},
	{Name: "cal list", Description: "List calendar events", Scopes: []string{"Calendars.Read"}},
	{Name: "cal create", Description: "Create calendar events", Scopes: []string{"Calendars.ReadWrite"}},
	{Name: "cal delete", Description: "Delete calendar events", Scopes: []string{"Calendars.ReadWrite"}},
	{Name: "sync", Description: "Sync calendars and contacts", Scopes: []string{"Calendars.Read", "Contacts.Read"}},
}

var features = []Feature{
	{Name: "mail-read", Description: "Read mail and attachments", Commands: []string{"mail list", "mail get", "mail attachments"}},
	{Name: "mail-manage", Description: "Read, draft, mark, archive, and delete mail", Commands: []string{"mail list", "mail get", "mail attachments", "mail draft", "mail mark-read", "mail archive", "mail delete"}},
	{Name: "mail-send", Description: "Send mail", Commands: []string{"mail send"}},
	{Name: "calendar-read", Description: "Read calendar events", Commands: []string{"cal list"}},
	{Name: "calendar", Description: "Read and manage calendar events", Commands: []string{"cal list", "cal create", "cal delete"}},
	{Name: "sync", Description: "Sync calendars and contacts", Commands: []string{"sync"}},
}

var scopeImplications = map[string][]string{
	"mail.readwrite":      {"mail.read"},
	"calendars.readwrite": {"calendars.read"},
	"contacts.readwrite":  {"contacts.read"},
	"files.readwrite":     {"files.read"},
	"files.readwrite.all": {"files.readwrite", "files.read.all", "files.read"},
}

func Commands() []Command { return append([]Command(nil), commands...) }
func Features() []Feature { return append([]Feature(nil), features...) }

// Resolve expands command selectors and feature names into minimal scopes.
// Selectors accept exact command names and group wildcards such as "mail *".
func Resolve(selectors, featureNames []string) (Plan, error) {
	selected := map[string]Command{}
	resolvedFeatures := make([]string, 0, len(featureNames))

	for _, name := range splitValues(featureNames) {
		feature, ok := findFeature(name)
		if !ok {
			return Plan{}, fmt.Errorf("unknown feature %q (available: %s)", name, featureList())
		}
		resolvedFeatures = append(resolvedFeatures, feature.Name)
		for _, commandName := range feature.Commands {
			command, _ := findCommand(commandName)
			selected[command.Name] = command
		}
	}

	for _, selector := range splitValues(selectors) {
		matched := false
		if strings.HasSuffix(selector, " *") {
			prefix := strings.TrimSpace(strings.TrimSuffix(selector, "*")) + " "
			for _, command := range commands {
				if strings.HasPrefix(command.Name, prefix) {
					selected[command.Name] = command
					matched = true
				}
			}
		} else if command, ok := findCommand(selector); ok {
			selected[command.Name] = command
			matched = true
		}
		if !matched {
			return Plan{}, fmt.Errorf("unknown command selector %q", selector)
		}
	}

	if len(selected) == 0 {
		return Plan{}, fmt.Errorf("select at least one --command or --feature")
	}

	plan := Plan{Features: uniqueSorted(resolvedFeatures)}
	for _, command := range selected {
		plan.Commands = append(plan.Commands, command)
	}
	sort.Slice(plan.Commands, func(i, j int) bool { return plan.Commands[i].Name < plan.Commands[j].Name })

	scopes := []string{"offline_access", "User.Read"}
	for _, command := range plan.Commands {
		scopes = append(scopes, command.Scopes...)
	}
	plan.Scopes = MinimalScopes(scopes)
	return plan, nil
}

// Allowed reports whether granted scopes satisfy every scope required by command.
func Allowed(command Command, granted []string) bool {
	for _, required := range command.Scopes {
		if !scopeCovered(required, granted) {
			return false
		}
	}
	return true
}

// MinimalScopes deduplicates scopes and removes read scopes superseded by read/write scopes.
func MinimalScopes(scopes []string) []string {
	byNormalized := map[string]string{}
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope != "" {
			byNormalized[strings.ToLower(scope)] = scope
		}
	}
	for granted := range byNormalized {
		for _, implied := range impliedScopes(granted) {
			delete(byNormalized, implied)
		}
	}
	result := make([]string, 0, len(byNormalized))
	for _, scope := range byNormalized {
		result = append(result, scope)
	}
	sort.Strings(result)
	return result
}

func scopeCovered(required string, granted []string) bool {
	required = strings.ToLower(required)
	for _, scope := range granted {
		normalized := normalizeGrantedScope(scope)
		if normalized == required {
			return true
		}
		for _, implied := range impliedScopes(normalized) {
			if implied == required {
				return true
			}
		}
	}
	return false
}

func normalizeGrantedScope(scope string) string {
	normalized := strings.ToLower(strings.TrimSpace(scope))
	return strings.TrimPrefix(normalized, "https://graph.microsoft.com/")
}

func impliedScopes(scope string) []string {
	seen := map[string]bool{}
	var visit func(string)
	visit = func(current string) {
		for _, implied := range scopeImplications[current] {
			if !seen[implied] {
				seen[implied] = true
				visit(implied)
			}
		}
	}
	visit(scope)
	result := make([]string, 0, len(seen))
	for scope := range seen {
		result = append(result, scope)
	}
	return result
}

func findCommand(name string) (Command, bool) {
	name = strings.ToLower(strings.Join(strings.Fields(name), " "))
	for _, command := range commands {
		if command.Name == name {
			return command, true
		}
	}
	return Command{}, false
}

func findFeature(name string) (Feature, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, feature := range features {
		if feature.Name == name {
			return feature, true
		}
	}
	return Feature{}, false
}

func splitValues(values []string) []string {
	var result []string
	for _, value := range values {
		for _, item := range strings.Split(value, ",") {
			if item = strings.TrimSpace(item); item != "" {
				result = append(result, item)
			}
		}
	}
	return result
}

func uniqueSorted(values []string) []string {
	seen := map[string]bool{}
	for _, value := range values {
		seen[value] = true
	}
	result := make([]string, 0, len(seen))
	for value := range seen {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func featureList() string {
	var names []string
	for _, feature := range features {
		names = append(names, feature.Name)
	}
	return strings.Join(names, ", ")
}
