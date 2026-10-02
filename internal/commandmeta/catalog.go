package commandmeta

import "strings"

type Mutability string

const (
	Read  Mutability = "read"
	Write Mutability = "write"
)

type Policy struct {
	Mutability      Mutability   `json:"mutability"`
	CanPrompt       bool         `json:"can_prompt"`
	Prompting       Prompting    `json:"prompting,omitempty"`
	DryRunSupported bool         `json:"dry_run_supported"`
	Effects         []string     `json:"effects,omitempty"`
	OutputModes     []string     `json:"output_modes"`
	Constraints     []Constraint `json:"constraints,omitempty"`
}

type Prompting struct {
	Mode         string   `json:"mode,omitempty"`
	WhenAnyFlags []string `json:"when_any_flags,omitempty"`
}

type FlagRequirement struct {
	Required bool
	Unless   []string
}

type Constraint struct {
	Kind        string        `json:"kind"`
	Description string        `json:"description"`
	Options     []InputOption `json:"options"`
}

type InputOption struct {
	Arguments []string `json:"arguments,omitempty"`
	Flags     []string `json:"flags,omitempty"`
}

var policies = map[string]Policy{
	"about":                 {Mutability: Read},
	"commands":              {Mutability: Read},
	"schema":                {Mutability: Read},
	"help":                  {Mutability: Read, OutputModes: []string{"human"}},
	"completion":            {Mutability: Read, OutputModes: []string{"human"}},
	"completion bash":       {Mutability: Read, OutputModes: []string{"human"}},
	"completion fish":       {Mutability: Read, OutputModes: []string{"human"}},
	"completion powershell": {Mutability: Read, OutputModes: []string{"human"}},
	"completion zsh":        {Mutability: Read, OutputModes: []string{"human"}},
	"skill":                 {Mutability: Read, OutputModes: []string{"human"}},
	"skill install":         {Mutability: Write, Effects: []string{"local_write"}},
	"auth add":              {Mutability: Write, CanPrompt: true, Prompting: Prompting{Mode: "conditional", WhenAnyFlags: []string{"interactive", "login"}}, Effects: []string{"local_write", "authentication", "browser", "keyring_write"}, OutputModes: []string{"human"}},
	"auth explain":          {Mutability: Read, Effects: []string{"keyring_read"}},
	"auth login":            {Mutability: Write, CanPrompt: true, Prompting: Prompting{Mode: "always"}, Effects: []string{"authentication", "browser", "keyring_write"}, OutputModes: []string{"human"}},
	"auth plan":             {Mutability: Read},
	"auth refresh":          {Mutability: Write, Effects: []string{"authentication", "keyring_write"}},
	"auth scopes":           {Mutability: Read, Effects: []string{"keyring_read"}},
	"auth status":           {Mutability: Read, Effects: []string{"keyring_read"}},
	"sync":                  {Mutability: Write, Effects: []string{"microsoft_graph_read", "local_write"}},
	"cal list":              {Mutability: Read, Effects: []string{"local_read", "microsoft_graph_read"}},
	"cal create":            {Mutability: Write, DryRunSupported: true, Effects: []string{"microsoft_graph_write", "local_write", "external_communication"}},
	"cal delete":            {Mutability: Write, DryRunSupported: true, Effects: []string{"microsoft_graph_write", "local_read", "local_write", "external_communication"}},
	"contacts search":       {Mutability: Read, Effects: []string{"local_read", "microsoft_graph_read"}},
	"mail list":             {Mutability: Read, Effects: []string{"microsoft_graph_read"}},
	"mail search":           {Mutability: Read, Effects: []string{"microsoft_graph_read", "microsoft_search"}},
	"mail get":              {Mutability: Read, Effects: []string{"microsoft_graph_read"}},
	"mail attachments":      {Mutability: Read, Effects: []string{"microsoft_graph_read"}},
	"mail send":             {Mutability: Write, DryRunSupported: true, Effects: []string{"microsoft_graph_write", "external_communication"}},
	"mail draft":            {Mutability: Write, DryRunSupported: true, Effects: []string{"microsoft_graph_write"}},
	"mail mark-read":        {Mutability: Write, DryRunSupported: true, Effects: []string{"microsoft_graph_write"}},
	"mail archive":          {Mutability: Write, DryRunSupported: true, Effects: []string{"microsoft_graph_write"}},
	"mail delete":           {Mutability: Write, DryRunSupported: true, Effects: []string{"microsoft_graph_write"}},
	"teams list":            {Mutability: Read, Effects: []string{"microsoft_graph_read"}},
	"teams channels":        {Mutability: Read, Effects: []string{"microsoft_graph_read"}},
	"teams files":           {Mutability: Read, Effects: []string{"microsoft_graph_read"}},
	"onedrive list":         {Mutability: Read, Effects: []string{"microsoft_graph_read"}},
	"sharepoint libraries":  {Mutability: Read, Effects: []string{"microsoft_graph_read"}},
	"sharepoint list":       {Mutability: Read, Effects: []string{"microsoft_graph_read"}},
	"files search":          {Mutability: Read, Effects: []string{"microsoft_graph_read", "microsoft_search"}},
}

var collectionOutputCommands = map[string]bool{
	"cal list": true, "contacts search": true, "mail list": true,
	"mail search": true, "mail attachments": true, "teams list": true,
	"teams channels": true, "teams files": true, "onedrive list": true,
	"sharepoint libraries": true, "sharepoint list": true, "files search": true,
}

