package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Context function tests (T017) ---

func TestReadProjectSummary(t *testing.T) {
	t.Run("returns first 500 chars of CLAUDE.md", func(t *testing.T) {
		dir := t.TempDir()
		content := "# Project\nThis is a test project with lots of content."
		require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(content), 0644))

		result := ReadProjectSummary(dir)
		assert.Equal(t, content, result)
	})

	t.Run("truncates to 500 chars", func(t *testing.T) {
		dir := t.TempDir()
		longContent := make([]byte, 1000)
		for i := range longContent {
			longContent[i] = 'a'
		}
		require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), longContent, 0644))

		result := ReadProjectSummary(dir)
		assert.Len(t, result, 500)
	})

	t.Run("returns empty string if no CLAUDE.md", func(t *testing.T) {
		dir := t.TempDir()
		result := ReadProjectSummary(dir)
		assert.Empty(t, result)
	})

	t.Run("returns empty string for empty cwd", func(t *testing.T) {
		result := ReadProjectSummary("")
		assert.Empty(t, result)
	})
}

func TestReadGitBranch(t *testing.T) {
	t.Run("returns empty string for empty cwd", func(t *testing.T) {
		result := ReadGitBranch("")
		assert.Empty(t, result)
	})

	t.Run("returns empty string for non-git directory", func(t *testing.T) {
		dir := t.TempDir()
		result := ReadGitBranch(dir)
		assert.Empty(t, result)
	})
}

func TestReadModifiedFiles(t *testing.T) {
	t.Run("returns nil for empty cwd", func(t *testing.T) {
		result := ReadModifiedFiles("")
		assert.Nil(t, result)
	})

	t.Run("returns nil for non-git directory", func(t *testing.T) {
		dir := t.TempDir()
		result := ReadModifiedFiles(dir)
		assert.Nil(t, result)
	})
}

// --- Session resolution tests (T020, T021) ---

func TestResolveSession(t *testing.T) {
	sessions := []SessionInfo{
		{PaneID: 10, DisplayName: "frontend"},
		{PaneID: 20, DisplayName: "backend"},
		{PaneID: 30, DisplayName: "backend"},
	}

	t.Run("resolves numeric pane ID", func(t *testing.T) {
		id, err := resolveSession(sessions, "10")
		require.NoError(t, err)
		assert.Equal(t, uint32(10), id)
	})

	t.Run("resolves unique display name", func(t *testing.T) {
		id, err := resolveSession(sessions, "frontend")
		require.NoError(t, err)
		assert.Equal(t, uint32(10), id)
	})

	t.Run("errors on ambiguous name", func(t *testing.T) {
		_, err := resolveSession(sessions, "backend")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ambiguous session name 'backend'")
		assert.Contains(t, err.Error(), "pane 20")
		assert.Contains(t, err.Error(), "pane 30")
	})

	t.Run("errors on unknown name", func(t *testing.T) {
		_, err := resolveSession(sessions, "nonexistent")
		require.Error(t, err)
		assert.Equal(t, "session not found: nonexistent", err.Error())
	})

	t.Run("errors on unknown numeric ID", func(t *testing.T) {
		_, err := resolveSession(sessions, "999")
		require.Error(t, err)
		assert.Equal(t, "session not found: 999", err.Error())
	})
}

// --- Mock pipe sender for handler tests ---

func mockPipeSender(responses map[string]string) PipeSender {
	return func(ctx context.Context, name string, payload string) (string, error) {
		if resp, ok := responses[name]; ok {
			return resp, nil
		}
		return "", nil
	}
}

// --- handleSessions tests (T016) ---

func TestHandleSessions(t *testing.T) {
	t.Run("returns enriched sessions", func(t *testing.T) {
		pluginSessions := []SessionInfo{
			{
				PaneID:      42,
				DisplayName: "test-session",
				Activity:    "idle",
				WorkingDir:  "",
				AgentName:   "claude",
				Topic:       "test topic",
				RecentTools: []string{"Read", "Edit"},
				Paused:      false,
			},
		}
		sessionsJSON, _ := json.Marshal(pluginSessions)

		origPipeSend := pipeSend
		pipeSend = mockPipeSender(map[string]string{
			"cc-deck:mcp-sessions": string(sessionsJSON),
		})
		defer func() { pipeSend = origPipeSend }()

		req := mcp.CallToolRequest{}
		result, err := handleSessions(context.Background(), req)
		require.NoError(t, err)
		assert.False(t, result.IsError)
		assert.Len(t, result.Content, 1)

		text := result.Content[0].(mcp.TextContent).Text
		var sessions []MCPSession
		require.NoError(t, json.Unmarshal([]byte(text), &sessions))
		assert.Len(t, sessions, 1)
		assert.Equal(t, "test-session", sessions[0].Name)
		assert.Equal(t, uint32(42), sessions[0].PaneID)
		assert.Equal(t, "claude", sessions[0].Agent)
		assert.Equal(t, "idle", sessions[0].State)
	})
}

