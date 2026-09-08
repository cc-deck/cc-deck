package ws

import (
	"context"
	"errors"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingLocalRunner struct {
	commands [][]string
}

func (r *recordingLocalRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.commands = append(r.commands, append([]string{name}, args...))
	return nil, nil
}

func TestLocalWorkspaceEnsureSessionCreatesSharedSession(t *testing.T) {
	// The suite itself is often run from inside a Zellij session, and creating
	// a session is refused there. Say so explicitly rather than depending on
	// where the tests happen to be run.
	t.Setenv("ZELLIJ", "")
	store := newTestStore(t)
	require.NoError(t, store.AddInstance(&WorkspaceInstance{Name: "demo", Type: WorkspaceTypeLocal, SessionState: SessionStateNone}))
	runner := &recordingLocalRunner{}
	workspace := &LocalWorkspace{
		name:          "demo",
		store:         store,
		commandRunner: runner,
		sessionState:  func(string) string { return "" },
	}

	result, err := workspace.EnsureSession(context.Background(), SessionStartOptions{WebSharing: true})
	require.NoError(t, err)
	require.True(t, result.Created)
	require.Equal(t, "cc-deck-demo", result.Name)
	require.True(t, reflect.DeepEqual([][]string{{"zellij", "--layout", "cc-deck", "attach", "-b", "cc-deck-demo", "options", "--web-sharing", "on"}}, runner.commands))
}

func TestLocalWorkspaceEnsureSessionIsIdempotent(t *testing.T) {
	runner := &recordingLocalRunner{}
	workspace := &LocalWorkspace{
		name:          "demo",
		store:         newTestStore(t),
		commandRunner: runner,
		sessionState:  func(string) string { return "running" },
	}

	result, err := workspace.EnsureSession(context.Background(), SessionStartOptions{WebSharing: true})
	require.NoError(t, err)
	require.False(t, result.Created)
	require.Equal(t, "cc-deck-demo", result.Name)
	require.Empty(t, runner.commands)
}

// Inside an existing Zellij session, the session-creating command adds a tab to
// the current session, exits zero, and creates nothing. Reporting success for
// that is the defect: every later step then acts on a session that never
// existed. Attach already refuses for the same reason.
func TestLocalWorkspaceEnsureSessionRefusesInsideAZellijSession(t *testing.T) {
	t.Setenv("ZELLIJ", "0")
	runner := &recordingLocalRunner{}
	workspace := &LocalWorkspace{
		name:          "demo",
		store:         newTestStore(t),
		commandRunner: runner,
		sessionState:  func(string) string { return "" },
	}

	result, err := workspace.EnsureSession(context.Background(), SessionStartOptions{})

	require.Error(t, err, "creating a session from inside Zellij must fail rather than silently add a tab")
	require.ErrorContains(t, err, "inside a Zellij session")
	require.ErrorContains(t, err, "Detach first")
	require.False(t, result.Created)
	require.Empty(t, runner.commands, "the command that would add a tab must never run")
}

// An already-running session is still reported, because nothing needs creating.
func TestLocalWorkspaceEnsureSessionStillReportsARunningSessionInsideZellij(t *testing.T) {
	t.Setenv("ZELLIJ", "0")
	workspace := &LocalWorkspace{
		name:          "demo",
		store:         newTestStore(t),
		commandRunner: &recordingLocalRunner{},
		sessionState:  func(string) string { return "running" },
	}

	result, err := workspace.EnsureSession(context.Background(), SessionStartOptions{})
	require.NoError(t, err)
	require.Equal(t, "cc-deck-demo", result.Name)
	require.False(t, result.Created)
}

// newTestStore is defined in state_test.go and shared across test files.

func TestLocalWorkspace_Type(t *testing.T) {
	store := newTestStore(t)
	env := &LocalWorkspace{name: "test", store: store}
	assert.Equal(t, WorkspaceTypeLocal, env.Type())
}

func TestLocalWorkspace_Name(t *testing.T) {
	store := newTestStore(t)
	env := &LocalWorkspace{name: "my-project", store: store}
	assert.Equal(t, "my-project", env.Name())
}

func TestLocalWorkspace_CreateAddsInstance(t *testing.T) {
	if _, err := exec.LookPath("zellij"); err != nil {
		t.Skip("zellij not found in PATH, skipping")
	}

	store := newTestStore(t)
	env := &LocalWorkspace{name: "test-env", store: store}

	err := env.Create(context.Background(), CreateOpts{})
	require.NoError(t, err)

	inst, err := store.FindInstanceByName("test-env")
	require.NoError(t, err)
	assert.Equal(t, WorkspaceTypeLocal, inst.Type)
	assert.Equal(t, SessionStateNone, inst.SessionState)
	assert.Nil(t, inst.InfraState)
}

func TestLocalWorkspace_CreateRejectsDuplicate(t *testing.T) {
	if _, err := exec.LookPath("zellij"); err != nil {
		t.Skip("zellij not found in PATH, skipping")
	}

	store := newTestStore(t)
	env := &LocalWorkspace{name: "dup-env", store: store}

	err := env.Create(context.Background(), CreateOpts{})
	require.NoError(t, err)

	err = env.Create(context.Background(), CreateOpts{})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNameConflict))
}

