package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// SessionInfo holds parsed session data from the plugin's pipe response.
type SessionInfo struct {
	PaneID      uint32   `json:"pane_id"`
	DisplayName string   `json:"display_name"`
	Activity    string   `json:"activity"`
	WorkingDir  string   `json:"working_dir"`
	AgentName   string   `json:"agent_name"`
	Topic       string   `json:"topic"`
	RecentTools []string `json:"recent_tools"`
	Paused      bool     `json:"paused"`
	Badges      []string `json:"badges"`
}

// MCPSession is the enriched session object returned by the cc_deck_sessions tool.
type MCPSession struct {
	Name           string   `json:"name"`
	PaneID         uint32   `json:"pane_id"`
	Agent          string   `json:"agent"`
	State          string   `json:"state"`
	Cwd            string   `json:"cwd"`
	Branch         string   `json:"branch,omitempty"`
	Topic          string   `json:"topic,omitempty"`
	ProjectSummary string   `json:"project_summary,omitempty"`
	RecentTools    []string `json:"recent_tools,omitempty"`
	ModifiedFiles  []string `json:"modified_files,omitempty"`
	Paused         bool     `json:"paused"`
}

// SessionState is the detailed state returned by the cc_deck_session_state tool.
type SessionState struct {
	Name             string          `json:"name"`
	PaneID           uint32          `json:"pane_id"`
	Cwd              string          `json:"cwd"`
	Branch           string          `json:"branch,omitempty"`
	GitDiff          string          `json:"git_diff,omitempty"`
	ModifiedFiles    []string        `json:"modified_files,omitempty"`
	RecentCommits    []string        `json:"recent_commits,omitempty"`
	RecentTools      []string        `json:"recent_tools,omitempty"`
	Topic            string          `json:"topic,omitempty"`
	ActivityTimeline json.RawMessage `json:"activity_timeline,omitempty"`
}

// pipeTimeout is the default timeout for pipe communication with the plugin.
const pipeTimeout = 5 * time.Second

// pipeSend is the pipe sender function, replaceable for testing.
var pipeSend PipeSender = SendPipe

// fetchSessions queries the plugin for the current session list.
func fetchSessions(ctx context.Context) ([]SessionInfo, error) {
	pipeCtx, cancel := context.WithTimeout(ctx, pipeTimeout)
	defer cancel()

	resp, err := pipeSend(pipeCtx, "cc-deck:mcp-sessions", "")
	if err != nil {
		return nil, err
	}
	if resp == "" {
		return nil, fmt.Errorf("cc-deck plugin not running")
	}

	var sessions []SessionInfo
	if err := json.Unmarshal([]byte(resp), &sessions); err != nil {
		return nil, fmt.Errorf("parsing session list: %w", err)
	}
	return sessions, nil
}

// resolveSession finds a session by display name or pane ID string.
// Returns the pane ID if exactly one match is found.
func resolveSession(sessions []SessionInfo, nameOrID string) (uint32, error) {
	// Try numeric pane ID first.
	if id, err := strconv.ParseUint(nameOrID, 10, 32); err == nil {
		paneID := uint32(id)
		for _, s := range sessions {
			if s.PaneID == paneID {
				return paneID, nil
			}
		}
		return 0, fmt.Errorf("session not found: %s", nameOrID)
	}

	// Match by display name.
	var matches []SessionInfo
	for _, s := range sessions {
		if s.DisplayName == nameOrID {
			matches = append(matches, s)
		}
	}

	switch len(matches) {
	case 0:
		return 0, fmt.Errorf("session not found: %s", nameOrID)
	case 1:
		return matches[0].PaneID, nil
	default:
		parts := make([]string, len(matches))
		for i, m := range matches {
			parts[i] = fmt.Sprintf("pane %d: '%s'", m.PaneID, m.DisplayName)
		}
		return 0, fmt.Errorf("ambiguous session name '%s', matches: [%s]",
			nameOrID, joinStrings(parts, ", "))
	}
}

// toolError creates an MCP tool result with an error message in the format
// expected by the contracts.
func toolError(msg string) (*mcp.CallToolResult, error) {
	errJSON, _ := json.Marshal(map[string]string{"error": msg})
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: string(errJSON)},
		},
		IsError: true,
	}, nil
}

// toolText creates an MCP tool result with plain text content.
func toolText(text string) (*mcp.CallToolResult, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: text},
		},
	}, nil
}

// handleSessions implements the cc_deck_sessions tool.
func handleSessions(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sessions, err := fetchSessions(ctx)
	if err != nil {
		return toolError("cc-deck plugin not running")
	}

	mcpSessions := make([]MCPSession, len(sessions))
	for i, s := range sessions {
		ms := MCPSession{
			Name:        s.DisplayName,
			PaneID:      s.PaneID,
			Agent:       s.AgentName,
			State:       s.Activity,
			Cwd:         s.WorkingDir,
			Topic:       s.Topic,
			RecentTools: s.RecentTools,
			Paused:      s.Paused,
		}

		if s.WorkingDir != "" {
			ms.Branch = ReadGitBranch(s.WorkingDir)
			ms.ModifiedFiles = ReadModifiedFiles(s.WorkingDir)
			ms.ProjectSummary = ReadProjectSummary(s.WorkingDir)
		}

		mcpSessions[i] = ms
	}

	data, err := json.Marshal(mcpSessions)
	if err != nil {
		return toolError(fmt.Sprintf("encoding sessions: %v", err))
	}
	return toolText(string(data))
}

