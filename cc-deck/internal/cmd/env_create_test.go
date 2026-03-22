package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/cc-deck/cc-deck/internal/env"
)

// newTestCreateCmd creates a cobra command for testing runEnvCreate.
func newTestCreateCmd() (*cobra.Command, *createFlags) {
	var cf createFlags
	cmd := &cobra.Command{
		Use:  "create [name]",
		Args: cobra.MaximumNArgs(1),
	}
	cmd.Flags().StringVarP(&cf.envType, "type", "t", "", "")
	cmd.Flags().StringVar(&cf.image, "image", "", "")
	cmd.Flags().StringVar(&cf.auth, "auth", "auto", "")
	cmd.Flags().StringSliceVar(&cf.allowedDomains, "allowed-domains", nil, "")
	cmd.Flags().StringSliceVar(&cf.ports, "port", nil, "")
	cmd.Flags().StringSliceVar(&cf.mount, "mount", nil, "")
	cmd.Flags().StringSliceVar(&cf.credential, "credential", nil, "")
	cmd.Flags().StringVar(&cf.storage, "storage", "", "")
	cmd.Flags().StringVar(&cf.path, "path", "", "")
	cmd.Flags().BoolVar(&cf.allPorts, "all-ports", false, "")
	cmd.Flags().BoolVar(&cf.gitignore, "gitignore", false, "")
	return cmd, &cf
}

func TestRunEnvCreate_ResolvesNameFromDefinition(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, exec.Command("git", "init", tmpDir).Run())

	// Create a project-local definition.
	def := &env.EnvironmentDefinition{
		Name: "my-test-api",
		Type: env.EnvironmentTypeLocal,
	}
	require.NoError(t, env.SaveProjectDefinition(tmpDir, def))

	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(origDir)

	// Set up isolated state files.
	stateFile := filepath.Join(tmpDir, "test-state.yaml")
	defFile := filepath.Join(tmpDir, "test-defs.yaml")
	t.Setenv("CC_DECK_STATE_FILE", stateFile)
	t.Setenv("CC_DECK_DEFINITIONS_FILE", defFile)

	cmd, cf := newTestCreateCmd()
	// No name argument, should resolve from definition.
	// Type defaults to local from definition, which doesn't need podman.
	err := runEnvCreate(nil, "", cf, cmd)
	require.NoError(t, err)

	// Verify project was registered in global registry.
	store := env.NewStateStore(stateFile)
	projects, err := store.ListProjects()
	require.NoError(t, err)
	assert.Len(t, projects, 1)

	// Verify status.yaml was created.
	statusStore := env.NewProjectStatusStore(tmpDir)
	status, err := statusStore.Load()
	require.NoError(t, err)
	assert.Equal(t, "cc-deck-my-test-api", status.ContainerName)
}

func TestRunEnvCreate_AutoScaffoldWhenNoDefinition(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, exec.Command("git", "init", tmpDir).Run())

	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(origDir)

	stateFile := filepath.Join(tmpDir, "test-state.yaml")
	defFile := filepath.Join(tmpDir, "test-defs.yaml")
	t.Setenv("CC_DECK_STATE_FILE", stateFile)
	t.Setenv("CC_DECK_DEFINITIONS_FILE", defFile)

	cmd, cf := newTestCreateCmd()
	require.NoError(t, cmd.Flags().Set("type", "local"))

	// No name, no definition. Should auto-scaffold.
	err := runEnvCreate(nil, "", cf, cmd)
	require.NoError(t, err)

	// Verify environment.yaml was scaffolded.
	defPath := filepath.Join(tmpDir, ".cc-deck", "environment.yaml")
	data, err := os.ReadFile(defPath)
	require.NoError(t, err)

	var savedDef env.EnvironmentDefinition
	require.NoError(t, yaml.Unmarshal(data, &savedDef))
	assert.Equal(t, filepath.Base(tmpDir), savedDef.Name)
	assert.Equal(t, env.EnvironmentTypeLocal, savedDef.Type)
}

func TestRunEnvCreate_CLIOverrideStoredInStatusYaml(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, exec.Command("git", "init", tmpDir).Run())

	// Create definition with image.
	def := &env.EnvironmentDefinition{
		Name:  "test-override",
		Type:  env.EnvironmentTypeLocal,
		Image: "original:latest",
	}
	require.NoError(t, env.SaveProjectDefinition(tmpDir, def))

	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(origDir)

	stateFile := filepath.Join(tmpDir, "test-state.yaml")
	defFile := filepath.Join(tmpDir, "test-defs.yaml")
	t.Setenv("CC_DECK_STATE_FILE", stateFile)
	t.Setenv("CC_DECK_DEFINITIONS_FILE", defFile)

	cmd, cf := newTestCreateCmd()
	require.NoError(t, cmd.Flags().Set("image", "override:latest"))

	err := runEnvCreate(nil, "test-override", cf, cmd)
	require.NoError(t, err)

	// Verify override is in status.yaml, NOT in environment.yaml.
	statusStore := env.NewProjectStatusStore(tmpDir)
	status, err := statusStore.Load()
	require.NoError(t, err)
	assert.Equal(t, "override:latest", status.Overrides["image"])

	// environment.yaml should still have original image.
	loadedDef, err := env.LoadProjectDefinition(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, "original:latest", loadedDef.Image)
}

func TestRunEnvCreate_FailsWithoutNameOutsideGitRepo(t *testing.T) {
	tmpDir := t.TempDir()

	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(origDir)

	cmd, cf := newTestCreateCmd()
	err := runEnvCreate(nil, "", cf, cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no environment name specified")
}