func TestLocalWorkspace_CreateRejectsInvalidName(t *testing.T) {
	store := newTestStore(t)
	env := &LocalWorkspace{name: "INVALID", store: store}

	err := env.Create(context.Background(), CreateOpts{})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidName))
}

func TestLocalWorkspace_ExecReturnsNotSupported(t *testing.T) {
	store := newTestStore(t)
	env := &LocalWorkspace{name: "test", store: store}

	err := env.Exec(context.Background(), []string{"ls"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotSupported))
}

func TestLocalWorkspace_DeleteRemovesInstance(t *testing.T) {
	store := newTestStore(t)

	// Manually add an instance (bypassing Create to avoid zellij dependency).
	inst := &WorkspaceInstance{
		Name:         "del-env",
		Type:         WorkspaceTypeLocal,
		SessionState: SessionStateNone,
	}
	require.NoError(t, store.AddInstance(inst))

	env := &LocalWorkspace{name: "del-env", store: store}
	err := env.Delete(context.Background(), true)
	require.NoError(t, err)

	_, err = store.FindInstanceByName("del-env")
	assert.True(t, errors.Is(err, ErrNotFound))
}

func TestNewWorkspace_Local(t *testing.T) {
	store := newTestStore(t)
	env, err := NewWorkspace(WorkspaceTypeLocal, "test", store, nil)
	require.NoError(t, err)

	assert.Equal(t, WorkspaceTypeLocal, env.Type())
	assert.Equal(t, "test", env.Name())
}

func TestNewWorkspace_Container(t *testing.T) {
	store := newTestStore(t)
	env, err := NewWorkspace(WorkspaceTypeContainer, "test", store, nil)
	require.NoError(t, err)

	assert.Equal(t, WorkspaceTypeContainer, env.Type())
	assert.Equal(t, "test", env.Name())
}

func TestNewWorkspace_UnimplementedType(t *testing.T) {
	store := newTestStore(t)
	_, err := NewWorkspace("k8s-sandbox", "test", store, nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotImplemented))
}

func TestNewWorkspace_K8sDeploy(t *testing.T) {
	store := newTestStore(t)
	e, err := NewWorkspace("k8s-deploy", "test", store, nil)
	require.NoError(t, err)
	assert.Equal(t, WorkspaceTypeK8sDeploy, e.Type())
	assert.Equal(t, "test", e.Name())
}

// A wedged Zellij server used to hang a cc-deck command indefinitely, which
// reports nothing at all. It must fail instead, and say what to do about it.
func TestRunBoundedZellijFailsRatherThanHanging(t *testing.T) {
	start := time.Now()
	_, err := runBoundedZellij(context.Background(), 150*time.Millisecond, "sleep", "30")
	elapsed := time.Since(start)

	if !errors.Is(err, ErrZellijUnresponsive) {
		t.Fatalf("expected ErrZellijUnresponsive, got %v", err)
	}
	if elapsed > 5*time.Second {
		t.Errorf("took %s; the bound was not applied", elapsed)
	}
	if !strings.Contains(err.Error(), "delete-session --force") {
		t.Errorf("the error must name the recovery, got %q", err)
	}
}

// A caller that has already set its own deadline keeps it.
func TestRunBoundedZellijKeepsACallerDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := runBoundedZellij(ctx, time.Hour, "sleep", "30")

	if !errors.Is(err, ErrZellijUnresponsive) {
		t.Fatalf("expected ErrZellijUnresponsive, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("took %s; the caller's shorter deadline was ignored", elapsed)
	}
}

// An ordinary command failure is not a hang, and must not be dressed up as one.
func TestRunBoundedZellijPassesThroughRealErrors(t *testing.T) {
	_, err := runBoundedZellij(context.Background(), 10*time.Second, "false")

	if err == nil {
		t.Fatal("expected the command's own failure")
	}
	if errors.Is(err, ErrZellijUnresponsive) {
		t.Errorf("a non-timeout failure must not be reported as unresponsive: %v", err)
	}
}

func TestRunBoundedZellijSucceedsNormally(t *testing.T) {
	out, err := runBoundedZellij(context.Background(), 10*time.Second, "echo", "ok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(out), "ok") {
		t.Errorf("got %q", out)
	}
}
