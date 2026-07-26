package cmd

import (
	"github.com/spf13/cobra"

	"github.com/cc-deck/cc-deck/internal/mcp"
)

// NewMcpCmd creates the mcp parent command with subcommands.
func NewMcpCmd() *cobra.Command {
	mcpCmd := &cobra.Command{
		Use:   "mcp",
		Short: "MCP server for cross-pane agent communication",
		Long:  "Run the cc-deck MCP server that exposes tools for cross-pane agent communication.",
	}

	mcpCmd.AddCommand(newMcpServeCmd())

	return mcpCmd
}

func newMcpServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the MCP server on stdio",
		Long: `Start the cc-deck MCP server using stdio transport. This command is
designed to be launched by an AI agent's MCP configuration. It communicates
with the cc-deck Zellij plugin via pipe messages to provide session listing,
scrollback reading, structured state queries, and cross-pane ask/response.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return mcp.Serve()
		},
	}
}
