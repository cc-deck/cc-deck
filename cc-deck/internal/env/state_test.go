package env

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) *FileStateStore {
	t.Helper()
	dir := t.TempDir()
	return NewStateStore(filepath.Join(dir, "state.yaml"))
}

func makeRecord(name string, envType EnvironmentType) *EnvironmentRecord {
	return &EnvironmentRecord{
		Name:      name,
		Type:      envType,
		State:     EnvironmentStateRunning,
		CreatedAt: time.Now().UTC().Truncate(time.Second),
	}
}

func TestNewStateStore_DefaultPath(t *testing.T) {
	store := NewStateStore("")
	assert.NotEmpty(t, store.Path())
	assert.Contains(t, store.Path(), "cc-deck")
	assert.Contains(t, store.Path(), "state.yaml")
}

func TestNewStateStore_CustomPath(t *testing.T) {
	store := NewStateStore("/tmp/custom/state.yaml")
	assert.Equal(t, "/tmp/custom/state.yaml", store.Path())
}

func TestLoad_MissingFile(t *testing.T) {
	store := newTestStore(t)
	state, err := store.Load()
	require.NoError(t, err)
	assert.Equal(t, 2, state.Version)
	assert.Empty(t, state.Environments)
	assert.Empty(t, state.Instances)
}

func TestLoad_CorruptedFile(t *testing.T) {
	store := newTestStore(t)
	dir := filepath.Dir(store.Path())
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(store.Path(), []byte(":\n  :\n    - [invalid"), 0o644))

	state, err := store.Load()
	require.NoError(t, err, "corrupted file should not return error")
	assert.Equal(t, 2, state.Version)
	assert.Empty(t, state.Environments)
}

func TestLoad_ValidFile(t *testing.T) {
	store := newTestStore(t)
	original := &StateFile{
		Version: 1,
		Environments: []EnvironmentRecord{
			{Name: "test-env", Type: EnvironmentTypeLocal, State: EnvironmentStateRunning},
		},
	}
	require.NoError(t, store.Save(original))

	loaded, err := store.Load()
	require.NoError(t, err)
	assert.Equal(t, 1, loaded.Version)
	require.Len(t, loaded.Environments, 1)
	assert.Equal(t, "test-env", loaded.Environments[0].Name)
}

func TestSave_CreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	store := NewStateStore(filepath.Join(dir, "nested", "deep", "state.yaml"))

	state := &StateFile{Version: 1}
	require.NoError(t, store.Save(state))

	_, err := os.Stat(store.Path())
	assert.NoError(t, err)
}

func TestSave_AtomicWrite(t *testing.T) {
	store := newTestStore(t)

	// Save initial state
	initial := &StateFile{
		Version:      1,
		Environments: []EnvironmentRecord{{Name: "env1", Type: EnvironmentTypeLocal}},
	}
	require.NoError(t, store.Save(initial))

	// Verify no .tmp file lingers after save
	tmpPath := store.Path() + ".tmp"
	_, err := os.Stat(tmpPath)
	assert.True(t, os.IsNotExist(err), ".tmp file should not exist after successful save")

	// Verify the actual file has correct content
	loaded, err := store.Load()
	require.NoError(t, err)
	require.Len(t, loaded.Environments, 1)
	assert.Equal(t, "env1", loaded.Environments[0].Name)
}

func TestAdd_Success(t *testing.T) {
	store := newTestStore(t)
	rec := makeRecord("my-env", EnvironmentTypeLocal)

	require.NoError(t, store.Add(rec))

	found, err := store.FindByName("my-env")
	require.NoError(t, err)
	assert.Equal(t, "my-env", found.Name)
	assert.Equal(t, EnvironmentTypeLocal, found.Type)
	assert.Equal(t, EnvironmentStateRunning, found.State)
}

func TestAdd_DuplicateName(t *testing.T) {
	store := newTestStore(t)
	rec := makeRecord("dup", EnvironmentTypeLocal)
	require.NoError(t, store.Add(rec))

	err := store.Add(makeRecord("dup", EnvironmentTypeContainer))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNameConflict))
}

func TestFindByName_NotFound(t *testing.T) {
	store := newTestStore(t)

	_, err := store.FindByName("nonexistent")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotFound))
}

