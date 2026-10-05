package cmd

import (
	"fmt"
	"strings"

	loopresource "github.com/lcorneliussen/md365/internal/loop"
	"github.com/lcorneliussen/md365/internal/output"
	"github.com/spf13/cobra"
)

var (
	loopAccount string
	loopLimit   int
)

var loopCmd = &cobra.Command{
	Use:   "loop",
	Short: "Microsoft Loop commands",
	Long:  "Discover Microsoft Loop components visible to Microsoft Search across OneDrive, SharePoint, and indexed SharePoint Embedded storage.",
}

var loopSearchCmd = &cobra.Command{
	Use:     "search [QUERY]",
	Aliases: []string{"list", "ls"},
	Short:   "Search Microsoft Loop components",
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if loopAccount == "" {
			return usageError("--account is required")
		}
		query := ""
		if len(args) == 1 {
			query = strings.TrimSpace(args[0])
			if query == "" {
				return usageError("search query must not be empty")
			}
		}
		requestLimit, err := lookaheadLimit(loopLimit)
		if err != nil {
			return err
		}
		values, err := loopresource.Search(cfg, loopAccount, query, requestLimit)
		if err != nil {
			return err
		}
		values, page := collectionPage(values, loopLimit)
		if writer.IsHuman() {
			printLoopComponents(cmd, values)
			return nil
		}
		return writeOK(values,
			output.WithSummary(fmt.Sprintf("%d Microsoft Loop components", len(values))),
			output.WithMeta("source", "microsoft_search"),
			output.WithMeta("workload", "microsoft_loop"),
			output.WithMeta("query", query),
			output.WithMeta("coverage", "components_visible_to_microsoft_search"),
			page,
		)
	},
}

func printLoopComponents(cmd *cobra.Command, values []loopresource.ComponentInfo) {
	for _, value := range values {
		fmt.Fprintf(cmd.OutOrStdout(), "%d. %s  [%s]\n  item: %s", value.Rank, value.Name, value.Format, value.ID)
		if value.DriveID != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "  drive: %s", value.DriveID)
		}
		if value.WebURL != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "\n  url: %s", value.WebURL)
		}
		fmt.Fprintln(cmd.OutOrStdout())
	}
	if len(values) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No Microsoft Loop components found")
	}
}

func init() {
	loopSearchCmd.Flags().StringVar(&loopAccount, "account", "", "Account (required)")
	loopSearchCmd.Flags().IntVar(&loopLimit, "limit", 25, "Maximum Loop components")
	loopCmd.AddCommand(loopSearchCmd)
}