// handleReadScrollback implements the cc_deck_read_scrollback tool.
func handleReadScrollback(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sessionName := request.GetString("session", "")
	if sessionName == "" {
		return toolError("session parameter is required")
	}

	lines := request.GetInt("lines", 50)
	if lines < 1 || lines > 500 {
		return toolError("lines must be between 1 and 500")
	}

	sessions, err := fetchSessions(ctx)
	if err != nil {
		return toolError("cc-deck plugin not running")
	}

	paneID, err := resolveSession(sessions, sessionName)
	if err != nil {
		return toolError(err.Error())
	}

	payload, _ := json.Marshal(map[string]any{
		"pane_id": paneID,
		"lines":   lines,
	})

	pipeCtx, cancel := context.WithTimeout(ctx, pipeTimeout)
	defer cancel()

	resp, err := pipeSend(pipeCtx, "cc-deck:mcp-scrollback", string(payload))
	if err != nil {
		return toolError("cc-deck plugin not running")
	}

	return toolText(resp)
}

// handleSessionState implements the cc_deck_session_state tool.
func handleSessionState(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sessionName := request.GetString("session", "")
	if sessionName == "" {
		return toolError("session parameter is required")
	}

	sessions, err := fetchSessions(ctx)
	if err != nil {
		return toolError("cc-deck plugin not running")
	}

	paneID, err := resolveSession(sessions, sessionName)
	if err != nil {
		return toolError(err.Error())
	}

	payload, _ := json.Marshal(map[string]any{"pane_id": paneID})

	pipeCtx, cancel := context.WithTimeout(ctx, pipeTimeout)
	defer cancel()

	resp, err := pipeSend(pipeCtx, "cc-deck:mcp-state", string(payload))
	if err != nil {
		return toolError("cc-deck plugin not running")
	}

	// Parse the plugin response to get working_dir and other fields.
	var pluginState struct {
		DisplayName      string          `json:"display_name"`
		PaneID           uint32          `json:"pane_id"`
		WorkingDir       string          `json:"working_dir"`
		RecentTools      []string        `json:"recent_tools"`
		Topic            string          `json:"topic"`
		ActivityTimeline json.RawMessage `json:"activity_timeline"`
	}
	if err := json.Unmarshal([]byte(resp), &pluginState); err != nil {
		return toolError(fmt.Sprintf("parsing session state: %v", err))
	}

	state := SessionState{
		Name:             pluginState.DisplayName,
		PaneID:           pluginState.PaneID,
		Cwd:              pluginState.WorkingDir,
		RecentTools:      pluginState.RecentTools,
		Topic:            pluginState.Topic,
		ActivityTimeline: pluginState.ActivityTimeline,
	}

	if pluginState.WorkingDir != "" {
		state.Branch = ReadGitBranch(pluginState.WorkingDir)
		state.GitDiff = ReadGitDiffStat(pluginState.WorkingDir)
		state.ModifiedFiles = ReadModifiedFiles(pluginState.WorkingDir)
		state.RecentCommits = ReadRecentCommits(pluginState.WorkingDir, 5)
	}

	data, err := json.Marshal(state)
	if err != nil {
		return toolError(fmt.Sprintf("encoding session state: %v", err))
	}
	return toolText(string(data))
}

// handleAsk implements the cc_deck_ask tool.
func handleAsk(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sessionName := request.GetString("session", "")
	if sessionName == "" {
		return toolError("session parameter is required")
	}

	question := request.GetString("question", "")
	if question == "" {
		return toolError("question parameter is required")
	}

	timeout := request.GetInt("timeout", 120)
	if timeout < 10 || timeout > 600 {
		return toolError("timeout must be between 10 and 600")
	}

	sessions, err := fetchSessions(ctx)
	if err != nil {
		return toolError("cc-deck plugin not running")
	}

	paneID, err := resolveSession(sessions, sessionName)
	if err != nil {
		return toolError(err.Error())
	}

	// Find the session to get its display name for error messages.
	var displayName string
	for _, s := range sessions {
		if s.PaneID == paneID {
			displayName = s.DisplayName
			break
		}
	}

	resp, err := AskSession(ctx, pipeSend, paneID, displayName, question, timeout)
	if err != nil {
		return toolError(err.Error())
	}

	return toolText(resp)
}

// joinStrings concatenates strings with a separator.
func joinStrings(parts []string, sep string) string {
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += sep
		}
		result += p
	}
	return result
}