var constraints = map[string][]Constraint{
	"auth add": {
		{Kind: "mutually_exclusive", Description: "Raw scopes cannot be combined with command selection", Options: []InputOption{{Flags: []string{"scopes"}}, {Flags: []string{"command"}}}},
		{Kind: "mutually_exclusive", Description: "Raw scopes cannot be combined with feature selection", Options: []InputOption{{Flags: []string{"scopes"}}, {Flags: []string{"feature"}}}},
	},
	"auth login": {
		{Kind: "mutually_exclusive", Description: "A scope override cannot be combined with command selection", Options: []InputOption{{Flags: []string{"scope"}}, {Flags: []string{"command"}}}},
		{Kind: "mutually_exclusive", Description: "A scope override cannot be combined with feature selection", Options: []InputOption{{Flags: []string{"scope"}}, {Flags: []string{"feature"}}}},
	},
	"auth plan":            {{Kind: "at_least_one", Description: "Select at least one command or feature bundle", Options: []InputOption{{Flags: []string{"command"}}, {Flags: []string{"feature"}}}}},
	"cal delete":           {{Kind: "at_least_one", Description: "Identify the event by cached file or by account and event ID", Options: []InputOption{{Arguments: []string{"file"}}, {Flags: []string{"account", "id"}}}}},
	"mail mark-read":       {{Kind: "at_least_one", Description: "Provide message IDs as positional arguments or --id", Options: []InputOption{{Arguments: []string{"MESSAGE_ID"}}, {Flags: []string{"id"}}}}},
	"mail archive":         {{Kind: "at_least_one", Description: "Provide message IDs as positional arguments or --id", Options: []InputOption{{Arguments: []string{"MESSAGE_ID"}}, {Flags: []string{"id"}}}}},
	"mail delete":          {{Kind: "at_least_one", Description: "Provide message IDs as positional arguments or --id", Options: []InputOption{{Arguments: []string{"MESSAGE_ID"}}, {Flags: []string{"id"}}}}},
	"onedrive list":        {{Kind: "mutually_exclusive", Description: "Browse by path or folder item ID", Options: []InputOption{{Flags: []string{"path"}}, {Flags: []string{"item-id"}}}}},
	"sharepoint libraries": {{Kind: "exactly_one", Description: "Select a Microsoft 365 group/team or SharePoint site", Options: []InputOption{{Flags: []string{"team-id"}}, {Flags: []string{"site-id"}}}}},
	"sharepoint list": {
		{Kind: "exactly_one", Description: "Select a Microsoft 365 group/team, SharePoint site, or document library", Options: []InputOption{{Flags: []string{"team-id"}}, {Flags: []string{"site-id"}}, {Flags: []string{"drive-id"}}}},
		{Kind: "mutually_exclusive", Description: "Browse by path or folder item ID", Options: []InputOption{{Flags: []string{"path"}}, {Flags: []string{"item-id"}}}},
	},
}

var flagRequirements = map[string]map[string]FlagRequirement{
	"auth add":             {"name": {Required: true, Unless: []string{"interactive"}}},
	"auth explain":         {"account": {Required: true}},
	"auth login":           {"account": {Required: true}},
	"auth refresh":         {"account": {Required: true}},
	"auth scopes":          {"account": {Required: true}},
	"cal create":           {"account": {Required: true}, "end": {Required: true}, "start": {Required: true}, "subject": {Required: true}},
	"files search":         {"account": {Required: true}},
	"mail archive":         {"account": {Required: true}},
	"mail attachments":     {"account": {Required: true}, "id": {Required: true}},
	"mail delete":          {"account": {Required: true}},
	"mail draft":           {"account": {Required: true}, "subject": {Required: true}, "to": {Required: true}},
	"mail get":             {"account": {Required: true}, "id": {Required: true}},
	"mail list":            {"account": {Required: true}},
	"mail mark-read":       {"account": {Required: true}},
	"mail search":          {"account": {Required: true}},
	"mail send":            {"account": {Required: true}, "subject": {Required: true}, "to": {Required: true}},
	"onedrive list":        {"account": {Required: true}},
	"sharepoint libraries": {"account": {Required: true}},
	"sharepoint list":      {"account": {Required: true}},
	"teams channels":       {"account": {Required: true}, "team-id": {Required: true}},
	"teams files":          {"account": {Required: true}, "channel-id": {Required: true}, "team-id": {Required: true}},
	"teams list":           {"account": {Required: true}},
}

func Lookup(path string) (Policy, bool) {
	path = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(path), "md365"))
	policy, ok := policies[path]
	if ok {
		if len(policy.OutputModes) == 0 {
			policy.OutputModes = []string{"human", "json", "quiet"}
		}
		if collectionOutputCommands[path] {
			policy.OutputModes = append(policy.OutputModes, "ids", "count")
		}
		policy.Constraints = append([]Constraint(nil), constraints[path]...)
	}
	return policy, ok
}

func All() map[string]Policy {
	result := make(map[string]Policy, len(policies))
	for path := range policies {
		policy, _ := Lookup(path)
		result[path] = policy
	}
	return result
}

func Requirement(path, flag string) (FlagRequirement, bool) {
	path = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(path), "md365"))
	requirements, ok := flagRequirements[path]
	if !ok {
		return FlagRequirement{}, false
	}
	requirement, ok := requirements[flag]
	return requirement, ok
}
