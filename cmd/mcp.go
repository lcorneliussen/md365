package cmd

import (
	"github.com/lcorneliussen/md365/internal/mcpserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Expose typed Microsoft 365 tools through MCP",
}

var mcpServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Serve fixed read-only Microsoft 365 tools over stdio",
	Long: `Serve a fixed set of typed, read-only Microsoft 365 tools over the
Model Context Protocol (MCP) stdio transport. The server never starts an
interactive sign-in and exposes no generic shell or Microsoft Graph request.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		server, err := mcpserver.NewProduction(cfg)
		if err != nil {
			return err
		}
		return server.Run(cmd.Context(), &mcp.StdioTransport{})
	},
}

func init() {
	mcpCmd.AddCommand(mcpServeCmd)
}
