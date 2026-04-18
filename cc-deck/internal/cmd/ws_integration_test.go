package cmd_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cc-deck/cc-deck/internal/cmd"
)

// buildRootCmd constructs a CLI tree identical to the real one but limited
// to the ws subcommand. This lets us exercise the full cobra path without
// pulling in Kubernetes or other heavy dependencies.
func buildRootCmd(gf *cmd.GlobalFlags) *cobra.Command {
	root := &cobra.Command{Use: "cc-deck", SilenceUsage: true}
	root.PersistentFlags().StringVarP(&gf.Output, "output", "o", "text", "Output format")
	root.AddCommand(cmd.NewWsCmd(gf))
	return root
}

// setupTestWs creates a temp state file and puts a dummy zellij script
// on PATH so that LookPath succeeds. Each test gets its own isolated
// state file and definitions file via environment variables.
func setupTestWs(t *testing.T) (stateDir string) {
	t.Helper()

	stateDir = t.TempDir()
	stateFile := filepath.Join(stateDir, "state.yaml")
	t.Setenv("CC_DECK_STATE_FILE", stateFile)

	// Isolate definition store so system definitions don't leak into tests.
	defsFile := filepath.Join(stateDir, "environments.yaml")
	t.Setenv("CC_DECK_DEFINITIONS_FILE", defsFile)

	// Change to temp dir so project-local .cc-deck/ config isn't discovered.
	t.Chdir(stateDir)

	// Create a dummy zellij binary so exec.LookPath("zellij") succeeds.
	binDir := filepath.Join(t.TempDir(), "bin")
	require.NoError(t, os.MkdirAll(binDir, 0o755))

	zellijStub := filepath.Join(binDir, "zellij")
	// The stub handles list-sessions (returns empty) and kill-session (no-op).
	script := "#!/bin/sh\nexit 0\n"
	require.NoError(t, os.WriteFile(zellijStub, []byte(script), 0o755))

	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))

	return stateDir
}

// run executes a command line against the test root and returns stdout,
// stderr, and any error.
func run(t *testing.T, gf *cmd.GlobalFlags, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	root := buildRootCmd(gf)

	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	root.SetOut(outBuf)
	root.SetErr(errBuf)
	root.SetArgs(args)

	// Capture os.Stdout since the ws commands write directly to it.
	origStdout := os.Stdout
	origStderr := os.Stderr
	rOut, wOut, pipeErr := os.Pipe()
	require.NoError(t, pipeErr, "failed to create stdout pipe")
	rErr, wErr, pipeErr2 := os.Pipe()
	require.NoError(t, pipeErr2, "failed to create stderr pipe")
	os.Stdout = wOut
	os.Stderr = wErr

	err = root.Execute()

	wOut.Close()
	wErr.Close()
	os.Stdout = origStdout
	os.Stderr = origStderr

	var pipedOut, pipedErr bytes.Buffer
	pipedOut.ReadFrom(rOut)
	pipedErr.ReadFrom(rErr)
	rOut.Close()
	rErr.Close()

	// Combine cobra output and piped stdout.
	combined := outBuf.String() + pipedOut.String()
	combinedErr := errBuf.String() + pipedErr.String()

	return combined, combinedErr, err
}

// --- Integration tests ---

func TestWsCreateAndList(t *testing.T) {
	setupTestWs(t)
	gf := &cmd.GlobalFlags{Output: "text"}

	// Create an environment.
	stdout, _, err := run(t, gf, "ws", "new", "mydev", "--type", "local")
	require.NoError(t, err)
	assert.Contains(t, stdout, `"mydev" created`)

	// List should show it.
	stdout, _, err = run(t, gf, "ws", "list")
	require.NoError(t, err)
	assert.Contains(t, stdout, "mydev")
	assert.Contains(t, stdout, "local")
}

func TestWsCreateAndListJSON(t *testing.T) {
	setupTestWs(t)
	gf := &cmd.GlobalFlags{Output: "json"}

	_, _, err := run(t, gf, "ws", "new", "jsontest", "--type", "local")
	require.NoError(t, err)

	stdout, _, err := run(t, gf, "ws", "list", "-o", "json")
	require.NoError(t, err)

	var entries []map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(stdout), &entries), "output should be valid JSON: %s", stdout)
	require.Len(t, entries, 1)
	assert.Equal(t, "jsontest", entries[0]["name"])
	assert.Equal(t, "local", entries[0]["type"])
}