func TestUpdate_Success(t *testing.T) {
	store := newTestStore(t)
	rec := makeRecord("updatable", EnvironmentTypeLocal)
	require.NoError(t, store.Add(rec))

	rec.State = EnvironmentStateStopped
	require.NoError(t, store.Update(rec))

	found, err := store.FindByName("updatable")
	require.NoError(t, err)
	assert.Equal(t, EnvironmentStateStopped, found.State)
}

func TestUpdate_NotFound(t *testing.T) {
	store := newTestStore(t)
	rec := makeRecord("ghost", EnvironmentTypeLocal)

	err := store.Update(rec)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotFound))
}

func TestRemove_Success(t *testing.T) {
	store := newTestStore(t)
	require.NoError(t, store.Add(makeRecord("removable", EnvironmentTypeLocal)))

	require.NoError(t, store.Remove("removable"))

	_, err := store.FindByName("removable")
	assert.True(t, errors.Is(err, ErrNotFound))
}

func TestRemove_NotFound(t *testing.T) {
	store := newTestStore(t)

	err := store.Remove("nonexistent")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotFound))
}

func TestList_All(t *testing.T) {
	store := newTestStore(t)
	require.NoError(t, store.Add(makeRecord("env1", EnvironmentTypeLocal)))
	require.NoError(t, store.Add(makeRecord("env2", EnvironmentTypeContainer)))
	require.NoError(t, store.Add(makeRecord("env3", EnvironmentTypeK8sDeploy)))

	list, err := store.List(nil)
	require.NoError(t, err)
	assert.Len(t, list, 3)
}

func TestList_FilterByType(t *testing.T) {
	store := newTestStore(t)
	require.NoError(t, store.Add(makeRecord("local1", EnvironmentTypeLocal)))
	require.NoError(t, store.Add(makeRecord("local2", EnvironmentTypeLocal)))
	require.NoError(t, store.Add(makeRecord("podman1", EnvironmentTypeContainer)))

	localType := EnvironmentTypeLocal
	filter := &ListFilter{Type: &localType}

	list, err := store.List(filter)
	require.NoError(t, err)
	assert.Len(t, list, 2)
	for _, rec := range list {
		assert.Equal(t, EnvironmentTypeLocal, rec.Type)
	}
}

func TestList_FilterNoMatch(t *testing.T) {
	store := newTestStore(t)
	require.NoError(t, store.Add(makeRecord("local1", EnvironmentTypeLocal)))

	sandboxType := EnvironmentTypeK8sSandbox
	filter := &ListFilter{Type: &sandboxType}

	list, err := store.List(filter)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestList_EmptyStore(t *testing.T) {
	store := newTestStore(t)

	list, err := store.List(nil)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestVersionField(t *testing.T) {
	store := newTestStore(t)
	require.NoError(t, store.Add(makeRecord("v-test", EnvironmentTypeLocal)))

	state, err := store.Load()
	require.NoError(t, err)
	assert.Equal(t, 2, state.Version)
}

func TestVersionField_V1Preserved(t *testing.T) {
	store := newTestStore(t)
	original := &StateFile{
		Version: 1,
		Environments: []EnvironmentRecord{
			{Name: "v1-env", Type: EnvironmentTypeLocal, State: EnvironmentStateRunning},
		},
	}
	require.NoError(t, store.Save(original))

	loaded, err := store.Load()
	require.NoError(t, err)
	assert.Equal(t, 1, loaded.Version)
}

func TestCRUD_FullCycle(t *testing.T) {
	store := newTestStore(t)

	// Create
	rec := makeRecord("lifecycle", EnvironmentTypeK8sDeploy)
	require.NoError(t, store.Add(rec))

	// Read
	found, err := store.FindByName("lifecycle")
	require.NoError(t, err)
	assert.Equal(t, EnvironmentTypeK8sDeploy, found.Type)

	// Update
	now := time.Now().UTC().Truncate(time.Second)
	rec.LastAttached = &now
	rec.State = EnvironmentStateStopped
	require.NoError(t, store.Update(rec))

	found, err = store.FindByName("lifecycle")
	require.NoError(t, err)
	assert.Equal(t, EnvironmentStateStopped, found.State)
	assert.NotNil(t, found.LastAttached)

	// Delete
	require.NoError(t, store.Remove("lifecycle"))
	_, err = store.FindByName("lifecycle")
	assert.True(t, errors.Is(err, ErrNotFound))
}

// --- Instance tests ---

func makeInstance(name string) *EnvironmentInstance {
	return &EnvironmentInstance{
		Name:      name,
		State:     EnvironmentStateRunning,
		CreatedAt: time.Now().UTC().Truncate(time.Second),
	}
}

func TestAddInstance_Success(t *testing.T) {
	store := newTestStore(t)
	inst := makeInstance("my-inst")

	require.NoError(t, store.AddInstance(inst))

	found, err := store.FindInstanceByName("my-inst")
	require.NoError(t, err)
	assert.Equal(t, "my-inst", found.Name)
	assert.Equal(t, EnvironmentStateRunning, found.State)
}

func TestAddInstance_Duplicate(t *testing.T) {
	store := newTestStore(t)
	require.NoError(t, store.AddInstance(makeInstance("dup")))

	err := store.AddInstance(makeInstance("dup"))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNameConflict))
}