// --- handleReadScrollback tests (T021) ---

func TestHandleReadScrollback(t *testing.T) {
	pluginSessions := []SessionInfo{
		{PaneID: 42, DisplayName: "cc-spex", Activity: "idle", AgentName: "claude"},
	}
	sessionsJSON, _ := json.Marshal(pluginSessions)

	t.Run("returns scrollback text", func(t *testing.T) {
		origPipeSend := pipeSend
		pipeSend = mockPipeSender(map[string]string{
			"cc-deck:mcp-sessions":   string(sessionsJSON),
			"cc-deck:mcp-scrollback": "line1\nline2\nline3",
		})
		defer func() { pipeSend = origPipeSend }()

		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{
			"session": "cc-spex",
			"lines":   float64(50),
		}

		result, err := handleReadScrollback(context.Background(), req)
		require.NoError(t, err)
		assert.False(t, result.IsError)
		text := result.Content[0].(mcp.TextContent).Text
		assert.Equal(t, "line1\nline2\nline3", text)
	})

	t.Run("rejects invalid line count", func(t *testing.T) {
		origPipeSend := pipeSend
		pipeSend = mockPipeSender(map[string]string{
			"cc-deck:mcp-sessions": string(sessionsJSON),
		})
		defer func() { pipeSend = origPipeSend }()

		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{
			"session": "cc-spex",
			"lines":   float64(501),
		}

		result, err := handleReadScrollback(context.Background(), req)
		require.NoError(t, err)
		assert.True(t, result.IsError)
		text := result.Content[0].(mcp.TextContent).Text
		assert.Contains(t, text, "lines must be between 1 and 500")
	})

	t.Run("rejects unknown session", func(t *testing.T) {
		origPipeSend := pipeSend
		pipeSend = mockPipeSender(map[string]string{
			"cc-deck:mcp-sessions": string(sessionsJSON),
		})
		defer func() { pipeSend = origPipeSend }()

		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{
			"session": "unknown",
		}

		result, err := handleReadScrollback(context.Background(), req)
		require.NoError(t, err)
		assert.True(t, result.IsError)
		text := result.Content[0].(mcp.TextContent).Text
		assert.Contains(t, text, "session not found: unknown")
	})
}

// --- handleSessionState tests (T028) ---

func TestHandleSessionState(t *testing.T) {
	pluginSessions := []SessionInfo{
		{PaneID: 42, DisplayName: "cc-spex", Activity: "idle", AgentName: "claude"},
	}
	sessionsJSON, _ := json.Marshal(pluginSessions)

	t.Run("returns enriched session state", func(t *testing.T) {
		stateResp := map[string]any{
			"name":         "cc-spex",
			"pane_id":      42,
			"cwd":          "",
			"recent_tools": []string{"Read"},
			"topic":        "testing",
		}
		stateJSON, _ := json.Marshal(stateResp)

		origPipeSend := pipeSend
		pipeSend = mockPipeSender(map[string]string{
			"cc-deck:mcp-sessions": string(sessionsJSON),
			"cc-deck:mcp-state":    string(stateJSON),
		})
		defer func() { pipeSend = origPipeSend }()

		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{
			"session": "cc-spex",
		}

		result, err := handleSessionState(context.Background(), req)
		require.NoError(t, err)
		assert.False(t, result.IsError)

		text := result.Content[0].(mcp.TextContent).Text
		var state SessionState
		require.NoError(t, json.Unmarshal([]byte(text), &state))
		assert.Equal(t, "cc-spex", state.Name)
		assert.Equal(t, uint32(42), state.PaneID)
		assert.Equal(t, "testing", state.Topic)
	})
}