func TestWsListEmpty(t *testing.T) {
	setupTestWs(t)
	gf := &cmd.GlobalFlags{Output: "text"}

	stdout, _, err := run(t, gf, "ws", "list")
	require.NoError(t, err)
	assert.Contains(t, stdout, "No workspaces found")
}

func TestWsListFilterByType(t *testing.T) {
	setupTestWs(t)
	gf := &cmd.GlobalFlags{Output: "text"}

	_, _, err := run(t, gf, "ws", "new", "localenv", "--type", "local")
	require.NoError(t, err)

	// Filter for local should find it.
	stdout, _, err := run(t, gf, "ws", "list", "--type", "local")
	require.NoError(t, err)
	assert.Contains(t, stdout, "localenv")

	// Filter for container should not find it.
	stdout, _, err = run(t, gf, "ws", "list", "--type", "container")
	require.NoError(t, err)
	assert.NotContains(t, stdout, "localenv")
}

func TestWsCreateInvalidName(t *testing.T) {
	setupTestWs(t)
	gf := &cmd.GlobalFlags{Output: "text"}

	_, _, err := run(t, gf, "ws", "new", "INVALID", "--type", "local")
	require.Error(t, err)
}

func TestWsCreateDuplicateName(t *testing.T) {
	setupTestWs(t)
	gf := &cmd.GlobalFlags{Output: "text"}

	_, _, err := run(t, gf, "ws", "new", "duptest", "--type", "local")
	require.NoError(t, err)

	_, _, err = run(t, gf, "ws", "new", "duptest", "--type", "local")
	require.Error(t, err)
}

func TestWsCreateUnsupportedType(t *testing.T) {
	setupTestWs(t)
	gf := &cmd.GlobalFlags{Output: "text"}

	_, _, err := run(t, gf, "ws", "new", "badtype", "--type", "k8s-sandbox")
	require.Error(t, err)
}

func TestWsStatus(t *testing.T) {
	setupTestWs(t)
	gf := &cmd.GlobalFlags{Output: "text"}

	_, _, err := run(t, gf, "ws", "new", "statustest", "--type", "local")
	require.NoError(t, err)

	stdout, _, err := run(t, gf, "ws", "status", "statustest")
	require.NoError(t, err)
	assert.Contains(t, stdout, "statustest")
	assert.Contains(t, stdout, "local")
}

func TestWsStatusJSON(t *testing.T) {
	setupTestWs(t)
	gf := &cmd.GlobalFlags{Output: "json"}

	_, _, err := run(t, gf, "ws", "new", "statusjson", "--type", "local")
	require.NoError(t, err)

	stdout, _, err := run(t, gf, "ws", "status", "-o", "json", "statusjson")
	require.NoError(t, err)

	var out map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "output should be valid JSON: %s", stdout)
	assert.Equal(t, "statusjson", out["name"])
	assert.Equal(t, "local", out["type"])
}

func TestWsStatusNotFound(t *testing.T) {
	setupTestWs(t)
	gf := &cmd.GlobalFlags{Output: "text"}

	_, _, err := run(t, gf, "ws", "status", "nonexistent")
	require.Error(t, err)
}

func TestWsDelete(t *testing.T) {
	setupTestWs(t)
	gf := &cmd.GlobalFlags{Output: "text"}

	_, _, err := run(t, gf, "ws", "new", "deltest", "--type", "local")
	require.NoError(t, err)

	// Delete with --force (zellij stub returns no sessions, but force is clean).
	stdout, _, err := run(t, gf, "ws", "kill", "deltest", "--force")
	require.NoError(t, err)
	assert.Contains(t, stdout, `"deltest" deleted`)
}

func TestWsDeleteNotFound(t *testing.T) {
	setupTestWs(t)
	gf := &cmd.GlobalFlags{Output: "text"}

	_, _, err := run(t, gf, "ws", "kill", "ghost", "--force")
	require.Error(t, err)
}

func TestWsStopLocal(t *testing.T) {
	setupTestWs(t)
	gf := &cmd.GlobalFlags{Output: "text"}

	_, _, err := run(t, gf, "ws", "new", "stoptest", "--type", "local")
	require.NoError(t, err)

	// Stop calls zellij kill-session; the stub zellij exits 0 but the
	// session doesn't actually exist, so Stop reports "not running".
	_, _, err = run(t, gf, "ws", "stop", "stoptest")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not running")
}