func TestFindInstanceByName_NotFound(t *testing.T) {
	store := newTestStore(t)

	_, err := store.FindInstanceByName("nonexistent")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotFound))
}

func TestUpdateInstance_Success(t *testing.T) {
	store := newTestStore(t)
	inst := makeInstance("updatable")
	require.NoError(t, store.AddInstance(inst))

	inst.State = EnvironmentStateStopped
	require.NoError(t, store.UpdateInstance(inst))

	found, err := store.FindInstanceByName("updatable")
	require.NoError(t, err)
	assert.Equal(t, EnvironmentStateStopped, found.State)
}

func TestUpdateInstance_NotFound(t *testing.T) {
	store := newTestStore(t)
	inst := makeInstance("ghost")

	err := store.UpdateInstance(inst)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotFound))
}

func TestRemoveInstance_Success(t *testing.T) {
	store := newTestStore(t)
	require.NoError(t, store.AddInstance(makeInstance("removable")))

	require.NoError(t, store.RemoveInstance("removable"))

	_, err := store.FindInstanceByName("removable")
	assert.True(t, errors.Is(err, ErrNotFound))
}

func TestRemoveInstance_NotFound(t *testing.T) {
	store := newTestStore(t)

	err := store.RemoveInstance("nonexistent")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotFound))
}

func TestListInstances(t *testing.T) {
	store := newTestStore(t)
	require.NoError(t, store.AddInstance(makeInstance("inst1")))
	require.NoError(t, store.AddInstance(makeInstance("inst2")))

	list, err := store.ListInstances()
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestListInstances_Empty(t *testing.T) {
	store := newTestStore(t)

	list, err := store.ListInstances()
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestInstanceCRUD_FullCycle(t *testing.T) {
	store := newTestStore(t)

	// Create
	inst := makeInstance("lifecycle")
	inst.Container = &ContainerFields{
		ContainerID: "abc123",
		Image:       "my-image:latest",
	}
	require.NoError(t, store.AddInstance(inst))

	// Read
	found, err := store.FindInstanceByName("lifecycle")
	require.NoError(t, err)
	assert.Equal(t, EnvironmentStateRunning, found.State)
	require.NotNil(t, found.Container)
	assert.Equal(t, "abc123", found.Container.ContainerID)

	// Update
	now := time.Now().UTC().Truncate(time.Second)
	inst.LastAttached = &now
	inst.State = EnvironmentStateStopped
	require.NoError(t, store.UpdateInstance(inst))

	found, err = store.FindInstanceByName("lifecycle")
	require.NoError(t, err)
	assert.Equal(t, EnvironmentStateStopped, found.State)
	assert.NotNil(t, found.LastAttached)

	// Delete
	require.NoError(t, store.RemoveInstance("lifecycle"))
	_, err = store.FindInstanceByName("lifecycle")
	assert.True(t, errors.Is(err, ErrNotFound))
}

func TestStateFile_V2_WithInstances(t *testing.T) {
	store := newTestStore(t)

	// Save a v2 state file with instances.
	state := &StateFile{
		Version: 2,
		Instances: []EnvironmentInstance{
			{Name: "inst1", State: EnvironmentStateRunning, CreatedAt: time.Now().UTC()},
		},
	}
	require.NoError(t, store.Save(state))

	loaded, err := store.Load()
	require.NoError(t, err)
	assert.Equal(t, 2, loaded.Version)
	require.Len(t, loaded.Instances, 1)
	assert.Equal(t, "inst1", loaded.Instances[0].Name)
	assert.Empty(t, loaded.Environments)
}
