package env

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cc-deck/cc-deck/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrateFromConfig_WithSessions(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	statePath := filepath.Join(dir, "state.yaml")

	// Create a config file with sessions.
	cfg := &config.Config{
		Sessions: []config.Session{
			{
				Name:      "my-session",
				Namespace: "dev",
				Profile:   "vertex-prod",
				Status:    "running",
				PodName:   "my-session-0",
				CreatedAt: "2025-01-15T10:30:00Z",
			},
			{
				Name:      "other-session",
				Namespace: "staging",
				Profile:   "anthropic",
				Status:    "stopped",
				PodName:   "other-session-0",
				CreatedAt: "2025-02-20T14:00:00Z",
			},
		},
	}
	require.NoError(t, cfg.Save(configPath))

	store := NewStateStore(statePath)
	require.NoError(t, MigrateFromConfig(configPath, store))

	// Verify environments were created.
	list, err := store.List(nil)
	require.NoError(t, err)
	require.Len(t, list, 2)

	// Verify first record.
	rec1, err := store.FindByName("my-session")
	require.NoError(t, err)
	assert.Equal(t, EnvironmentTypeK8sDeploy, rec1.Type)
	assert.Equal(t, EnvironmentStateRunning, rec1.State)
	assert.Equal(t, "dev", rec1.K8s.Namespace)
	assert.Equal(t, "vertex-prod", rec1.K8s.Profile)
	assert.Equal(t, "my-session", rec1.K8s.StatefulSet)
	assert.Equal(t, 2025, rec1.CreatedAt.Year())

	// Verify second record (status "stopped" maps to Unknown).
	rec2, err := store.FindByName("other-session")
	require.NoError(t, err)
	assert.Equal(t, EnvironmentStateUnknown, rec2.State)
	assert.Equal(t, "staging", rec2.K8s.Namespace)
	assert.Equal(t, "other-session", rec2.K8s.StatefulSet)

	// Verify sessions were removed from config.
	cfgAfter, err := config.Load(configPath)
	require.NoError(t, err)
	assert.Empty(t, cfgAfter.Sessions)
}

func TestMigrateFromConfig_EmptySessions(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	statePath := filepath.Join(dir, "state.yaml")

	// Create a config with no sessions.
	cfg := &config.Config{
		DefaultProfile: "test",
	}
	require.NoError(t, cfg.Save(configPath))

	store := NewStateStore(statePath)
	require.NoError(t, MigrateFromConfig(configPath, store))

	// State file should not have been created (no environments).
	list, err := store.List(nil)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestMigrateFromConfig_FileNotFound(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "nonexistent.yaml")
	statePath := filepath.Join(dir, "state.yaml")

	store := NewStateStore(statePath)

	// Should be a no-op, not an error.
	require.NoError(t, MigrateFromConfig(configPath, store))

	// State file should be empty.
	list, err := store.List(nil)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestMigrateFromConfig_Idempotent(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	statePath := filepath.Join(dir, "state.yaml")

	cfg := &config.Config{
		Sessions: []config.Session{
			{
				Name:      "idem-env",
				Namespace: "dev",
				Status:    "running",
				PodName:   "idem-env-0",
				CreatedAt: "2025-06-01T00:00:00Z",
			},
		},
	}
	require.NoError(t, cfg.Save(configPath))

	store := NewStateStore(statePath)

	// First migration.
	require.NoError(t, MigrateFromConfig(configPath, store))

	// Write sessions back to config to simulate re-run scenario.
	cfg.Sessions = []config.Session{
		{
			Name:      "idem-env",
			Namespace: "dev",
			Status:    "running",
			PodName:   "idem-env-0",
			CreatedAt: "2025-06-01T00:00:00Z",
		},
	}
	require.NoError(t, cfg.Save(configPath))

	// Second migration should skip duplicates.
	require.NoError(t, MigrateFromConfig(configPath, store))

	list, err := store.List(nil)
	require.NoError(t, err)
	assert.Len(t, list, 1, "should not duplicate environment on re-migration")
}

func TestDeriveStatefulSet(t *testing.T) {
	tests := []struct {
		podName string
		want    string
	}{
		{"my-app-0", "my-app"},
		{"simple-0", "simple"},
		{"no-suffix", "no-suffix"},
		{"", ""},
		{"multi-0-0", "multi-0"},
	}
	for _, tt := range tests {
		t.Run(tt.podName, func(t *testing.T) {
			got := deriveStatefulSet(tt.podName)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMapSessionStatus(t *testing.T) {
	tests := []struct {
		input string
		want  EnvironmentState
	}{
		{"running", EnvironmentStateRunning},
		{"Running", EnvironmentStateRunning},
		{"RUNNING", EnvironmentStateRunning},
		{"stopped", EnvironmentStateUnknown},
		{"error", EnvironmentStateUnknown},
		{"", EnvironmentStateUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := mapSessionStatus(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMigrateFromConfig_PreservesConfigFields(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	statePath := filepath.Join(dir, "state.yaml")

	cfg := &config.Config{
		DefaultProfile: "my-profile",
		Sessions: []config.Session{
			{Name: "to-migrate", Namespace: "ns", Status: "running", PodName: "to-migrate-0"},
		},
	}
	require.NoError(t, cfg.Save(configPath))

	store := NewStateStore(statePath)
	require.NoError(t, MigrateFromConfig(configPath, store))

	// Verify non-session fields are preserved.
	cfgAfter, err := config.Load(configPath)
	require.NoError(t, err)
	assert.Equal(t, "my-profile", cfgAfter.DefaultProfile)
	assert.Empty(t, cfgAfter.Sessions)

	// Verify the config file still exists.
	_, err = os.Stat(configPath)
	assert.NoError(t, err)
}
