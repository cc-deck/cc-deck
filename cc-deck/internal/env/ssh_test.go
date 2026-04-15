package env

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSSHRunner implements ssh.Runner for testing workspace resolution.
type mockSSHRunner struct {
	handler func(cmd string) (string, error)
}

func (m *mockSSHRunner) Run(_ context.Context, cmd string) (string, error) {
	return m.handler(cmd)
}

func TestSSHEnvironment_TypeAndName(t *testing.T) {
	e := &SSHEnvironment{name: "test-ssh"}
	if e.Type() != EnvironmentTypeSSH {
		t.Errorf("Type() = %q, want %q", e.Type(), EnvironmentTypeSSH)
	}
	if e.Name() != "test-ssh" {
		t.Errorf("Name() = %q, want %q", e.Name(), "test-ssh")
	}
}

func TestSSHEnvironment_SessionName(t *testing.T) {
	e := &SSHEnvironment{name: "myenv"}
	got := e.sshSessionName()
	want := "cc-deck-myenv"
	if got != want {
		t.Errorf("sshSessionName() = %q, want %q", got, want)
	}
}

func TestSSHEnvironment_StartStop(t *testing.T) {
	e := &SSHEnvironment{name: "test"}
	if err := e.Start(nil); err != ErrNotSupported {
		t.Errorf("Start() error = %v, want ErrNotSupported", err)
	}
	if err := e.Stop(nil); err != ErrNotSupported {
		t.Errorf("Stop() error = %v, want ErrNotSupported", err)
	}
}

func TestSSHEnvironment_CreateNameValidation(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state.yaml")
	store := NewStateStore(stateFile)

	e := &SSHEnvironment{name: "INVALID!", store: store}
	err := e.Create(nil, CreateOpts{})
	if err == nil {
		t.Fatal("expected error for invalid name, got nil")
	}
}

func TestSSHEnvironment_CreateConflict(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state.yaml")
	store := NewStateStore(stateFile)

	// Pre-populate with an existing instance.
	_ = store.AddInstance(&EnvironmentInstance{
		Name: "existing",
		Type: EnvironmentTypeSSH,
	})

	e := &SSHEnvironment{name: "existing", store: store}
	err := e.Create(nil, CreateOpts{})
	if err == nil {
		t.Fatal("expected ErrNameConflict, got nil")
	}
}

func TestSSHEnvironment_DeleteNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state.yaml")
	store := NewStateStore(stateFile)

	e := &SSHEnvironment{name: "nonexistent", store: store}
	err := e.Delete(nil, false)
	if err == nil {
		t.Fatal("expected error for nonexistent environment, got nil")
	}
}

func TestSSHEnvironment_DeleteRemovesDefinition(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state.yaml")
	store := NewStateStore(stateFile)

	defFile := filepath.Join(tmpDir, "defs.yaml")
	defs := NewDefinitionStore(defFile)

	// Add a definition and an instance.
	require.NoError(t, defs.Add(&EnvironmentDefinition{
		Name: "ssh-env",
		Type: EnvironmentTypeSSH,
		Host: "user@host",
	}))
	require.NoError(t, store.AddInstance(&EnvironmentInstance{
		Name:  "ssh-env",
		Type:  EnvironmentTypeSSH,
		State: EnvironmentStateStopped,
		SSH:   &SSHFields{Host: "user@host"},
	}))

	e := &SSHEnvironment{name: "ssh-env", store: store, defs: defs}
	err := e.Delete(context.Background(), true)
	require.NoError(t, err)

	// Verify definition was removed.
	_, findErr := defs.FindByName("ssh-env")
	assert.Error(t, findErr, "definition should be removed after delete")

	// Verify instance was removed.
	_, instErr := store.FindInstanceByName("ssh-env")
	assert.Error(t, instErr, "instance should be removed after delete")
}

func TestSSHEnvironment_DeleteSucceedsWhenDefRemovalFails(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state.yaml")
	store := NewStateStore(stateFile)

	defFile := filepath.Join(tmpDir, "defs.yaml")
	defs := NewDefinitionStore(defFile)

	// Add instance but no definition (simulates already-removed definition).
	require.NoError(t, store.AddInstance(&EnvironmentInstance{
		Name:  "ssh-env",
		Type:  EnvironmentTypeSSH,
		State: EnvironmentStateStopped,
		SSH:   &SSHFields{Host: "user@host"},
	}))

	e := &SSHEnvironment{name: "ssh-env", store: store, defs: defs}
	err := e.Delete(context.Background(), true)
	assert.NoError(t, err, "delete should succeed even when definition removal fails")
}

