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
	"github.com/cc-deck/cc-deck/internal/env"
)

// buildRootCmd constructs a CLI tree identical to the real one but limited
// to the env subcommand. This lets us exercise the full cobra path without
// pulling in Kubernetes or other heavy dependencies.
func buildRootCmd(gf *cmd.GlobalFlags) *cobra.Command {
	root := &cobra.Command{Use: "cc-deck", SilenceUsage: true}
	root.PersistentFlags().StringVarP(&gf.Output, "output", "o", "text", "Output format")
	root.AddCommand(cmd.NewEnvCmd(gf))
	return root
}

// setupTestEnv creates a temp state file and puts a dummy zellij script
// on PATH so that LookPath succeeds. Each test gets its own isolated
// state file via CC_DECK_STATE_FILE.
func setupTestEnv(t *testing.T) (stateDir string) {
	t.Helper()

	stateDir = t.TempDir()
	stateFile := filepath.Join(stateDir, "state.yaml")
	t.Setenv("CC_DECK_STATE_FILE", stateFile)

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

	// Capture os.Stdout since the env commands write directly to it.
	origStdout := os.Stdout
	origStderr := os.Stderr
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
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

	// Combine cobra output and piped stdout.
	combined := outBuf.String() + pipedOut.String()
	combinedErr := errBuf.String() + pipedErr.String()

	return combined, combinedErr, err
}

// --- Integration tests ---

func TestEnvCreateAndList(t *testing.T) {
	setupTestEnv(t)
	gf := &cmd.GlobalFlags{Output: "text"}

	// Create an environment.
	stdout, _, err := run(t, gf, "env", "create", "mydev", "--type", "local")
	require.NoError(t, err)
	assert.Contains(t, stdout, `"mydev" created`)

	// List should show it.
	stdout, _, err = run(t, gf, "env", "list")
	require.NoError(t, err)
	assert.Contains(t, stdout, "mydev")
	assert.Contains(t, stdout, "local")
}

func TestEnvCreateAndListJSON(t *testing.T) {
	setupTestEnv(t)
	gf := &cmd.GlobalFlags{Output: "json"}

	_, _, err := run(t, gf, "env", "create", "jsontest", "--type", "local")
	require.NoError(t, err)

	stdout, _, err := run(t, gf, "env", "list", "-o", "json")
	require.NoError(t, err)

	var records []env.EnvironmentRecord
	require.NoError(t, json.Unmarshal([]byte(stdout), &records), "output should be valid JSON: %s", stdout)
	require.Len(t, records, 1)
	assert.Equal(t, "jsontest", records[0].Name)
	assert.Equal(t, env.EnvironmentTypeLocal, records[0].Type)
}

func TestEnvListEmpty(t *testing.T) {
	setupTestEnv(t)
	gf := &cmd.GlobalFlags{Output: "text"}

	stdout, _, err := run(t, gf, "env", "list")
	require.NoError(t, err)
	assert.Contains(t, stdout, "No environments found")
}

func TestEnvListFilterByType(t *testing.T) {
	setupTestEnv(t)
	gf := &cmd.GlobalFlags{Output: "text"}

	_, _, err := run(t, gf, "env", "create", "localenv", "--type", "local")
	require.NoError(t, err)

	// Filter for local should find it.
	stdout, _, err := run(t, gf, "env", "list", "--type", "local")
	require.NoError(t, err)
	assert.Contains(t, stdout, "localenv")

	// Filter for container should not find it.
	stdout, _, err = run(t, gf, "env", "list", "--type", "container")
	require.NoError(t, err)
	assert.NotContains(t, stdout, "localenv")
}

func TestEnvCreateInvalidName(t *testing.T) {
	setupTestEnv(t)
	gf := &cmd.GlobalFlags{Output: "text"}

	_, _, err := run(t, gf, "env", "create", "INVALID", "--type", "local")
	require.Error(t, err)
}

func TestEnvCreateDuplicateName(t *testing.T) {
	setupTestEnv(t)
	gf := &cmd.GlobalFlags{Output: "text"}

	_, _, err := run(t, gf, "env", "create", "duptest", "--type", "local")
	require.NoError(t, err)

	_, _, err = run(t, gf, "env", "create", "duptest", "--type", "local")
	require.Error(t, err)
}

func TestEnvCreateUnsupportedType(t *testing.T) {
	setupTestEnv(t)
	gf := &cmd.GlobalFlags{Output: "text"}

	_, _, err := run(t, gf, "env", "create", "containertest", "--type", "container")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not yet implemented")
}

func TestEnvStatus(t *testing.T) {
	setupTestEnv(t)
	gf := &cmd.GlobalFlags{Output: "text"}

	_, _, err := run(t, gf, "env", "create", "statustest", "--type", "local")
	require.NoError(t, err)

	stdout, _, err := run(t, gf, "env", "status", "statustest")
	require.NoError(t, err)
	assert.Contains(t, stdout, "statustest")
	assert.Contains(t, stdout, "local")
}

func TestEnvStatusJSON(t *testing.T) {
	setupTestEnv(t)
	gf := &cmd.GlobalFlags{Output: "json"}

	_, _, err := run(t, gf, "env", "create", "statusjson", "--type", "local")
	require.NoError(t, err)

	stdout, _, err := run(t, gf, "env", "status", "-o", "json", "statusjson")
	require.NoError(t, err)

	var out map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "output should be valid JSON: %s", stdout)
	assert.Equal(t, "statusjson", out["name"])
	assert.Equal(t, "local", out["type"])
}

func TestEnvStatusNotFound(t *testing.T) {
	setupTestEnv(t)
	gf := &cmd.GlobalFlags{Output: "text"}

	_, _, err := run(t, gf, "env", "status", "nonexistent")
	require.Error(t, err)
}

