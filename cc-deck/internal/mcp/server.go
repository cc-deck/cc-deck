// Package mcp implements the cc-deck MCP server that exposes cross-pane
// agent communication tools via the Model Context Protocol.
package mcp

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// NewServer creates a configured MCP server with all cc-deck tools registered.
func NewServer() *server.MCPServer {
	s := server.NewMCPServer(
		"cc-deck",
		"1.0.0",
	)

	s.AddTool(
		mcp.NewTool("cc_deck_sessions",
			mcp.WithDescription("List all active agent sessions with metadata including branch, modified files, and project context"),
		),
		handleSessions,
	)

	s.AddTool(
		mcp.NewTool("cc_deck_read_scrollback",
			mcp.WithDescription("Read terminal scrollback from a target session's pane"),
			mcp.WithString("session",
				mcp.Required(),
				mcp.Description("Session display name or pane ID"),
			),
			mcp.WithInteger("lines",
				mcp.Description("Number of lines to read (1-500, default 50)"),
			),
		),
		handleReadScrollback,
	)

	s.AddTool(
		mcp.NewTool("cc_deck_session_state",
			mcp.WithDescription("Return structured state for a single session including git info and activity timeline"),
			mcp.WithString("session",
				mcp.Required(),
				mcp.Description("Session display name or pane ID"),
			),
		),
		handleSessionState,
	)

	s.AddTool(
		mcp.NewTool("cc_deck_ask",
			mcp.WithDescription("Inject a question into an idle target session and wait for a file-based response"),
			mcp.WithString("session",
				mcp.Required(),
				mcp.Description("Target session display name or pane ID"),
			),
			mcp.WithString("question",
				mcp.Required(),
				mcp.Description("Question text to ask the target agent"),
			),
			mcp.WithInteger("timeout",
				mcp.Description("Timeout in seconds (10-600, default 120)"),
			),
		),
		handleAsk,
	)

	return s
}

// Serve starts the MCP server using stdio transport, blocking until
// the connection is closed or an error occurs.
func Serve() error {
	s := NewServer()
	return server.ServeStdio(s)
}
