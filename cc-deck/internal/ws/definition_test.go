package ws

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestDefinitionStore(t *testing.T) *DefinitionStore {
	t.Helper()
	dir := t.TempDir()
	return NewDefinitionStore(filepath.Join(dir, "workspaces.yaml"))
}

func makeDef(name string, wsType WorkspaceType) *WorkspaceDefinition {
	return &WorkspaceDefinition{
		Name: name,
		Type: wsType,
	}
}

func TestNewDefinitionStore_DefaultPath(t *testing.T) {
	store := NewDefinitionStore("")
	assert.NotEmpty(t, store.Path())
	assert.Contains(t, store.Path(), "cc-deck")
	assert.Contains(t, store.Path(), "workspaces.yaml")
}

func TestNewDefinitionStore_CustomPath(t *testing.T) {
	store := NewDefinitionStore("/tmp/custom/workspaces.yaml")
	assert.Equal(t, "/tmp/custom/workspaces.yaml", store.Path())
}

func TestDefinitionLoad_MissingFile(t *testing.T) {
	store := newTestDefinitionStore(t)
	defs, err := store.Load()
	require.NoError(t, err)
	assert.Equal(t, 1, defs.Version)
	assert.Empty(t, defs.Workspaces)
}

func TestDefinitionLoad_CorruptedFile(t *testing.T) {
	store := newTestDefinitionStore(t)
	dir := filepath.Dir(store.Path())
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(store.Path(), []byte(":\n  :\n    - [invalid"), 0o644))

	defs, err := store.Load()
	require.NoError(t, err, "corrupted file should not return error")
	assert.Equal(t, 1, defs.Version)
	assert.Empty(t, defs.Workspaces)
}

func TestDefinitionAdd_Success(t *testing.T) {
	store := newTestDefinitionStore(t)
	def := makeDef("my-env", WorkspaceTypeContainer)

	require.NoError(t, store.Add(def))

	found, err := store.FindByName("my-env")
	require.NoError(t, err)
	assert.Equal(t, "my-env", found.Name)
	assert.Equal(t, WorkspaceTypeContainer, found.Type)
}

func TestDefinitionAdd_Duplicate(t *testing.T) {
	store := newTestDefinitionStore(t)
	require.NoError(t, store.Add(makeDef("dup", WorkspaceTypeContainer)))

	err := store.Add(makeDef("dup", WorkspaceTypeLocal))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNameConflict))
}

func TestDefinitionFindByName_NotFound(t *testing.T) {
	store := newTestDefinitionStore(t)

	_, err := store.FindByName("nonexistent")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotFound))
}

func TestDefinitionUpdate_Success(t *testing.T) {
	store := newTestDefinitionStore(t)
	def := makeDef("updatable", WorkspaceTypeContainer)
	def.Image = "old-image:latest"
	require.NoError(t, store.Add(def))

	def.Image = "new-image:v2"
	require.NoError(t, store.Update(def))

	found, err := store.FindByName("updatable")
	require.NoError(t, err)
	assert.Equal(t, "new-image:v2", found.Image)
}

func TestDefinitionUpdate_NotFound(t *testing.T) {
	store := newTestDefinitionStore(t)
	def := makeDef("ghost", WorkspaceTypeContainer)

	err := store.Update(def)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotFound))
}

func TestDefinitionRemove_Success(t *testing.T) {
	store := newTestDefinitionStore(t)
	require.NoError(t, store.Add(makeDef("removable", WorkspaceTypeContainer)))

	require.NoError(t, store.Remove("removable"))

	_, err := store.FindByName("removable")
	assert.True(t, errors.Is(err, ErrNotFound))
}

func TestDefinitionRemove_NotFound(t *testing.T) {
	store := newTestDefinitionStore(t)

	err := store.Remove("nonexistent")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotFound))
}

func TestDefinitionList_All(t *testing.T) {
	store := newTestDefinitionStore(t)
	require.NoError(t, store.Add(makeDef("env1", WorkspaceTypeLocal)))
	require.NoError(t, store.Add(makeDef("env2", WorkspaceTypeContainer)))
	require.NoError(t, store.Add(makeDef("env3", WorkspaceTypeK8sDeploy)))

	list, err := store.List(nil)
	require.NoError(t, err)
	assert.Len(t, list, 3)
}

func TestDefinitionList_FilterByType(t *testing.T) {
	store := newTestDefinitionStore(t)
	require.NoError(t, store.Add(makeDef("local1", WorkspaceTypeLocal)))
	require.NoError(t, store.Add(makeDef("local2", WorkspaceTypeLocal)))
	require.NoError(t, store.Add(makeDef("container1", WorkspaceTypeContainer)))

	localType := WorkspaceTypeLocal
	filter := &ListFilter{Type: &localType}

	list, err := store.List(filter)
	require.NoError(t, err)
	assert.Len(t, list, 2)
	for _, def := range list {
		assert.Equal(t, WorkspaceTypeLocal, def.Type)
	}
}

