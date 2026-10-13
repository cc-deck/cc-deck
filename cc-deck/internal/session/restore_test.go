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
