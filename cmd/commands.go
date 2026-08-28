package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/lcorneliussen/md365/internal/output"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type commandInfo struct {
	Path        string     `json:"path"`
	Use         string     `json:"use"`
	Short       string     `json:"short,omitempty"`
	Flags       []flagInfo `json:"flags,omitempty"`
	Subcommands []string   `json:"subcommands,omitempty"`
}

type flagInfo struct {
	Name      string `json:"name"`
	Shorthand string `json:"shorthand,omitempty"`
	Usage     string `json:"usage,omitempty"`
	Default   string `json:"default,omitempty"`
}

var commandsCmd = &cobra.Command{
	Use:   "commands",
	Short: "List the md365 command surface",
	Long:  "List md365 commands and flags in a machine-readable catalog.",
	RunE: func(cmd *cobra.Command, args []string) error {
		catalog := commandCatalog(rootCmd)
		if writer.IsHuman() {
			for _, entry := range catalog {
				fmt.Fprintln(cmd.OutOrStdout(), entry.Path)
			}
			return nil
		}
		return writeOK(catalog,
			output.WithSummary(fmt.Sprintf("%d commands", len(catalog))),
			output.WithBreadcrumbs(output.Breadcrumb{
				Action:      "about",
				Command:     "md365 about --json",
				Description: "Read md365's source model and conventions",
			}),
		)
	},
}

func commandCatalog(root *cobra.Command) []commandInfo {
	var catalog []commandInfo
	var walk func(cmd *cobra.Command)
	walk = func(cmd *cobra.Command) {
		if cmd.Hidden {
			return
		}

		info := commandInfo{
			Path:  cmd.CommandPath(),
			Use:   strings.TrimSpace(cmd.UseLine()),
			Short: cmd.Short,
			Flags: commandFlags(cmd),
		}
		for _, child := range cmd.Commands() {
			if !child.Hidden {
				info.Subcommands = append(info.Subcommands, child.Name())
			}
		}
		sort.Strings(info.Subcommands)
		catalog = append(catalog, info)

		for _, child := range cmd.Commands() {
			walk(child)
		}
	}
	walk(root)
	sort.Slice(catalog, func(i, j int) bool {
		return catalog[i].Path < catalog[j].Path
	})
	return catalog
}

func commandFlags(cmd *cobra.Command) []flagInfo {
	flags := []flagInfo{}
	seen := map[string]bool{}
	addFlags := func(set *pflag.FlagSet) {
		set.VisitAll(func(flag *pflag.Flag) {
			if flag.Hidden || seen[flag.Name] {
				return
			}
			seen[flag.Name] = true
			flags = append(flags, flagInfo{
				Name:      flag.Name,
				Shorthand: flag.Shorthand,
				Usage:     flag.Usage,
				Default:   flag.DefValue,
			})
		})
	}

	addFlags(cmd.InheritedFlags())
	addFlags(cmd.NonInheritedFlags())
	sort.Slice(flags, func(i, j int) bool {
		return flags[i].Name < flags[j].Name
	})
	return flags
}
