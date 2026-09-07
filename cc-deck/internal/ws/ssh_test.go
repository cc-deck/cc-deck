package ws

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

func TestSSHWorkspace_TypeAndName(t *testing.T) {
	e := &SSHWorkspace{name: "test-ssh"}
	if e.Type() != WorkspaceTypeSSH {
		t.Errorf("Type() = %q, want %q", e.Type(), WorkspaceTypeSSH)
	}
	if e.Name() != "test-ssh" {
		t.Errorf("Name() = %q, want %q", e.Name(), "test-ssh")
	}
}

func TestSSHWorkspace_SessionName(t *testing.T) {
	e := &SSHWorkspace{name: "myenv"}
	got := e.sshSessionName()
	want := "cc-deck-myenv"
	if got != want {
		t.Errorf("sshSessionName() = %q, want %q", got, want)
	}
}

func TestSSHWorkspace_KillSessionNoInstance(t *testing.T) {
	store := newTestStore(t)
	e := &SSHWorkspace{name: "test", store: store}
	if err := e.KillSession(nil); err != nil {
		t.Errorf("KillSession() error = %v, want nil", err)
	}
}

func TestSSHWorkspace_CreateNameValidation(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state.yaml")
	store := NewStateStore(stateFile)

	e := &SSHWorkspace{name: "INVALID!", store: store}
	err := e.Create(nil, CreateOpts{})
	if err == nil {
		t.Fatal("expected error for invalid name, got nil")
	}
}

func TestSSHWorkspace_CreateConflict(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state.yaml")
	store := NewStateStore(stateFile)

	// Pre-populate with an existing instance.
	_ = store.AddInstance(&WorkspaceInstance{
		Name: "existing",
		Type: WorkspaceTypeSSH,
	})

	e := &SSHWorkspace{name: "existing", store: store}
	err := e.Create(nil, CreateOpts{})
	if err == nil {
		t.Fatal("expected ErrNameConflict, got nil")
	}
}

func TestSSHWorkspace_DeleteNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state.yaml")
	store := NewStateStore(stateFile)

	e := &SSHWorkspace{name: "nonexistent", store: store}
	err := e.Delete(nil, false)
	if err == nil {
		t.Fatal("expected error for nonexistent workspace, got nil")
	}
}

func TestSSHWorkspace_DeleteRemovesDefinition(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state.yaml")
	store := NewStateStore(stateFile)

	defFile := filepath.Join(tmpDir, "defs.yaml")
	defs := NewDefinitionStore(defFile)

	// Add a definition and an instance.
	require.NoError(t, defs.Add(&WorkspaceDefinition{
		Name: "ssh-env",
		Type: WorkspaceTypeSSH,
		WorkspaceSpec: WorkspaceSpec{
			Host: "user@host",
		},
	}))
	require.NoError(t, store.AddInstance(&WorkspaceInstance{
		Name:  "ssh-env",
		Type:  WorkspaceTypeSSH,
		SessionState: SessionStateNone,
		SSH:   &SSHFields{Host: "user@host"},
	}))

	e := &SSHWorkspace{name: "ssh-env", store: store, defs: defs}
	err := e.Delete(context.Background(), true)
	require.NoError(t, err)

	// Verify definition was removed.
	_, findErr := defs.FindByName("ssh-env")
	assert.Error(t, findErr, "definition should be removed after delete")

	// Verify instance was removed.
	_, instErr := store.FindInstanceByName("ssh-env")
	assert.Error(t, instErr, "instance should be removed after delete")
}

func TestSSHWorkspace_DeleteSucceedsWhenDefRemovalFails(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state.yaml")
	store := NewStateStore(stateFile)

	defFile := filepath.Join(tmpDir, "defs.yaml")
	defs := NewDefinitionStore(defFile)

	// Add instance but no definition (simulates already-removed definition).
	require.NoError(t, store.AddInstance(&WorkspaceInstance{
		Name:  "ssh-env",
		Type:  WorkspaceTypeSSH,
		SessionState: SessionStateNone,
		SSH:   &SSHFields{Host: "user@host"},
	}))

	e := &SSHWorkspace{name: "ssh-env", store: store, defs: defs}
	err := e.Delete(context.Background(), true)
	assert.NoError(t, err, "delete should succeed even when definition removal fails")
}

func TestSSHWorkspace_DeleteRefusesRunning(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state.yaml")
	store := NewStateStore(stateFile)

	_ = store.AddInstance(&WorkspaceInstance{
		Name:         "running-env",
		Type:         WorkspaceTypeSSH,
		SessionState: SessionStateExists,
		SSH: &SSHFields{
			Host: "user@host",
		},
	})

	e := &SSHWorkspace{name: "running-env", store: store}
	err := e.Delete(nil, false)
	if err != ErrRunning {
		t.Errorf("Delete(force=false) error = %v, want ErrRunning", err)
	}
}

func TestSSHWorkspace_AttachInsideZellij(t *testing.T) {
	// Simulate running inside Zellij.
	t.Setenv("ZELLIJ", "1")

	e := &SSHWorkspace{name: "test"}
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
		def := &WorkspaceDefinition{WorkspaceSpec: WorkspaceSpec{Workspace: tt.workspace}}
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

func TestSSHWorkspace_ExecRequiresDefinition(t *testing.T) {
	e := &SSHWorkspace{name: "test", defs: nil}
	err := e.Exec(nil, []string{"echo", "hello"})
	if err == nil {
		t.Fatal("expected error when no definition store, got nil")
	}
}

func TestSSHWorkspace_PushRequiresLocalPath(t *testing.T) {
	tmpDir := t.TempDir()
	defFile := filepath.Join(tmpDir, "defs.yaml")
	os.WriteFile(defFile, []byte("version: 1\nworkspaces:\n- name: test\n  type: ssh\n  host: user@host\n"), 0o644)
	defs := NewDefinitionStore(defFile)

	e := &SSHWorkspace{name: "test", defs: defs}
	err := e.Push(nil, SyncOpts{})
	if err == nil {
		t.Fatal("expected error for missing local path, got nil")
	}
}

func TestSSHWorkspace_PullRequiresRemotePath(t *testing.T) {
	tmpDir := t.TempDir()
	defFile := filepath.Join(tmpDir, "defs.yaml")
	os.WriteFile(defFile, []byte("version: 1\nworkspaces:\n- name: test\n  type: ssh\n  host: user@host\n"), 0o644)
	defs := NewDefinitionStore(defFile)

	e := &SSHWorkspace{name: "test", defs: defs}
	err := e.Pull(nil, SyncOpts{})
	if err == nil {
		t.Fatal("expected error for missing remote path, got nil")
	}
}
