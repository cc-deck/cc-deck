package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/cc-deck/cc-deck/internal/env"
)

func TestRunEnvInit_InGitRepo(t *testing.T) {
	tmpDir := t.TempDir()

	// Initialize a git repo.
	cmd := exec.Command("git", "init", tmpDir)
	require.NoError(t, cmd.Run())

	// Change to the git repo directory.
	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(origDir)

	inf := &initFlags{
		envType: "compose",
		image:   "quay.io/cc-deck/cc-deck-demo:latest",
		auth:    "auto",
	}

	err := runEnvInit(inf)
	require.NoError(t, err)

	// Verify environment.yaml was created.
	defPath := filepath.Join(tmpDir, ".cc-deck", "environment.yaml")
	data, err := os.ReadFile(defPath)
	require.NoError(t, err)

	var def env.EnvironmentDefinition
	require.NoError(t, yaml.Unmarshal(data, &def))
	assert.Equal(t, filepath.Base(tmpDir), def.Name)
	assert.Equal(t, env.EnvironmentType("compose"), def.Type)
	assert.Equal(t, "quay.io/cc-deck/cc-deck-demo:latest", def.Image)

	// Verify .gitignore was created.
	giPath := filepath.Join(tmpDir, ".cc-deck", ".gitignore")
	giData, err := os.ReadFile(giPath)
	require.NoError(t, err)
	assert.Contains(t, string(giData), "status.yaml")
	assert.Contains(t, string(giData), "run/")
}

func TestRunEnvInit_AlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	cmd := exec.Command("git", "init", tmpDir)
	require.NoError(t, cmd.Run())

	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(origDir)

	// Create initial definition.
	inf := &initFlags{envType: "compose"}
	require.NoError(t, runEnvInit(inf))

	// Second init should fail.
	err := runEnvInit(inf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestRunEnvInit_NotGitRepo(t *testing.T) {
	tmpDir := t.TempDir()

	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(origDir)

	inf := &initFlags{envType: "compose"}

	// Should succeed with warning (warning goes to stderr, we just check no error).
	err := runEnvInit(inf)
	require.NoError(t, err)

	// Verify definition was created.
	defPath := filepath.Join(tmpDir, ".cc-deck", "environment.yaml")
	_, err = os.Stat(defPath)
	assert.NoError(t, err)
}

func TestRunEnvInit_CustomName(t *testing.T) {
	tmpDir := t.TempDir()
	cmd := exec.Command("git", "init", tmpDir)
	require.NoError(t, cmd.Run())

	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(origDir)

	inf := &initFlags{
		envType: "compose",
		name:    "my-custom-env",
	}

	require.NoError(t, runEnvInit(inf))

	def, err := env.LoadProjectDefinition(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, "my-custom-env", def.Name)
}
