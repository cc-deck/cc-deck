package session

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	// Blank imports to register agents via their init() functions.
	_ "github.com/cc-deck/cc-deck/internal/agent"
	"github.com/cc-deck/cc-deck/internal/config"
)

func TestLaunchCommand_PreFeatureSnapshot_WithSession(t *testing.T) {
	entry := SessionEntry{SessionID: "abc123"}
	cmd, warning := launchCommand(entry, nil)
	assert.Equal(t, "claude --resume abc123", cmd)
	assert.Empty(t, warning)
}

func TestLaunchCommand_PreFeatureSnapshot_NoSession(t *testing.T) {
	entry := SessionEntry{}
	cmd, warning := launchCommand(entry, nil)
	assert.Equal(t, "claude", cmd)
	assert.Empty(t, warning)
}

func TestLaunchCommand_ClaudeWithProfile(t *testing.T) {
	cfg := &config.Config{
		Profiles: config.ProfileGroupsFromFlat(map[string]config.Profile{
			"work": {Backend: config.BackendAnthropic},
		}),
	}
	entry := SessionEntry{
		Agent:     "claude",
		Profile:   "work",
		SessionID: "abc123",
	}
	cmd, warning := launchCommand(entry, cfg)
	assert.Equal(t, "claude-work --resume abc123", cmd)
	assert.Empty(t, warning)
}

func TestLaunchCommand_SameNameAcrossHarnesses(t *testing.T) {
	cfg := &config.Config{Profiles: config.ProfileGroups{
		"claude": {"private": {Auth: &config.AuthConfig{Login: true}, Color: "#E27C5A"}},
		"codex":  {"private": {Auth: &config.AuthConfig{APIKey: &config.CredentialSource{Env: "CODEX_KEY"}}, Color: "#5AC0E2"}},
	}}
	claudeCmd, claudeWarning := launchCommand(SessionEntry{Agent: "claude", Profile: "private", SessionID: "claude-id"}, cfg)
	codexCmd, codexWarning := launchCommand(SessionEntry{Agent: "codex", Profile: "private", SessionID: "codex-id"}, cfg)
	assert.Equal(t, "claude-private --resume claude-id", claudeCmd)
	assert.Equal(t, "codex-private resume codex-id", codexCmd)
	assert.Empty(t, claudeWarning)
	assert.Empty(t, codexWarning)
	entries := []SessionEntry{
		{Agent: "claude", Profile: "private", WorkingDir: "/work/claude"},
		{Agent: "codex", Profile: "private", WorkingDir: "/work/codex"},
	}
	overrides := buildOverrides(entries, cfg)
	assert.Equal(t, "#E27C5A", overrides["/work/claude"][0].ProfileColor)
	assert.Equal(t, "#5AC0E2", overrides["/work/codex"][0].ProfileColor)
}

func TestSnapshotRoundTripRestoresSameNamedProfilesByHarness(t *testing.T) {
	const dump = `{"sessions":{
		"1":{"session_id":"claude-id","display_name":"claude","tab_index":0,"agent_name":"claude","profile":"private"},
		"2":{"session_id":"codex-id","display_name":"codex","tab_index":1,"agent_name":"codex","profile":"private"}
	}}`
	sessions, err := parseDumpState([]byte(dump))
	require.NoError(t, err)
	snapshot := Snapshot{Version: 1, Name: "same-name", Sessions: buildEntries(sessions)}
	data, err := json.Marshal(snapshot)
	require.NoError(t, err)
	var restored Snapshot
	require.NoError(t, json.Unmarshal(data, &restored))
	require.Len(t, restored.Sessions, 2)

	cfg := &config.Config{Profiles: config.ProfileGroups{
		"claude": {"private": {Auth: &config.AuthConfig{Login: true}}},
		"codex":  {"private": {Auth: &config.AuthConfig{APIKey: &config.CredentialSource{Env: "CODEX_KEY"}}}},
	}}
	claudeCommand, claudeWarning := launchCommand(restored.Sessions[0], cfg)
	codexCommand, codexWarning := launchCommand(restored.Sessions[1], cfg)
	assert.Equal(t, "claude-private --resume claude-id", claudeCommand)
	assert.Equal(t, "codex-private resume codex-id", codexCommand)
	assert.Empty(t, claudeWarning)
	assert.Empty(t, codexWarning)
}

func TestLaunchCommand_MissingProfile(t *testing.T) {
	cfg := &config.Config{
		Profiles: config.ProfileGroupsFromFlat(map[string]config.Profile{}),
	}
	entry := SessionEntry{
		Agent:     "claude",
		Profile:   "deleted",
		SessionID: "abc123",
	}
	cmd, warning := launchCommand(entry, cfg)
	assert.Equal(t, "claude --resume abc123", cmd)
	assert.Contains(t, warning, "not found in config")
	assert.Contains(t, warning, "deleted")
}

func TestLaunchCommand_CodexAgent(t *testing.T) {
	entry := SessionEntry{
		Agent:     "codex",
		SessionID: "sess1",
	}
	cmd, warning := launchCommand(entry, nil)
	assert.Equal(t, "codex resume sess1", cmd)
	assert.Empty(t, warning)
}

func TestLaunchCommand_ProfileNoSessionID(t *testing.T) {
	cfg := &config.Config{
		Profiles: config.ProfileGroupsFromFlat(map[string]config.Profile{
			"work": {Backend: config.BackendAnthropic},
		}),
	}
	entry := SessionEntry{
		Agent:   "claude",
		Profile: "work",
	}
	cmd, warning := launchCommand(entry, cfg)
	assert.Equal(t, "claude-work", cmd)
	assert.Empty(t, warning)
}

func TestLaunchCommand_OpenCodeAgent(t *testing.T) {
	entry := SessionEntry{
		Agent:     "opencode",
		SessionID: "oc1",
	}
	cmd, warning := launchCommand(entry, nil)
	assert.Equal(t, "opencode --session oc1", cmd)
	assert.Empty(t, warning)
}

// A backgrounded Claude entry relaunches as a foreground resume; the background
// marker travels via the restore-meta override, not the launch command.
func TestLaunchCommand_BackgroundEntryResumesForeground(t *testing.T) {
	entry := SessionEntry{
		Agent:      "claude",
		SessionID:  "bg1",
		Background: true,
	}
	cmd, warning := launchCommand(entry, nil)
	assert.Equal(t, "claude --resume bg1", cmd)
	assert.Empty(t, warning)
}

func TestBuildOverrides_CarriesBackgroundMarker(t *testing.T) {
	sessions := []SessionEntry{
		{WorkingDir: "/src/fg", DisplayName: "fg", Background: false},
		{WorkingDir: "/src/bg", DisplayName: "bg", Paused: true, Background: true},
		{WorkingDir: "", DisplayName: "skipped"}, // no working dir: dropped
	}
	overrides := buildOverrides(sessions, nil)
	assert.Len(t, overrides, 2)
	assert.False(t, overrides["/src/fg"][0].Background)
	assert.True(t, overrides["/src/bg"][0].Background, "background marker must be sent to the plugin on restore")
	assert.True(t, overrides["/src/bg"][0].Paused)
}
