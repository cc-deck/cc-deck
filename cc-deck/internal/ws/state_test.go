package ws

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

func makeInstance(name string) *WorkspaceInstance {
	return &WorkspaceInstance{
		Name:      name,
		State:     WorkspaceStateRunning,
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
	assert.Equal(t, 3, state.Version)
	assert.Empty(t, state.Instances)
}

func TestLoad_CorruptedFile(t *testing.T) {
	store := newTestStore(t)
	dir := filepath.Dir(store.Path())
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(store.Path(), []byte(":\n  :\n    - [invalid"), 0o644))

	state, err := store.Load()
	require.NoError(t, err, "corrupted file should not return error")
	assert.Equal(t, 3, state.Version)
	assert.Empty(t, state.Instances)
}

func TestLoad_ValidV3File(t *testing.T) {
	store := newTestStore(t)
	original := &StateFile{
		Version: 3,
		Instances: []WorkspaceInstance{
			{Name: "test-env", Type: WorkspaceTypeLocal, SessionState: SessionStateExists},
		},
	}
	require.NoError(t, store.Save(original))

	loaded, err := store.Load()
	require.NoError(t, err)
	assert.Equal(t, 3, loaded.Version)
	require.Len(t, loaded.Instances, 1)
	assert.Equal(t, "test-env", loaded.Instances[0].Name)
	assert.Equal(t, SessionStateExists, loaded.Instances[0].SessionState)
	assert.Nil(t, loaded.Instances[0].InfraState)
}

func TestSave_CreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	store := NewStateStore(filepath.Join(dir, "nested", "deep", "state.yaml"))

	state := &StateFile{Version: 3}
	require.NoError(t, store.Save(state))

	_, err := os.Stat(store.Path())
	assert.NoError(t, err)
}

func TestSave_AtomicWrite(t *testing.T) {
	store := newTestStore(t)

	initial := &StateFile{
		Version: 3,
		Instances: []WorkspaceInstance{
			{Name: "env1", Type: WorkspaceTypeLocal},
		},
	}
	require.NoError(t, store.Save(initial))

	// Verify no .tmp file lingers after save
	tmpPath := store.Path() + ".tmp"
	_, err := os.Stat(tmpPath)
	assert.True(t, os.IsNotExist(err), ".tmp file should not exist after successful save")

	loaded, err := store.Load()
	require.NoError(t, err)
	require.Len(t, loaded.Instances, 1)
	assert.Equal(t, "env1", loaded.Instances[0].Name)
}

