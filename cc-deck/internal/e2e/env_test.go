//go:build e2e

// Package e2e contains end-to-end tests that build and run the cc-deck
// binary as an external process. These tests verify the compiled binary
// works correctly, including flag parsing, exit codes, and output format.
//
// Run with: go test -tags e2e -v ./internal/e2e/
// The binary is built once in TestMain and reused across all tests.
package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var binaryPath string

func TestMain(m *testing.M) {
	// Build the binary once for all tests.
	tmpDir, err := os.MkdirTemp("", "cc-deck-e2e-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	binaryPath = filepath.Join(tmpDir, "cc-deck")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/cc-deck")
	buildCmd.Dir = filepath.Join(mustFindProjectRoot(), "cc-deck")
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to build cc-deck binary: %v\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// mustFindProjectRoot walks up from cwd to find the project root (contains Makefile).
func mustFindProjectRoot() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "Makefile")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Fallback: assume we're inside cc-deck/internal/e2e
			d, _ := os.Getwd()
			return filepath.Join(d, "..", "..", "..")
		}
		dir = parent
	}
}

// testEnv holds isolated test state for a single test.
type testEnv struct {
	t         *testing.T
	stateFile string
	binDir    string
	env       []string
}

// setup creates an isolated environment for one test: temp state file
// and a zellij stub on PATH.
func setup(t *testing.T) *testEnv {
	t.Helper()

	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state.yaml")

	// Create zellij stub.
	binDir := filepath.Join(tmpDir, "bin")
	require.NoError(t, os.MkdirAll(binDir, 0o755))
	zellijStub := filepath.Join(binDir, "zellij")
	require.NoError(t, os.WriteFile(zellijStub, []byte("#!/bin/sh\nexit 0\n"), 0o755))

	env := append(os.Environ(),
		"CC_DECK_STATE_FILE="+stateFile,
		"PATH="+binDir+":"+os.Getenv("PATH"),
	)

	return &testEnv{t: t, stateFile: stateFile, binDir: binDir, env: env}
}

// run executes cc-deck with the given args and returns stdout, stderr,
// and any error.
func (te *testEnv) run(args ...string) (stdout, stderr string, err error) {
	te.t.Helper()

	cmd := exec.Command(binaryPath, args...)
	cmd.Env = te.env

	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

// mustRun calls run and fails the test if there is an error.
func (te *testEnv) mustRun(args ...string) (stdout, stderr string) {
	te.t.Helper()
	stdout, stderr, err := te.run(args...)
	require.NoError(te.t, err, "cc-deck %s\nstdout: %s\nstderr: %s",
		strings.Join(args, " "), stdout, stderr)
	return stdout, stderr
}

// --- E2E tests ---

func TestE2ECreateAndList(t *testing.T) {
	te := setup(t)

	stdout, _ := te.mustRun("env", "create", "e2e-test", "--type", "local")
	assert.Contains(t, stdout, "created")

	stdout, _ = te.mustRun("env", "list")
	assert.Contains(t, stdout, "e2e-test")
	assert.Contains(t, stdout, "local")
}

func TestE2EListJSON(t *testing.T) {
	te := setup(t)

	te.mustRun("env", "create", "jsonenv", "--type", "local")

	stdout, _ := te.mustRun("env", "list", "-o", "json")

	var records []map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(stdout), &records),
		"JSON output should be valid: %s", stdout)
	require.Len(t, records, 1)
	assert.Equal(t, "jsonenv", records[0]["Name"])
	assert.Equal(t, "local", records[0]["Type"])
}

func TestE2EListYAML(t *testing.T) {
	te := setup(t)

	te.mustRun("env", "create", "yamlenv")

	stdout, _ := te.mustRun("env", "list", "-o", "yaml")
	assert.Contains(t, stdout, "name: yamlenv")
	assert.Contains(t, stdout, "type: local")
}

func TestE2EStatus(t *testing.T) {
	te := setup(t)

	te.mustRun("env", "create", "statusenv")

	stdout, _ := te.mustRun("env", "status", "statusenv")
	assert.Contains(t, stdout, "statusenv")
	assert.Contains(t, stdout, "local")
}

func TestE2EStatusJSON(t *testing.T) {
	te := setup(t)

	te.mustRun("env", "create", "statusjson")

	stdout, _ := te.mustRun("env", "status", "statusjson", "-o", "json")

	var out map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, "statusjson", out["name"])
	assert.Equal(t, "local", out["type"])
}