func TestWsStartLocal(t *testing.T) {
	setupTestWs(t)
	gf := &cmd.GlobalFlags{Output: "text"}

	_, _, err := run(t, gf, "ws", "new", "starttest", "--type", "local")
	require.NoError(t, err)

	// Start should succeed (zellij stub creates session, exits 0).
	stdout, _, err := run(t, gf, "ws", "start", "starttest")
	require.NoError(t, err)
	assert.Contains(t, stdout, "started")
}

func TestWsStubCommandsReturnError(t *testing.T) {
	setupTestWs(t)
	gf := &cmd.GlobalFlags{Output: "text"}

	// Create the environment first so resolveEnvironment succeeds.
	_, _, err := run(t, gf, "ws", "new", "stubtest", "--type", "local")
	require.NoError(t, err)

	// These commands should fail because local environments don't support them.
	stubs := []struct {
		args    []string
		errText string
	}{
		{[]string{"ws", "exec", "stubtest", "--", "echo"}, "not supported"},
		{[]string{"ws", "push", "stubtest"}, "not supported"},
		{[]string{"ws", "pull", "stubtest"}, "not supported"},
		{[]string{"ws", "harvest", "stubtest"}, "not supported"},
		{[]string{"ws", "logs", "stubtest"}, "not yet implemented"},
	}

	for _, tt := range stubs {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			_, _, err := run(t, gf, tt.args...)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errText)
		})
	}
}

func TestWsFullLifecycle(t *testing.T) {
	setupTestWs(t)
	gf := &cmd.GlobalFlags{Output: "text"}

	// 1. Create
	stdout, _, err := run(t, gf, "ws", "new", "lifecycle", "--type", "local")
	require.NoError(t, err)
	assert.Contains(t, stdout, "created")

	// 2. List shows it
	stdout, _, err = run(t, gf, "ws", "list")
	require.NoError(t, err)
	assert.Contains(t, stdout, "lifecycle")

	// 3. Status works
	stdout, _, err = run(t, gf, "ws", "status", "lifecycle")
	require.NoError(t, err)
	assert.Contains(t, stdout, "lifecycle")

	// 4. Delete
	stdout, _, err = run(t, gf, "ws", "kill", "lifecycle", "--force")
	require.NoError(t, err)
	assert.Contains(t, stdout, "deleted")
}

func TestWsMultipleEnvironments(t *testing.T) {
	setupTestWs(t)
	gf := &cmd.GlobalFlags{Output: "text"}

	names := []string{"alpha", "beta", "gamma"}
	for _, name := range names {
		_, _, err := run(t, gf, "ws", "new", name, "--type", "local")
		require.NoError(t, err)
	}

	stdout, _, err := run(t, gf, "ws", "list")
	require.NoError(t, err)
	for _, name := range names {
		assert.Contains(t, stdout, name)
	}

	// Delete one and verify
	_, _, err = run(t, gf, "ws", "kill", "beta", "--force")
	require.NoError(t, err)

	stdout, _, err = run(t, gf, "ws", "list")
	require.NoError(t, err)
	assert.Contains(t, stdout, "alpha")
	assert.NotContains(t, stdout, "beta")
	assert.Contains(t, stdout, "gamma")
}

func TestWsCreateDefaultTypeIsLocal(t *testing.T) {
	setupTestWs(t)
	gf := &cmd.GlobalFlags{Output: "json"}

	// Create without --type flag should default to local.
	_, _, err := run(t, gf, "ws", "new", "defaulttype")
	require.NoError(t, err)

	stdout, _, err := run(t, gf, "ws", "list", "-o", "json")
	require.NoError(t, err)

	var entries []map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(stdout), &entries))
	require.Len(t, entries, 1)
	assert.Equal(t, "local", entries[0]["type"])
}

func TestWsStatePersistence(t *testing.T) {
	stateDir := setupTestWs(t)
	gf := &cmd.GlobalFlags{Output: "text"}

	_, _, err := run(t, gf, "ws", "new", "persist", "--type", "local")
	require.NoError(t, err)

	// Verify the state file was actually written to disk.
	stateFile := filepath.Join(stateDir, "state.yaml")
	data, err := os.ReadFile(stateFile)
	require.NoError(t, err, "state file should exist on disk")
	assert.Contains(t, string(data), "persist")
	assert.Contains(t, string(data), "local")
}