func TestSSHEnvironment_DeleteRefusesRunning(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state.yaml")
	store := NewStateStore(stateFile)

	_ = store.AddInstance(&EnvironmentInstance{
		Name:  "running-env",
		Type:  EnvironmentTypeSSH,
		State: EnvironmentStateRunning,
		SSH: &SSHFields{
			Host: "user@host",
		},
	})

	e := &SSHEnvironment{name: "running-env", store: store}
	err := e.Delete(nil, false)
	if err != ErrRunning {
		t.Errorf("Delete(force=false) error = %v, want ErrRunning", err)
	}
}

func TestSSHEnvironment_AttachInsideZellij(t *testing.T) {
	// Simulate running inside Zellij.
	t.Setenv("ZELLIJ", "1")

	e := &SSHEnvironment{name: "test"}
	err := e.Attach(nil)
	if err != nil {
		t.Errorf("Attach() inside Zellij should return nil, got %v", err)
	}
}

func TestWorkspacePath(t *testing.T) {
	tests := []struct {
		workspace string
		want      string
	}{
		{"", "~/workspace"},
		{"/home/user/projects", "/home/user/projects"},
		{"~/custom", "~/custom"},
	}

	for _, tt := range tests {
		def := &EnvironmentDefinition{Workspace: tt.workspace}
		got := workspacePath(def)
		if got != tt.want {
			t.Errorf("workspacePath(%q) = %q, want %q", tt.workspace, got, tt.want)
		}
	}
}

func TestResolveWorkspaceRemote(t *testing.T) {
	runner := &mockSSHRunner{
		handler: func(cmd string) (string, error) {
			// Simulate remote shell expanding ~/workspace to /home/dev/workspace.
			if strings.Contains(cmd, "~/workspace") {
				return "/home/dev/workspace", nil
			}
			if strings.Contains(cmd, "/abs/path") {
				return "/abs/path", nil
			}
			return "", fmt.Errorf("unexpected command: %s", cmd)
		},
	}

	ws, err := resolveWorkspaceRemote(context.Background(), runner, "~/workspace")
	require.NoError(t, err)
	assert.Equal(t, "/home/dev/workspace", ws)

	ws, err = resolveWorkspaceRemote(context.Background(), runner, "/abs/path")
	require.NoError(t, err)
	assert.Equal(t, "/abs/path", ws)
}

func TestSSHEnvironment_ExecRequiresDefinition(t *testing.T) {
	e := &SSHEnvironment{name: "test", defs: nil}
	err := e.Exec(nil, []string{"echo", "hello"})
	if err == nil {
		t.Fatal("expected error when no definition store, got nil")
	}
}

func TestSSHEnvironment_PushRequiresLocalPath(t *testing.T) {
	tmpDir := t.TempDir()
	defFile := filepath.Join(tmpDir, "defs.yaml")
	os.WriteFile(defFile, []byte("version: 1\nenvironments:\n- name: test\n  type: ssh\n  host: user@host\n"), 0o644)
	defs := NewDefinitionStore(defFile)

	e := &SSHEnvironment{name: "test", defs: defs}
	err := e.Push(nil, SyncOpts{})
	if err == nil {
		t.Fatal("expected error for missing local path, got nil")
	}
}

func TestSSHEnvironment_PullRequiresRemotePath(t *testing.T) {
	tmpDir := t.TempDir()
	defFile := filepath.Join(tmpDir, "defs.yaml")
	os.WriteFile(defFile, []byte("version: 1\nenvironments:\n- name: test\n  type: ssh\n  host: user@host\n"), 0o644)
	defs := NewDefinitionStore(defFile)

	e := &SSHEnvironment{name: "test", defs: defs}
	err := e.Pull(nil, SyncOpts{})
	if err == nil {
		t.Fatal("expected error for missing remote path, got nil")
	}
}