func TestEnvDelete(t *testing.T) {
	setupTestEnv(t)
	gf := &cmd.GlobalFlags{Output: "text"}

	_, _, err := run(t, gf, "env", "create", "deltest", "--type", "local")
	require.NoError(t, err)

	// Delete with --force (zellij stub returns no sessions, but force is clean).
	stdout, _, err := run(t, gf, "env", "delete", "deltest", "--force")
	require.NoError(t, err)
	assert.Contains(t, stdout, `"deltest" deleted`)

	// List should be empty now.
	stdout, _, err = run(t, gf, "env", "list")
	require.NoError(t, err)
	assert.Contains(t, stdout, "No environments found")
}

func TestEnvDeleteNotFound(t *testing.T) {
	setupTestEnv(t)
	gf := &cmd.GlobalFlags{Output: "text"}

	_, _, err := run(t, gf, "env", "delete", "ghost", "--force")
	require.Error(t, err)
}

func TestEnvStopLocal(t *testing.T) {
	setupTestEnv(t)
	gf := &cmd.GlobalFlags{Output: "text"}

	_, _, err := run(t, gf, "env", "create", "stoptest", "--type", "local")
	require.NoError(t, err)

	// Stop calls zellij kill-session; the stub zellij exits 0 but the
	// session doesn't actually exist, so Stop reports "not running".
	_, _, err = run(t, gf, "env", "stop", "stoptest")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not running")
}

func TestEnvStartLocal(t *testing.T) {
	setupTestEnv(t)
	gf := &cmd.GlobalFlags{Output: "text"}

	_, _, err := run(t, gf, "env", "create", "starttest", "--type", "local")
	require.NoError(t, err)

	// Start should succeed (zellij stub creates session, exits 0).
	stdout, _, err := run(t, gf, "env", "start", "starttest")
	require.NoError(t, err)
	assert.Contains(t, stdout, "started")
}

func TestEnvStubCommandsReturnNotImplemented(t *testing.T) {
	setupTestEnv(t)
	gf := &cmd.GlobalFlags{Output: "text"}

	stubs := [][]string{
		{"env", "exec", "test"},
		{"env", "push", "test"},
		{"env", "pull", "test"},
		{"env", "harvest", "test"},
		{"env", "logs", "test"},
	}

	for _, args := range stubs {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			_, _, err := run(t, gf, args...)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "not yet implemented")
		})
	}
}

func TestEnvFullLifecycle(t *testing.T) {
	setupTestEnv(t)
	gf := &cmd.GlobalFlags{Output: "text"}

	// 1. Create
	stdout, _, err := run(t, gf, "env", "create", "lifecycle", "--type", "local")
	require.NoError(t, err)
	assert.Contains(t, stdout, "created")

	// 2. List shows it
	stdout, _, err = run(t, gf, "env", "list")
	require.NoError(t, err)
	assert.Contains(t, stdout, "lifecycle")

	// 3. Status works
	stdout, _, err = run(t, gf, "env", "status", "lifecycle")
	require.NoError(t, err)
	assert.Contains(t, stdout, "lifecycle")

	// 4. Delete
	stdout, _, err = run(t, gf, "env", "delete", "lifecycle", "--force")
	require.NoError(t, err)
	assert.Contains(t, stdout, "deleted")

	// 5. List is empty
	stdout, _, err = run(t, gf, "env", "list")
	require.NoError(t, err)
	assert.Contains(t, stdout, "No environments found")

	// 6. Status on deleted env fails
	_, _, err = run(t, gf, "env", "status", "lifecycle")
	require.Error(t, err)
}

func TestEnvMultipleEnvironments(t *testing.T) {
	setupTestEnv(t)
	gf := &cmd.GlobalFlags{Output: "text"}

	names := []string{"alpha", "beta", "gamma"}
	for _, name := range names {
		_, _, err := run(t, gf, "env", "create", name, "--type", "local")
		require.NoError(t, err)
	}

	stdout, _, err := run(t, gf, "env", "list")
	require.NoError(t, err)
	for _, name := range names {
		assert.Contains(t, stdout, name)
	}

	// Delete one and verify
	_, _, err = run(t, gf, "env", "delete", "beta", "--force")
	require.NoError(t, err)

	stdout, _, err = run(t, gf, "env", "list")
	require.NoError(t, err)
	assert.Contains(t, stdout, "alpha")
	assert.NotContains(t, stdout, "beta")
	assert.Contains(t, stdout, "gamma")
}

func TestEnvCreateDefaultTypeIsLocal(t *testing.T) {
	setupTestEnv(t)
	gf := &cmd.GlobalFlags{Output: "json"}

	// Create without --type flag should default to local.
	_, _, err := run(t, gf, "env", "create", "defaulttype")
	require.NoError(t, err)

	stdout, _, err := run(t, gf, "env", "list", "-o", "json")
	require.NoError(t, err)

	var records []env.EnvironmentRecord
	require.NoError(t, json.Unmarshal([]byte(stdout), &records))
	require.Len(t, records, 1)
	assert.Equal(t, env.EnvironmentTypeLocal, records[0].Type)
}

func TestEnvStatePersistence(t *testing.T) {
	stateDir := setupTestEnv(t)
	gf := &cmd.GlobalFlags{Output: "text"}

	_, _, err := run(t, gf, "env", "create", "persist", "--type", "local")
	require.NoError(t, err)

	// Verify the state file was actually written to disk.
	stateFile := filepath.Join(stateDir, "state.yaml")
	data, err := os.ReadFile(stateFile)
	require.NoError(t, err, "state file should exist on disk")
	assert.Contains(t, string(data), "persist")
	assert.Contains(t, string(data), "local")
}