func TestAddInstance_Success(t *testing.T) {
	store := newTestStore(t)
	inst := makeInstance("my-inst")

	require.NoError(t, store.AddInstance(inst))

	found, err := store.FindInstanceByName("my-inst")
	require.NoError(t, err)
	assert.Equal(t, "my-inst", found.Name)
	assert.Equal(t, WorkspaceStateRunning, found.State)
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

	inst.State = WorkspaceStateStopped
	require.NoError(t, store.UpdateInstance(inst))

	found, err := store.FindInstanceByName("updatable")
	require.NoError(t, err)
	assert.Equal(t, WorkspaceStateStopped, found.State)
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

	list, err := store.ListInstances(nil)
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestListInstances_Empty(t *testing.T) {
	store := newTestStore(t)

	list, err := store.ListInstances(nil)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestListInstances_FilterByType(t *testing.T) {
	store := newTestStore(t)
	local := makeInstance("local1")
	local.Type = WorkspaceTypeLocal
	require.NoError(t, store.AddInstance(local))

	container := makeInstance("container1")
	container.Type = WorkspaceTypeContainer
	require.NoError(t, store.AddInstance(container))

	localType := WorkspaceTypeLocal
	list, err := store.ListInstances(&ListFilter{Type: &localType})
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, "local1", list[0].Name)
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
	assert.Equal(t, WorkspaceStateRunning, found.State)
	require.NotNil(t, found.Container)
	assert.Equal(t, "abc123", found.Container.ContainerID)

	// Update
	now := time.Now().UTC().Truncate(time.Second)
	inst.LastAttached = &now
	inst.State = WorkspaceStateStopped
	require.NoError(t, store.UpdateInstance(inst))

	found, err = store.FindInstanceByName("lifecycle")
	require.NoError(t, err)
	assert.Equal(t, WorkspaceStateStopped, found.State)
	assert.NotNil(t, found.LastAttached)

	// Delete
	require.NoError(t, store.RemoveInstance("lifecycle"))
	_, err = store.FindInstanceByName("lifecycle")
	assert.True(t, errors.Is(err, ErrNotFound))
}

func TestVersionField(t *testing.T) {
	store := newTestStore(t)
	require.NoError(t, store.AddInstance(makeInstance("v-test")))

	state, err := store.Load()
	require.NoError(t, err)
	assert.Equal(t, 3, state.Version)
}

func TestMigrateV2toV3_LocalRunning(t *testing.T) {
	store := newTestStore(t)
	v2 := &StateFile{
		Version: 2,
		Instances: []WorkspaceInstance{
			{Name: "mydev", Type: WorkspaceTypeLocal, State: WorkspaceStateRunning},
		},
	}
	require.NoError(t, store.Save(v2))

	loaded, err := store.Load()
	require.NoError(t, err)
	assert.Equal(t, 3, loaded.Version)
	inst := loaded.Instances[0]
	assert.Nil(t, inst.InfraState)
	assert.Equal(t, SessionStateExists, inst.SessionState)
	assert.Equal(t, WorkspaceState(""), inst.State)
}

func TestMigrateV2toV3_ContainerRunning(t *testing.T) {
	store := newTestStore(t)
	v2 := &StateFile{
		Version: 2,
		Instances: []WorkspaceInstance{
			{Name: "mycontainer", Type: WorkspaceTypeContainer, State: WorkspaceStateRunning},
		},
	}
	require.NoError(t, store.Save(v2))

	loaded, err := store.Load()
	require.NoError(t, err)
	inst := loaded.Instances[0]
	require.NotNil(t, inst.InfraState)
	assert.Equal(t, InfraStateRunning, *inst.InfraState)
	assert.Equal(t, SessionStateNone, inst.SessionState)
}

func TestMigrateV2toV3_ContainerStopped(t *testing.T) {
	store := newTestStore(t)
	v2 := &StateFile{
		Version: 2,
		Instances: []WorkspaceInstance{
			{Name: "stopped-c", Type: WorkspaceTypeContainer, State: WorkspaceStateStopped},
		},
	}
	require.NoError(t, store.Save(v2))

	loaded, err := store.Load()
	require.NoError(t, err)
	inst := loaded.Instances[0]
	require.NotNil(t, inst.InfraState)
	assert.Equal(t, InfraStateStopped, *inst.InfraState)
	assert.Equal(t, SessionStateNone, inst.SessionState)
}

func TestMigrateV2toV3_SSHRunning(t *testing.T) {
	store := newTestStore(t)
	v2 := &StateFile{
		Version: 2,
		Instances: []WorkspaceInstance{
			{Name: "remote", Type: WorkspaceTypeSSH, State: WorkspaceStateRunning},
		},
	}
	require.NoError(t, store.Save(v2))

	loaded, err := store.Load()
	require.NoError(t, err)
	inst := loaded.Instances[0]
	assert.Nil(t, inst.InfraState)
	assert.Equal(t, SessionStateExists, inst.SessionState)
}

func TestMigrateV2toV3_LocalStopped(t *testing.T) {
	store := newTestStore(t)
	v2 := &StateFile{
		Version: 2,
		Instances: []WorkspaceInstance{
			{Name: "stopped-local", Type: WorkspaceTypeLocal, State: WorkspaceStateStopped},
		},
	}
	require.NoError(t, store.Save(v2))

	loaded, err := store.Load()
	require.NoError(t, err)
	inst := loaded.Instances[0]
	assert.Nil(t, inst.InfraState)
	assert.Equal(t, SessionStateNone, inst.SessionState)
}

func TestMigrateV2toV3_ErrorState(t *testing.T) {
	store := newTestStore(t)
	v2 := &StateFile{
		Version: 2,
		Instances: []WorkspaceInstance{
			{Name: "errored", Type: WorkspaceTypeContainer, State: WorkspaceStateError},
		},
	}
	require.NoError(t, store.Save(v2))

	loaded, err := store.Load()
	require.NoError(t, err)
	inst := loaded.Instances[0]
	require.NotNil(t, inst.InfraState)
	assert.Equal(t, InfraStateError, *inst.InfraState)
	assert.Equal(t, SessionStateNone, inst.SessionState)
}

func TestMigrateV2toV3_UnknownState(t *testing.T) {
	store := newTestStore(t)
	v2 := &StateFile{
		Version: 2,
		Instances: []WorkspaceInstance{
			{Name: "unknown", Type: WorkspaceTypeLocal, State: WorkspaceStateUnknown},
		},
	}
	require.NoError(t, store.Save(v2))

	loaded, err := store.Load()
	require.NoError(t, err)
	inst := loaded.Instances[0]
	assert.Nil(t, inst.InfraState)
	assert.Equal(t, SessionStateNone, inst.SessionState)
}

func TestMigrateV2toV3_AvailableState(t *testing.T) {
	store := newTestStore(t)
	v2 := &StateFile{
		Version: 2,
		Instances: []WorkspaceInstance{
			{Name: "avail", Type: WorkspaceTypeSSH, State: WorkspaceStateAvailable},
		},
	}
	require.NoError(t, store.Save(v2))

	loaded, err := store.Load()
	require.NoError(t, err)
	inst := loaded.Instances[0]
	assert.Nil(t, inst.InfraState)
	assert.Equal(t, SessionStateNone, inst.SessionState)
}

func TestMigrateV2toV3_CreatingState(t *testing.T) {
	store := newTestStore(t)
	v2 := &StateFile{
		Version: 2,
		Instances: []WorkspaceInstance{
			{Name: "creating", Type: WorkspaceTypeK8sDeploy, State: WorkspaceStateCreating},
		},
	}
	require.NoError(t, store.Save(v2))

	loaded, err := store.Load()
	require.NoError(t, err)
	inst := loaded.Instances[0]
	require.NotNil(t, inst.InfraState)
	assert.Equal(t, InfraStateRunning, *inst.InfraState)
	assert.Equal(t, SessionStateNone, inst.SessionState)
}

func TestMigrateV2toV3_V3NotReMigrated(t *testing.T) {
	store := newTestStore(t)
	running := InfraStateRunning
	v3 := &StateFile{
		Version: 3,
		Instances: []WorkspaceInstance{
			{Name: "already-v3", Type: WorkspaceTypeContainer, InfraState: &running, SessionState: SessionStateExists},
		},
	}
	require.NoError(t, store.Save(v3))

	loaded, err := store.Load()
	require.NoError(t, err)
	assert.Equal(t, 3, loaded.Version)
	inst := loaded.Instances[0]
	require.NotNil(t, inst.InfraState)
	assert.Equal(t, InfraStateRunning, *inst.InfraState)
	assert.Equal(t, SessionStateExists, inst.SessionState)
}

func TestMigrateV2toV3_MultipleInstances(t *testing.T) {
	store := newTestStore(t)
	v2 := &StateFile{
		Version: 2,
		Instances: []WorkspaceInstance{
			{Name: "local1", Type: WorkspaceTypeLocal, State: WorkspaceStateRunning},
			{Name: "container1", Type: WorkspaceTypeContainer, State: WorkspaceStateStopped},
			{Name: "compose1", Type: WorkspaceTypeCompose, State: WorkspaceStateRunning},
			{Name: "ssh1", Type: WorkspaceTypeSSH, State: WorkspaceStateRunning},
			{Name: "k8s1", Type: WorkspaceTypeK8sDeploy, State: WorkspaceStateError},
		},
	}
	require.NoError(t, store.Save(v2))

	loaded, err := store.Load()
	require.NoError(t, err)
	assert.Equal(t, 3, loaded.Version)
	require.Len(t, loaded.Instances, 5)

	// local running -> InfraState nil, SessionState exists
	assert.Nil(t, loaded.Instances[0].InfraState)
	assert.Equal(t, SessionStateExists, loaded.Instances[0].SessionState)

	// container stopped -> InfraState stopped, SessionState none
	require.NotNil(t, loaded.Instances[1].InfraState)
	assert.Equal(t, InfraStateStopped, *loaded.Instances[1].InfraState)
	assert.Equal(t, SessionStateNone, loaded.Instances[1].SessionState)

	// compose running -> InfraState running, SessionState none
	require.NotNil(t, loaded.Instances[2].InfraState)
	assert.Equal(t, InfraStateRunning, *loaded.Instances[2].InfraState)
	assert.Equal(t, SessionStateNone, loaded.Instances[2].SessionState)

	// ssh running -> InfraState nil, SessionState exists
	assert.Nil(t, loaded.Instances[3].InfraState)
	assert.Equal(t, SessionStateExists, loaded.Instances[3].SessionState)

	// k8s error -> InfraState error, SessionState none
	require.NotNil(t, loaded.Instances[4].InfraState)
	assert.Equal(t, InfraStateError, *loaded.Instances[4].InfraState)
	assert.Equal(t, SessionStateNone, loaded.Instances[4].SessionState)
}

