package ws

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cc-deck/cc-deck/internal/config"
)

func TestNeededProfileProviders_Empty(t *testing.T) {
	cfg := &config.Config{}
	assert.Empty(t, NeededProfileProviders(cfg, "myws"))
}

func TestNeededProfileProviders_ClaudeAnthropic(t *testing.T) {
	cfg := &config.Config{
		Profiles: config.ProfileGroups{
			"claude": {
				"work": config.Profile{Backend: config.BackendAnthropic},
			},
		},
	}
	got := NeededProfileProviders(cfg, "myws")
	assert.Equal(t, []string{"cc-deck-myws-claude-work"}, got)
}

func TestNeededProfileProviders_ClaudeVertexMapsToGoogleCloud(t *testing.T) {
	cfg := &config.Config{
		Profiles: config.ProfileGroups{
			"claude": {
				"vertex-profile": config.Profile{Backend: config.BackendVertex},
			},
		},
	}
	got := NeededProfileProviders(cfg, "ws1")
	assert.Equal(t, []string{"cc-deck-ws1-claude-vertex-profile"}, got)
}

func TestNeededProfileProviders_CodexAlwaysOpenAI(t *testing.T) {
	cfg := &config.Config{
		Profiles: config.ProfileGroups{
			"codex": {
				"default": config.Profile{},
			},
		},
	}
	got := NeededProfileProviders(cfg, "ws1")
	assert.Equal(t, []string{"cc-deck-ws1-codex-default"}, got)
}

func TestNeededProfileProviders_SkipsUnknownHarness(t *testing.T) {
	cfg := &config.Config{
		Profiles: config.ProfileGroups{
			"no-such-harness": {
				"x": config.Profile{},
			},
		},
	}
	assert.Empty(t, NeededProfileProviders(cfg, "ws1"))
}

func TestNeededProfileProviders_MultipleProfilesSortedByKey(t *testing.T) {
	cfg := &config.Config{
		Profiles: config.ProfileGroups{
			"claude": {
				"b-profile": config.Profile{Backend: config.BackendAnthropic},
				"a-profile": config.Profile{Backend: config.BackendAnthropic},
			},
		},
	}
	got := NeededProfileProviders(cfg, "ws1")
	assert.Equal(t, []string{
		"cc-deck-ws1-claude-a-profile",
		"cc-deck-ws1-claude-b-profile",
	}, got)
}

func TestBuildProfileTarget_WorkspaceNotFound(t *testing.T) {
	store := newTestStore(t)
	_, err := BuildProfileTarget(context.Background(), "missing-ws", store, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing-ws")
	assert.Contains(t, err.Error(), "not found")
}

func TestBuildProfileTarget_UnsupportedWorkspaceType(t *testing.T) {
	store := newTestStore(t)
	inst := makeInstance("local-ws")
	inst.Type = WorkspaceTypeLocal
	require.NoError(t, store.AddInstance(inst))

	_, err := BuildProfileTarget(context.Background(), "local-ws", store, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not support remote profile provisioning")
}

func TestBuildProfileTarget_SSHRequiresDefinitionStore(t *testing.T) {
	store := newTestStore(t)
	inst := makeInstance("ssh-ws")
	inst.Type = WorkspaceTypeSSH
	require.NoError(t, store.AddInstance(inst))

	_, err := BuildProfileTarget(context.Background(), "ssh-ws", store, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no definition store available")
}
