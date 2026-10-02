package commandmeta

import "strings"

type Mutability string

const (
	Read  Mutability = "read"
	Write Mutability = "write"
)

type Policy struct {
	Mutability Mutability `json:"mutability"`
	CanPrompt  bool       `json:"can_prompt"`
	Effects    []string   `json:"effects,omitempty"`
}

var policies = map[string]Policy{
	"about":                {Mutability: Read},
	"commands":             {Mutability: Read},
	"schema":               {Mutability: Read},
	"skill":                {Mutability: Read},
	"skill install":        {Mutability: Write, Effects: []string{"local_write"}},
	"auth add":             {Mutability: Write, CanPrompt: true, Effects: []string{"local_write", "authentication"}},
	"auth explain":         {Mutability: Read, Effects: []string{"keyring_read"}},
	"auth login":           {Mutability: Write, CanPrompt: true, Effects: []string{"authentication", "browser", "keyring_write"}},
	"auth plan":            {Mutability: Read},
	"auth refresh":         {Mutability: Write, Effects: []string{"authentication", "keyring_write"}},
	"auth scopes":          {Mutability: Read, Effects: []string{"keyring_read"}},
	"auth status":          {Mutability: Read, Effects: []string{"keyring_read"}},
	"sync":                 {Mutability: Write, Effects: []string{"microsoft_graph_read", "local_write"}},
	"cal list":             {Mutability: Read, Effects: []string{"local_read", "microsoft_graph_read"}},
	"cal create":           {Mutability: Write, Effects: []string{"microsoft_graph_write"}},
	"cal delete":           {Mutability: Write, Effects: []string{"microsoft_graph_write"}},
	"contacts search":      {Mutability: Read, Effects: []string{"local_read", "microsoft_graph_read"}},
	"mail list":            {Mutability: Read, Effects: []string{"microsoft_graph_read"}},
	"mail search":          {Mutability: Read, Effects: []string{"microsoft_graph_read", "microsoft_search"}},
	"mail get":             {Mutability: Read, Effects: []string{"microsoft_graph_read"}},
	"mail attachments":     {Mutability: Read, Effects: []string{"microsoft_graph_read"}},
	"mail send":            {Mutability: Write, Effects: []string{"microsoft_graph_write", "external_communication"}},
	"mail draft":           {Mutability: Write, Effects: []string{"microsoft_graph_write"}},
	"mail mark-read":       {Mutability: Write, Effects: []string{"microsoft_graph_write"}},
	"mail archive":         {Mutability: Write, Effects: []string{"microsoft_graph_write"}},
	"mail delete":          {Mutability: Write, Effects: []string{"microsoft_graph_write"}},
	"teams list":           {Mutability: Read, Effects: []string{"microsoft_graph_read"}},
	"teams channels":       {Mutability: Read, Effects: []string{"microsoft_graph_read"}},
	"teams files":          {Mutability: Read, Effects: []string{"microsoft_graph_read"}},
	"onedrive list":        {Mutability: Read, Effects: []string{"microsoft_graph_read"}},
	"sharepoint libraries": {Mutability: Read, Effects: []string{"microsoft_graph_read"}},
	"sharepoint list":      {Mutability: Read, Effects: []string{"microsoft_graph_read"}},
	"files search":         {Mutability: Read, Effects: []string{"microsoft_graph_read", "microsoft_search"}},
}

func Lookup(path string) (Policy, bool) {
	path = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(path), "md365"))
	policy, ok := policies[path]
	return policy, ok
}

func All() map[string]Policy {
	result := make(map[string]Policy, len(policies))
	for path, policy := range policies {
		result[path] = policy
	}
	return result
}