func TestE2EDelete(t *testing.T) {
	te := setup(t)

	te.mustRun("env", "create", "delenv")
	stdout, _ := te.mustRun("env", "delete", "delenv", "--force")
	assert.Contains(t, stdout, "deleted")

	stdout, _ = te.mustRun("env", "list")
	assert.Contains(t, stdout, "No environments found")
}

func TestE2EFullLifecycle(t *testing.T) {
	te := setup(t)

	// Create
	stdout, _ := te.mustRun("env", "create", "lifecycle")
	assert.Contains(t, stdout, "created")

	// List
	stdout, _ = te.mustRun("env", "list")
	assert.Contains(t, stdout, "lifecycle")

	// Status
	stdout, _ = te.mustRun("env", "status", "lifecycle")
	assert.Contains(t, stdout, "lifecycle")

	// Status JSON
	stdout, _ = te.mustRun("env", "status", "lifecycle", "-o", "json")
	var status map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(stdout), &status))
	assert.Equal(t, "lifecycle", status["name"])

	// Delete
	stdout, _ = te.mustRun("env", "delete", "lifecycle", "--force")
	assert.Contains(t, stdout, "deleted")

	// Gone
	stdout, _ = te.mustRun("env", "list")
	assert.Contains(t, stdout, "No environments found")
}

func TestE2ECreateInvalidName(t *testing.T) {
	te := setup(t)

	_, stderr, err := te.run("env", "create", "INVALID")
	require.Error(t, err)
	assert.Contains(t, stderr, "invalid")
}

func TestE2ECreateDuplicate(t *testing.T) {
	te := setup(t)

	te.mustRun("env", "create", "dupenv")
	_, _, err := te.run("env", "create", "dupenv")
	require.Error(t, err)
}

func TestE2EDeleteNotFound(t *testing.T) {
	te := setup(t)

	_, _, err := te.run("env", "delete", "ghost", "--force")
	require.Error(t, err)
}

func TestE2EStatusNotFound(t *testing.T) {
	te := setup(t)

	_, _, err := te.run("env", "status", "ghost")
	require.Error(t, err)
}

func TestE2EStopLocalPrintsNote(t *testing.T) {
	te := setup(t)

	te.mustRun("env", "create", "stopenv")
	_, stderr := te.mustRun("env", "stop", "stopenv")
	assert.Contains(t, stderr, "not supported")
}

func TestE2EMultipleEnvironments(t *testing.T) {
	te := setup(t)

	for _, name := range []string{"alpha", "beta", "gamma"} {
		te.mustRun("env", "create", name)
	}

	stdout, _ := te.mustRun("env", "list")
	assert.Contains(t, stdout, "alpha")
	assert.Contains(t, stdout, "beta")
	assert.Contains(t, stdout, "gamma")

	te.mustRun("env", "delete", "beta", "--force")

	stdout, _ = te.mustRun("env", "list")
	assert.Contains(t, stdout, "alpha")
	assert.NotContains(t, stdout, "beta")
	assert.Contains(t, stdout, "gamma")
}

func TestE2EListFilterByType(t *testing.T) {
	te := setup(t)

	te.mustRun("env", "create", "filterenv")

	stdout, _ := te.mustRun("env", "list", "--type", "local")
	assert.Contains(t, stdout, "filterenv")

	stdout, _ = te.mustRun("env", "list", "--type", "podman")
	assert.NotContains(t, stdout, "filterenv")
}

func TestE2EDefaultTypeIsLocal(t *testing.T) {
	te := setup(t)

	// Omit --type, should default to local.
	te.mustRun("env", "create", "defaultenv")

	stdout, _ := te.mustRun("env", "list", "-o", "json")
	var records []map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(stdout), &records))
	require.Len(t, records, 1)
	assert.Equal(t, "local", records[0]["Type"])
}

func TestE2EVersionFlag(t *testing.T) {
	te := setup(t)

	stdout, _ := te.mustRun("version")
	assert.Contains(t, stdout, "cc-deck")
}

func TestE2EHelpFlag(t *testing.T) {
	te := setup(t)

	stdout, _ := te.mustRun("env", "--help")
	assert.Contains(t, stdout, "create")
	assert.Contains(t, stdout, "list")
	assert.Contains(t, stdout, "delete")
	assert.Contains(t, stdout, "status")
}

func TestE2EStatePersistence(t *testing.T) {
	te := setup(t)

	te.mustRun("env", "create", "persist")

	data, err := os.ReadFile(te.stateFile)
	require.NoError(t, err, "state file should exist")
	assert.Contains(t, string(data), "persist")
	assert.Contains(t, string(data), "local")
}
