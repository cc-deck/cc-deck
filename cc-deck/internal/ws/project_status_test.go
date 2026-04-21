package ws

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProjectStatusStore_LoadMissing(t *testing.T) {
	dir := t.TempDir()
	store := NewProjectStatusStore(dir)

	status, err := store.Load()
	require.NoError(t, err)
	assert.Equal(t, WorkspaceState(""), status.State)
	assert.Empty(t, status.ContainerName)
}

func TestProjectStatusStore_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	store := NewProjectStatusStore(dir)

	now := time.Now().UTC().Truncate(time.Second)
	status := &ProjectStatusFile{
		State:         WorkspaceStateRunning,
		ContainerName: "cc-deck-my-api",
		CreatedAt:     now,
	}

	require.NoError(t, store.Save(status))

	loaded, err := store.Load()
	require.NoError(t, err)
	assert.Equal(t, WorkspaceStateRunning, loaded.State)
	assert.Equal(t, "cc-deck-my-api", loaded.ContainerName)
	assert.Equal(t, now, loaded.CreatedAt)
}

func TestProjectStatusStore_SaveWithVariant(t *testing.T) {
	dir := t.TempDir()
	store := NewProjectStatusStore(dir)

	now := time.Now().UTC().Truncate(time.Second)
	attached := now.Add(time.Hour)
	status := &ProjectStatusFile{
		Variant:       "auth",
		State:         WorkspaceStateRunning,
		ContainerName: "cc-deck-my-api-auth",
		CreatedAt:     now,
		LastAttached:  &attached,
		Overrides:     map[string]string{"image": "custom:latest"},
	}

	require.NoError(t, store.Save(status))

	loaded, err := store.Load()
	require.NoError(t, err)
	assert.Equal(t, "auth", loaded.Variant)
	assert.NotNil(t, loaded.LastAttached)
	assert.Equal(t, "custom:latest", loaded.Overrides["image"])
}

func TestProjectStatusStore_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	store := NewProjectStatusStore(dir)

	status := &ProjectStatusFile{
		State:         WorkspaceStateStopped,
		ContainerName: "cc-deck-test",
		CreatedAt:     time.Now().UTC().Truncate(time.Second),
	}
	require.NoError(t, store.Save(status))

	tmpPath := filepath.Join(dir, ".cc-deck", "status.yaml.tmp")
	_, err := os.Stat(tmpPath)
	assert.True(t, os.IsNotExist(err), ".tmp file should not exist after save")
}

func TestProjectStatusStore_Remove(t *testing.T) {
	dir := t.TempDir()
	store := NewProjectStatusStore(dir)

	status := &ProjectStatusFile{
		State:         WorkspaceStateRunning,
		ContainerName: "cc-deck-remove-me",
		CreatedAt:     time.Now().UTC().Truncate(time.Second),
	}
	require.NoError(t, store.Save(status))

	require.NoError(t, store.Remove())

	// After remove, Load should return empty status.
	loaded, err := store.Load()
	require.NoError(t, err)
	assert.Equal(t, WorkspaceState(""), loaded.State)
}

func TestProjectStatusStore_RemoveNonexistent(t *testing.T) {
	dir := t.TempDir()
	store := NewProjectStatusStore(dir)

	// Removing a nonexistent file should not error.
	require.NoError(t, store.Remove())
}

func TestProjectStatusStore_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	store := NewProjectStatusStore(dir)

	status := &ProjectStatusFile{
		State:         WorkspaceStateRunning,
		ContainerName: "cc-deck-test",
		CreatedAt:     time.Now().UTC().Truncate(time.Second),
	}
	require.NoError(t, store.Save(status))

	info, err := os.Stat(filepath.Join(dir, ".cc-deck"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}
