package tui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cc-deck/cc-deck/internal/env"
)

func TestBuildEnvRows_EmptyState(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.yaml")
	defsPath := filepath.Join(dir, "environments.yaml")

	store := env.NewStateStore(statePath)
	defs := env.NewDefinitionStore(defsPath)

	rows := buildEnvRows(store, defs)
	if len(rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(rows))
	}
}

func TestBuildEnvRows_LocalEnvironment(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.yaml")
	defsPath := filepath.Join(dir, "environments.yaml")

	store := env.NewStateStore(statePath)
	defs := env.NewDefinitionStore(defsPath)

	now := time.Now().UTC()
	record := &env.EnvironmentRecord{
		Name:         "test-local",
		Type:         env.EnvironmentTypeLocal,
		State:        env.EnvironmentStateRunning,
		CreatedAt:    now,
		LastAttached: &now,
	}
	if err := store.Add(record); err != nil {
		t.Fatalf("failed to add record: %v", err)
	}

	rows := buildEnvRows(store, defs)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}

	row := rows[0]
	if row.name != "test-local" {
		t.Errorf("name = %q, want %q", row.name, "test-local")
	}
	if row.envType != "local" {
		t.Errorf("envType = %q, want %q", row.envType, "local")
	}
	if row.storageName != "host" {
		t.Errorf("storageName = %q, want %q", row.storageName, "host")
	}
}

func TestBuildEnvRows_ContainerInstance(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.yaml")
	defsPath := filepath.Join(dir, "environments.yaml")

	// Set CC_DECK_STATE_FILE to use test state file.
	os.Setenv("CC_DECK_STATE_FILE", statePath)
	defer os.Unsetenv("CC_DECK_STATE_FILE")

	store := env.NewStateStore(statePath)
	defs := env.NewDefinitionStore(defsPath)

	inst := &env.EnvironmentInstance{
		Name:      "test-container",
		Type:      env.EnvironmentTypeContainer,
		State:     env.EnvironmentStateStopped,
		CreatedAt: time.Now().UTC(),
		Container: &env.ContainerFields{
			ContainerName: "cc-deck-test-container",
			Image:         "quay.io/test:latest",
		},
	}
	if err := store.AddInstance(inst); err != nil {
		t.Fatalf("failed to add instance: %v", err)
	}

	rows := buildEnvRows(store, defs)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}

	row := rows[0]
	if row.name != "test-container" {
		t.Errorf("name = %q, want %q", row.name, "test-container")
	}
	// ReconcileContainerEnvs checks podman inspect; since the container
	// doesn't exist in tests, state may be reconciled to "error" or "stopped".
	if row.state != "stopped" && row.state != "error" {
		t.Errorf("state = %q, want %q or %q", row.state, "stopped", "error")
	}
	if row.storageName != "named-volume" {
		t.Errorf("storageName = %q, want %q", row.storageName, "named-volume")
	}
}