func TestDefinitionList_Empty(t *testing.T) {
	store := newTestDefinitionStore(t)

	list, err := store.List(nil)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestDefinitionAtomicWrite(t *testing.T) {
	store := newTestDefinitionStore(t)

	def := makeDef("atomic-test", WorkspaceTypeContainer)
	require.NoError(t, store.Add(def))

	// Verify no .tmp file lingers after save.
	tmpPath := store.Path() + ".tmp"
	_, err := os.Stat(tmpPath)
	assert.True(t, os.IsNotExist(err), ".tmp file should not exist after successful save")

	// Verify content is correct.
	found, err := store.FindByName("atomic-test")
	require.NoError(t, err)
	assert.Equal(t, "atomic-test", found.Name)
}

func TestDefinitionCRUD_FullCycle(t *testing.T) {
	store := newTestDefinitionStore(t)

	// Create
	def := &WorkspaceDefinition{
		Name: "lifecycle",
		Type: WorkspaceTypeContainer,
		WorkspaceSpec: WorkspaceSpec{
			Image: "my-image:latest",
			Ports: []string{"8080:8080"},
		},
	}
	require.NoError(t, store.Add(def))

	// Read
	found, err := store.FindByName("lifecycle")
	require.NoError(t, err)
	assert.Equal(t, WorkspaceTypeContainer, found.Type)
	assert.Equal(t, "my-image:latest", found.Image)
	assert.Equal(t, []string{"8080:8080"}, found.Ports)

	// Update
	def.Image = "my-image:v2"
	def.Credentials = []string{"gcloud-adc"}
	require.NoError(t, store.Update(def))

	found, err = store.FindByName("lifecycle")
	require.NoError(t, err)
	assert.Equal(t, "my-image:v2", found.Image)
	assert.Equal(t, []string{"gcloud-adc"}, found.Credentials)

	// Delete
	require.NoError(t, store.Remove("lifecycle"))
	_, err = store.FindByName("lifecycle")
	assert.True(t, errors.Is(err, ErrNotFound))
}

func TestDefinitionEnvVarOverride(t *testing.T) {
	customPath := filepath.Join(t.TempDir(), "custom-defs.yaml")
	t.Setenv("CC_DECK_WORKSPACES_FILE", customPath)

	path := DefaultDefinitionPath()
	assert.Equal(t, customPath, path)
}

func TestFindByProjectDir_ExactMatch(t *testing.T) {
	store := newTestDefinitionStore(t)
	def := makeDef("proj-ws", WorkspaceTypeContainer)
	projDir := t.TempDir()
	resolved, _ := filepath.EvalSymlinks(projDir)
	def.ProjectDir = resolved
	require.NoError(t, store.Add(def))

	results, err := store.FindByProjectDir(projDir)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "proj-ws", results[0].Name)
}

func TestFindByProjectDir_SubdirectoryMatch(t *testing.T) {
	store := newTestDefinitionStore(t)
	def := makeDef("proj-ws", WorkspaceTypeContainer)
	projDir := t.TempDir()
	resolved, _ := filepath.EvalSymlinks(projDir)
	def.ProjectDir = resolved
	require.NoError(t, store.Add(def))

	subDir := filepath.Join(projDir, "src", "pkg")
	require.NoError(t, os.MkdirAll(subDir, 0o755))

	results, err := store.FindByProjectDir(subDir)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "proj-ws", results[0].Name)
}

func TestFindByProjectDir_NoMatch(t *testing.T) {
	store := newTestDefinitionStore(t)
	def := makeDef("proj-ws", WorkspaceTypeContainer)
	def.ProjectDir = "/some/other/path"
	require.NoError(t, store.Add(def))

	results, err := store.FindByProjectDir(t.TempDir())
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestAddWithCollisionHandling_NoCollision(t *testing.T) {
	store := newTestDefinitionStore(t)
	def := makeDef("my-ws", WorkspaceTypeContainer)

	name, err := store.AddWithCollisionHandling(def)
	require.NoError(t, err)
	assert.Equal(t, "my-ws", name)

	found, err := store.FindByName("my-ws")
	require.NoError(t, err)
	assert.Equal(t, WorkspaceTypeContainer, found.Type)
}

func TestAddWithCollisionHandling_SameTypeCollision(t *testing.T) {
	store := newTestDefinitionStore(t)
	require.NoError(t, store.Add(makeDef("my-ws", WorkspaceTypeContainer)))

	def := makeDef("my-ws", WorkspaceTypeContainer)
	_, err := store.AddWithCollisionHandling(def)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists (type: container)")
}

func TestAddWithCollisionHandling_DifferentTypeAutoSuffix(t *testing.T) {
	store := newTestDefinitionStore(t)
	require.NoError(t, store.Add(makeDef("my-ws", WorkspaceTypeContainer)))

	def := makeDef("my-ws", WorkspaceTypeSSH)
	name, err := store.AddWithCollisionHandling(def)
	require.NoError(t, err)
	assert.Equal(t, "my-ws-ssh", name)

	found, err := store.FindByName("my-ws-ssh")
	require.NoError(t, err)
	assert.Equal(t, WorkspaceTypeSSH, found.Type)
}

func TestAddWithCollisionHandling_DoesNotMutateInputOnCollision(t *testing.T) {
	store := newTestDefinitionStore(t)
	require.NoError(t, store.Add(makeDef("my-ws", WorkspaceTypeContainer)))

	def := makeDef("my-ws", WorkspaceTypeSSH)
	originalName := def.Name

	name, err := store.AddWithCollisionHandling(def)
	require.NoError(t, err)
	assert.Equal(t, "my-ws-ssh", name)
	assert.Equal(t, "my-ws-ssh", def.Name, "def.Name should be updated to final name after save")
	_ = originalName
}

func TestAddWithCollisionHandling_ValidatesAutoSuffixedName(t *testing.T) {
	store := newTestDefinitionStore(t)
	longName := "a234567890123456789012345678901234567890"
	require.NoError(t, store.Add(makeDef(longName, WorkspaceTypeContainer)))

	def := makeDef(longName, WorkspaceTypeSSH)
	_, err := store.AddWithCollisionHandling(def)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "auto-suffixed name")
}

func TestDefinitionSave_CreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	store := NewDefinitionStore(filepath.Join(dir, "nested", "deep", "workspaces.yaml"))

	defs := &DefinitionFile{Version: 1}
	require.NoError(t, store.Save(defs))

	_, err := os.Stat(store.Path())
	assert.NoError(t, err)
}
