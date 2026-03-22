package env

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterProject_NewEntry(t *testing.T) {
	store := newTestStore(t)
	dir := t.TempDir()

	require.NoError(t, store.RegisterProject(dir))

	projects, err := store.ListProjects()
	require.NoError(t, err)
	require.Len(t, projects, 1)
	assert.NotEmpty(t, projects[0].Path)
	assert.False(t, projects[0].LastSeen.IsZero())
}

func TestRegisterProject_UpdatesLastSeen(t *testing.T) {
	store := newTestStore(t)
	dir := t.TempDir()

	require.NoError(t, store.RegisterProject(dir))

	projects, err := store.ListProjects()
	require.NoError(t, err)
	firstSeen := projects[0].LastSeen

	// Wait to ensure time difference.
	time.Sleep(time.Millisecond)

	require.NoError(t, store.RegisterProject(dir))

	projects, err = store.ListProjects()
	require.NoError(t, err)
	require.Len(t, projects, 1, "should not duplicate entry")
	assert.True(t, !projects[0].LastSeen.Before(firstSeen))
}

func TestRegisterProject_ResolvesSymlinks(t *testing.T) {
	store := newTestStore(t)
	dir := t.TempDir()

	target := filepath.Join(dir, "target")
	require.NoError(t, os.MkdirAll(target, 0o755))
	link := filepath.Join(dir, "link")
	require.NoError(t, os.Symlink(target, link))

	// Register via symlink.
	require.NoError(t, store.RegisterProject(link))

	// Register via target path.
	require.NoError(t, store.RegisterProject(target))

	projects, err := store.ListProjects()
	require.NoError(t, err)
	assert.Len(t, projects, 1, "symlink and target should resolve to same entry")
}

func TestUnregisterProject(t *testing.T) {
	store := newTestStore(t)
	dir := t.TempDir()

	require.NoError(t, store.RegisterProject(dir))
	require.NoError(t, store.UnregisterProject(dir))

	projects, err := store.ListProjects()
	require.NoError(t, err)
	assert.Empty(t, projects)
}

func TestUnregisterProject_Nonexistent(t *testing.T) {
	store := newTestStore(t)

	// Should not error when unregistering something not registered.
	require.NoError(t, store.UnregisterProject("/nonexistent/path"))
}

func TestListProjects_Empty(t *testing.T) {
	store := newTestStore(t)

	projects, err := store.ListProjects()
	require.NoError(t, err)
	assert.Empty(t, projects)
}

func TestListProjects_Multiple(t *testing.T) {
	store := newTestStore(t)
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	require.NoError(t, store.RegisterProject(dir1))
	require.NoError(t, store.RegisterProject(dir2))

	projects, err := store.ListProjects()
	require.NoError(t, err)
	assert.Len(t, projects, 2)
}

func TestPruneStaleProjects_RemovesStale(t *testing.T) {
	store := newTestStore(t)

	existing := t.TempDir()
	require.NoError(t, store.RegisterProject(existing))

	// Register a path that we then remove.
	stale := filepath.Join(t.TempDir(), "gone")
	require.NoError(t, os.MkdirAll(stale, 0o755))
	require.NoError(t, store.RegisterProject(stale))
	require.NoError(t, os.RemoveAll(stale))

	removed, err := store.PruneStaleProjects()
	require.NoError(t, err)
	assert.Equal(t, 1, removed)

	projects, err := store.ListProjects()
	require.NoError(t, err)
	assert.Len(t, projects, 1)
}

func TestPruneStaleProjects_NothingToRemove(t *testing.T) {
	store := newTestStore(t)
	dir := t.TempDir()
	require.NoError(t, store.RegisterProject(dir))

	removed, err := store.PruneStaleProjects()
	require.NoError(t, err)
	assert.Equal(t, 0, removed)
}

func TestPruneStaleProjects_EmptyRegistry(t *testing.T) {
	store := newTestStore(t)

	removed, err := store.PruneStaleProjects()
	require.NoError(t, err)
	assert.Equal(t, 0, removed)
}
