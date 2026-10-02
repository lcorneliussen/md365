package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/lcorneliussen/md365/internal/capability"
	"github.com/lcorneliussen/md365/internal/commandmeta"
	"github.com/lcorneliussen/md365/internal/output"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const commandSchemaVersion = "1.0.0"

type schemaDocument struct {
	SchemaVersion string             `json:"schema_version"`
	OutputModes   []schemaOutputMode `json:"output_modes"`
	ExitStatuses  []schemaExitStatus `json:"exit_statuses"`
	Commands      []schemaCommand    `json:"commands"`
}

type schemaCommand struct {
	Path                 string                 `json:"path"`
	Use                  string                 `json:"use"`
	Short                string                 `json:"short,omitempty"`
	Aliases              []string               `json:"aliases,omitempty"`
	Arguments            []schemaArgument       `json:"arguments,omitempty"`
	Flags                []schemaFlag           `json:"flags,omitempty"`
	Subcommands          []string               `json:"subcommands,omitempty"`
	DelegatedPermissions []string               `json:"delegated_permissions,omitempty"`
	FeatureBundles       []string               `json:"feature_bundles,omitempty"`
	Mutability           commandmeta.Mutability `json:"mutability,omitempty"`
	CanPrompt            bool                   `json:"can_prompt"`
	Effects              []string               `json:"effects,omitempty"`
}

type schemaArgument struct {
	Name     string `json:"name"`
	Required bool   `json:"required"`
	Variadic bool   `json:"variadic,omitempty"`
}

type schemaFlag struct {
	Name      string `json:"name"`
	Shorthand string `json:"shorthand,omitempty"`
	Type      string `json:"type"`
	Usage     string `json:"usage,omitempty"`
	Default   string `json:"default,omitempty"`
	Required  bool   `json:"required"`
	Inherited bool   `json:"inherited,omitempty"`
}

type schemaOutputMode struct {
	Name        string `json:"name"`
	Flag        string `json:"flag,omitempty"`
	Description string `json:"description"`
}

type schemaExitStatus struct {
	Code   string `json:"code"`
	Status int    `json:"status"`
}

var schemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "Print the versioned md365 command contract",
	Long:  "Print deterministic command, permission, safety, output, and exit-status metadata for automation and agents.",
	RunE: func(cmd *cobra.Command, args []string) error {
		document, err := buildSchema(rootCmd)
		if err != nil {
			return err
		}
		if !writer.IsHuman() {
			return writeOK(document,
				output.WithSummary("md365 command schema "+commandSchemaVersion),
				output.WithMeta("schema_version", commandSchemaVersion),
			)
		}
		encoder := json.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent("", "  ")
		encoder.SetEscapeHTML(false)
		return encoder.Encode(document)
	},
}

func buildSchema(root *cobra.Command) (schemaDocument, error) {
	root.InitDefaultHelpCmd()
	root.InitDefaultHelpFlag()
	document := schemaDocument{
		SchemaVersion: commandSchemaVersion,
		OutputModes: []schemaOutputMode{
			{Name: "human", Description: "Human-readable output (default)"},
			{Name: "json", Flag: "--json", Description: "Stable JSON response envelope"},
			{Name: "quiet", Flag: "--quiet", Description: "Result data only"},
			{Name: "ids", Flag: "--ids-only", Description: "Result IDs, one per line"},
			{Name: "count", Flag: "--count", Description: "Result count only"},
		},
		ExitStatuses: []schemaExitStatus{
			{Code: "ok", Status: 0},
			{Code: "usage", Status: 1},
			{Code: "unknown", Status: 1},
			{Code: "not_found", Status: 2},
			{Code: "auth", Status: 3},
			{Code: "forbidden", Status: 4},
			{Code: "rate_limit", Status: 5},
			{Code: "network", Status: 6},
			{Code: "graph", Status: 7},
		},
	}

	var walk func(*cobra.Command) error
	walk = func(command *cobra.Command) error {
		if command.Hidden {
			return nil
		}
		entry, err := schemaEntry(command)
		if err != nil {
			return err
		}
		document.Commands = append(document.Commands, entry)
		for _, child := range command.Commands() {
			if err := walk(child); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(root); err != nil {
		return schemaDocument{}, err
	}
	sort.Slice(document.Commands, func(i, j int) bool { return document.Commands[i].Path < document.Commands[j].Path })
	return document, nil
}

func schemaEntry(command *cobra.Command) (schemaCommand, error) {
	flags := schemaFlags(command)
	entry := schemaCommand{
		Path:      command.CommandPath(),
		Use:       strings.TrimSpace(command.UseLine()),
		Short:     command.Short,
		Aliases:   append([]string(nil), command.Aliases...),
		Arguments: parseSchemaArguments(command.Use),
		Flags:     flags,
	}
	sort.Strings(entry.Aliases)
	for _, child := range command.Commands() {
		if !child.Hidden {
			entry.Subcommands = append(entry.Subcommands, child.Name())
		}
	}
	sort.Strings(entry.Subcommands)

	shortPath := strings.TrimSpace(strings.TrimPrefix(entry.Path, rootCmd.Name()))
	if graphCommand, ok := capability.CommandByName(shortPath); ok {
		entry.DelegatedPermissions = append([]string(nil), graphCommand.Scopes...)
		sort.Strings(entry.DelegatedPermissions)
		entry.FeatureBundles = capability.FeaturesForCommand(shortPath)
	}
	if policy, ok := commandmeta.Lookup(shortPath); ok {
		entry.Mutability = policy.Mutability
		entry.CanPrompt = policy.CanPrompt
		entry.Effects = append([]string(nil), policy.Effects...)
		sort.Strings(entry.Effects)
	} else if command.Runnable() && !isFrameworkCommand(shortPath) {
		return schemaCommand{}, fmt.Errorf("command %q has no execution policy", entry.Path)
	}
	return entry, nil
}

func schemaFlags(command *cobra.Command) []schemaFlag {
	var result []schemaFlag
	seen := map[string]bool{}
	add := func(flags *pflag.FlagSet, inherited bool) {
		flags.VisitAll(func(flag *pflag.Flag) {
			if flag.Hidden || seen[flag.Name] {
				return
			}
			seen[flag.Name] = true
			result = append(result, schemaFlag{
				Name: flag.Name, Shorthand: flag.Shorthand, Type: flag.Value.Type(),
				Usage: flag.Usage, Default: flag.DefValue,
				Required:  strings.Contains(strings.ToLower(flag.Usage), "(required)"),
				Inherited: inherited,
			})
		})
	}
	add(command.InheritedFlags(), true)
	add(command.NonInheritedFlags(), false)
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func parseSchemaArguments(use string) []schemaArgument {
	fields := strings.Fields(use)
	if len(fields) < 2 {
		return nil
	}
	var result []schemaArgument
	for _, field := range fields[1:] {
		if field == "[flags]" {
			continue
		}
		optional := strings.HasPrefix(field, "[") && strings.HasSuffix(field, "]")
		name := strings.Trim(field, "[]")
		variadic := strings.HasSuffix(name, "...")
		name = strings.TrimSuffix(name, "...")
		result = append(result, schemaArgument{Name: name, Required: !optional, Variadic: variadic})
	}
	return result
}

func isFrameworkCommand(path string) bool {
	return path == "help" || strings.HasPrefix(path, "completion")
}
