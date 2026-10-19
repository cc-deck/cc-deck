package session

import (
	"testing"

	"github.com/stretchr/testify/assert"

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
		Profiles: map[string]config.Profile{
			"work": {Backend: config.BackendAnthropic},
		},
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

func TestLaunchCommand_MissingProfile(t *testing.T) {
	cfg := &config.Config{
		Profiles: map[string]config.Profile{},
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
		Profiles: map[string]config.Profile{
			"work": {Backend: config.BackendAnthropic},
		},
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
