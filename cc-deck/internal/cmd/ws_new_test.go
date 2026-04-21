package cmd

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/cc-deck/cc-deck/internal/ws"
)

// ensureZellijStub puts a dummy zellij binary on PATH so that
// exec.LookPath("zellij") succeeds in CI where zellij is not installed.
func ensureZellijStub(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("zellij"); err == nil {
		return // real zellij available
	}
	binDir := filepath.Join(t.TempDir(), "bin")
	require.NoError(t, os.MkdirAll(binDir, 0o755))
	stub := filepath.Join(binDir, "zellij")
	require.NoError(t, os.WriteFile(stub, []byte("#!/bin/sh\nexit 0\n"), 0o755))
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))
}

// newTestNewCmd creates a cobra command for testing runWsNew.
func newTestNewCmd() (*cobra.Command, *newFlags) {
	var cf newFlags
	cmd := &cobra.Command{
		Use:  "new [name]",
		Args: cobra.MaximumNArgs(1),
	}
	cmd.Flags().StringVarP(&cf.wsType, "type", "t", "", "")
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
	cmd.Flags().StringVar(&cf.host, "host", "", "")
	cmd.Flags().IntVar(&cf.sshPort, "ssh-port", 0, "")
	cmd.Flags().StringVar(&cf.identityFile, "identity-file", "", "")
	cmd.Flags().StringVar(&cf.jumpHost, "jump-host", "", "")
	cmd.Flags().StringVar(&cf.sshConfig, "ssh-config", "", "")
	cmd.Flags().StringVar(&cf.workspace, "workspace", "", "")
	cmd.Flags().StringVar(&cf.variant, "variant", "", "")
	cmd.Flags().BoolVar(&cf.global, "global", false, "")
	cmd.Flags().BoolVar(&cf.local, "local", false, "")
	cmd.MarkFlagsMutuallyExclusive("global", "local")
	return cmd, &cf
}

func TestRunWsNew_ResolvesNameFromDefinition(t *testing.T) {
	ensureZellijStub(t)
	tmpDir := t.TempDir()
	require.NoError(t, exec.Command("git", "init", tmpDir).Run())

	// Create a project-local definition.
	def := &ws.WorkspaceDefinition{
		Name: "my-test-api",
		Type: ws.WorkspaceTypeLocal,
	}
	require.NoError(t, ws.SaveProjectDefinition(tmpDir, def))

	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(origDir)

	// Set up isolated state files.
	stateFile := filepath.Join(tmpDir, "test-state.yaml")
	defFile := filepath.Join(tmpDir, "test-defs.yaml")
	t.Setenv("CC_DECK_STATE_FILE", stateFile)
	t.Setenv("CC_DECK_WORKSPACES_FILE", defFile)

	cmd, cf := newTestNewCmd()
	// No name argument, should resolve from definition.
	// Type defaults to local from definition, which doesn't need podman.
	err := runWsNew(nil, "", cf, cmd)
	require.NoError(t, err)

	// Verify project was registered in global registry.
	store := ws.NewStateStore(stateFile)
	projects, err := store.ListProjects()
	require.NoError(t, err)
	assert.Len(t, projects, 1)

	// Verify status.yaml was created.
	statusStore := ws.NewProjectStatusStore(tmpDir)
	status, err := statusStore.Load()
	require.NoError(t, err)
	assert.Equal(t, "cc-deck-my-test-api", status.ContainerName)
}

func TestRunWsNew_ScaffoldsDefinitionWhenMissing(t *testing.T) {
	ensureZellijStub(t)
	tmpDir := t.TempDir()
	require.NoError(t, exec.Command("git", "init", tmpDir).Run())

	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(origDir)

	stateFile := filepath.Join(tmpDir, "test-state.yaml")
	defFile := filepath.Join(tmpDir, "test-defs.yaml")
	t.Setenv("CC_DECK_STATE_FILE", stateFile)
	t.Setenv("CC_DECK_WORKSPACES_FILE", defFile)

	cmd, cf := newTestNewCmd()
	require.NoError(t, cmd.Flags().Set("type", "local"))

	// No name, no definition. Should scaffold + create.
	err := runWsNew(nil, "", cf, cmd)
	require.NoError(t, err)

	// Verify definition was scaffolded.
	def, loadErr := ws.LoadProjectDefinition(tmpDir)
	require.NoError(t, loadErr)
	assert.Equal(t, filepath.Base(tmpDir), def.Name)
	assert.Equal(t, ws.WorkspaceTypeLocal, def.Type)
}

func TestRunWsNew_CLIOverrideStoredInStatusYaml(t *testing.T) {
	ensureZellijStub(t)
	tmpDir := t.TempDir()
	require.NoError(t, exec.Command("git", "init", tmpDir).Run())

	// Create definition with image.
	def := &ws.WorkspaceDefinition{
		Name:  "test-override",
		Type:  ws.WorkspaceTypeLocal,
		Image: "original:latest",
	}
	require.NoError(t, ws.SaveProjectDefinition(tmpDir, def))

	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(origDir)

	stateFile := filepath.Join(tmpDir, "test-state.yaml")
	defFile := filepath.Join(tmpDir, "test-defs.yaml")
	t.Setenv("CC_DECK_STATE_FILE", stateFile)
	t.Setenv("CC_DECK_WORKSPACES_FILE", defFile)

	cmd, cf := newTestNewCmd()
	require.NoError(t, cmd.Flags().Set("image", "override:latest"))

	err := runWsNew(nil, "test-override", cf, cmd)
	require.NoError(t, err)

	// Verify override is in status.yaml, NOT in workspace.yaml.
	statusStore := ws.NewProjectStatusStore(tmpDir)
	status, err := statusStore.Load()
	require.NoError(t, err)
	assert.Equal(t, "override:latest", status.Overrides["image"])

	// workspace.yaml should still have original image.
	loadedDef, err := ws.LoadProjectDefinition(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, "original:latest", loadedDef.Image)
}

func TestRunWsNew_ExplicitNameUsesGlobalDefinition(t *testing.T) {
	ensureZellijStub(t)
	tmpDir := t.TempDir()
	require.NoError(t, exec.Command("git", "init", tmpDir).Run())

	// Create a project-local definition with a different name.
	projDef := &ws.WorkspaceDefinition{
		Name: "project-env",
		Type: ws.WorkspaceTypeLocal,
	}
	require.NoError(t, ws.SaveProjectDefinition(tmpDir, projDef))

	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(origDir)

	stateFile := filepath.Join(tmpDir, "test-state.yaml")
	defFile := filepath.Join(tmpDir, "test-defs.yaml")
	t.Setenv("CC_DECK_STATE_FILE", stateFile)
	t.Setenv("CC_DECK_WORKSPACES_FILE", defFile)

	// Add a global definition for a different name.
	defs := ws.NewDefinitionStore(defFile)
	require.NoError(t, defs.Add(&ws.WorkspaceDefinition{
		Name: "global-env",
		Type: ws.WorkspaceTypeLocal,
	}))

	cmd, cf := newTestNewCmd()
	// Explicit name matching global def, not project-local def.
	err := runWsNew(nil, "global-env", cf, cmd)
	require.NoError(t, err)

	// Verify instance was created with the global env name.
	store := ws.NewStateStore(stateFile)
	inst, findErr := store.FindInstanceByName("global-env")
	require.NoError(t, findErr)
	assert.Equal(t, ws.WorkspaceTypeLocal, inst.Type)

	// Verify no scaffolding happened (FR-002a): project definition should still be "project-env".
	loadedDef, loadErr := ws.LoadProjectDefinition(tmpDir)
	require.NoError(t, loadErr)
	assert.Equal(t, "project-env", loadedDef.Name, "project-local definition should not be overwritten")
}

func TestRunWsNew_ExplicitNameNotFoundFallsToLocal(t *testing.T) {
	ensureZellijStub(t)
	tmpDir := t.TempDir()
	require.NoError(t, exec.Command("git", "init", tmpDir).Run())

	// Create a project-local definition with a different name.
	projDef := &ws.WorkspaceDefinition{
		Name: "project-env",
		Type: ws.WorkspaceTypeLocal,
	}
	require.NoError(t, ws.SaveProjectDefinition(tmpDir, projDef))

	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(origDir)

	stateFile := filepath.Join(tmpDir, "test-state.yaml")
	defFile := filepath.Join(tmpDir, "test-defs.yaml")
	t.Setenv("CC_DECK_STATE_FILE", stateFile)
	t.Setenv("CC_DECK_WORKSPACES_FILE", defFile)

	cmd, cf := newTestNewCmd()
	// Explicit name not in any store: should fall back to local type (FR-003).
	err := runWsNew(nil, "unknown-env", cf, cmd)
	require.NoError(t, err)

	store := ws.NewStateStore(stateFile)
	inst, findErr := store.FindInstanceByName("unknown-env")
	require.NoError(t, findErr)
	assert.Equal(t, ws.WorkspaceTypeLocal, inst.Type)
}

func TestRunWsNew_ExplicitNameMatchingProjectLocalUsesIt(t *testing.T) {
	ensureZellijStub(t)
	tmpDir := t.TempDir()
	require.NoError(t, exec.Command("git", "init", tmpDir).Run())

	// Create a project-local definition.
	projDef := &ws.WorkspaceDefinition{
		Name: "my-env",
		Type: ws.WorkspaceTypeLocal,
	}
	require.NoError(t, ws.SaveProjectDefinition(tmpDir, projDef))

	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(origDir)

	stateFile := filepath.Join(tmpDir, "test-state.yaml")
	defFile := filepath.Join(tmpDir, "test-defs.yaml")
	t.Setenv("CC_DECK_STATE_FILE", stateFile)
	t.Setenv("CC_DECK_WORKSPACES_FILE", defFile)

	cmd, cf := newTestNewCmd()
	// Explicit name matches project-local: should use project-local (FR-004).
	err := runWsNew(nil, "my-env", cf, cmd)
	require.NoError(t, err)

	store := ws.NewStateStore(stateFile)
	inst, findErr := store.FindInstanceByName("my-env")
	require.NoError(t, findErr)
	assert.Equal(t, ws.WorkspaceTypeLocal, inst.Type)
}

func TestRunWsNew_TypeFlagOverridesGlobalDefinition(t *testing.T) {
	ensureZellijStub(t)
	tmpDir := t.TempDir()
	require.NoError(t, exec.Command("git", "init", tmpDir).Run())

	// Create a project-local definition with a different name.
	projDef := &ws.WorkspaceDefinition{
		Name: "project-env",
		Type: ws.WorkspaceTypeLocal,
	}
	require.NoError(t, ws.SaveProjectDefinition(tmpDir, projDef))

	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(origDir)

	stateFile := filepath.Join(tmpDir, "test-state.yaml")
	defFile := filepath.Join(tmpDir, "test-defs.yaml")
	t.Setenv("CC_DECK_STATE_FILE", stateFile)
	t.Setenv("CC_DECK_WORKSPACES_FILE", defFile)

	// Add a global definition with a specific type.
	defs := ws.NewDefinitionStore(defFile)
	require.NoError(t, defs.Add(&ws.WorkspaceDefinition{
		Name: "global-env",
		Type: ws.WorkspaceTypeLocal,
	}))

	cmd, cf := newTestNewCmd()
	// --type flag should override global definition type (FR-011).
	require.NoError(t, cmd.Flags().Set("type", "local"))
	err := runWsNew(nil, "global-env", cf, cmd)
	require.NoError(t, err)

	store := ws.NewStateStore(stateFile)
	inst, findErr := store.FindInstanceByName("global-env")
	require.NoError(t, findErr)
	assert.Equal(t, ws.WorkspaceTypeLocal, inst.Type)
}

func TestRunWsNew_GlobalFlagSelectsGlobalDefinition(t *testing.T) {
	ensureZellijStub(t)
	tmpDir := t.TempDir()
	require.NoError(t, exec.Command("git", "init", tmpDir).Run())

	// Create project-local definition.
	projDef := &ws.WorkspaceDefinition{
		Name: "myenv",
		Type: ws.WorkspaceTypeLocal,
	}
	require.NoError(t, ws.SaveProjectDefinition(tmpDir, projDef))

	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(origDir)

	stateFile := filepath.Join(tmpDir, "test-state.yaml")
	defFile := filepath.Join(tmpDir, "test-defs.yaml")
	t.Setenv("CC_DECK_STATE_FILE", stateFile)
	t.Setenv("CC_DECK_WORKSPACES_FILE", defFile)

	// Add a global definition with the same name.
	defs := ws.NewDefinitionStore(defFile)
	require.NoError(t, defs.Add(&ws.WorkspaceDefinition{
		Name: "myenv",
		Type: ws.WorkspaceTypeLocal,
	}))

	cmd, cf := newTestNewCmd()
	require.NoError(t, cmd.Flags().Set("global", "true"))
	err := runWsNew(nil, "myenv", cf, cmd)
	require.NoError(t, err)

	store := ws.NewStateStore(stateFile)
	_, findErr := store.FindInstanceByName("myenv")
	require.NoError(t, findErr)
}

func TestRunWsNew_LocalFlagSelectsProjectLocal(t *testing.T) {
	ensureZellijStub(t)
	tmpDir := t.TempDir()
	require.NoError(t, exec.Command("git", "init", tmpDir).Run())

	projDef := &ws.WorkspaceDefinition{
		Name: "myenv",
		Type: ws.WorkspaceTypeLocal,
	}
	require.NoError(t, ws.SaveProjectDefinition(tmpDir, projDef))

	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(origDir)

	stateFile := filepath.Join(tmpDir, "test-state.yaml")
	defFile := filepath.Join(tmpDir, "test-defs.yaml")
	t.Setenv("CC_DECK_STATE_FILE", stateFile)
	t.Setenv("CC_DECK_WORKSPACES_FILE", defFile)

	cmd, cf := newTestNewCmd()
	require.NoError(t, cmd.Flags().Set("local", "true"))
	err := runWsNew(nil, "myenv", cf, cmd)
	require.NoError(t, err)

	store := ws.NewStateStore(stateFile)
	_, findErr := store.FindInstanceByName("myenv")
	require.NoError(t, findErr)
}

func TestRunWsNew_GlobalFlagErrorsWhenNoGlobalDef(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, exec.Command("git", "init", tmpDir).Run())

	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(origDir)

	stateFile := filepath.Join(tmpDir, "test-state.yaml")
	defFile := filepath.Join(tmpDir, "test-defs.yaml")
	t.Setenv("CC_DECK_STATE_FILE", stateFile)
	t.Setenv("CC_DECK_WORKSPACES_FILE", defFile)

	cmd, cf := newTestNewCmd()
	require.NoError(t, cmd.Flags().Set("global", "true"))
	err := runWsNew(nil, "nonexistent", cf, cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no global definition found")
}

func TestRunWsNew_LocalFlagErrorsWhenNoProjectDef(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, exec.Command("git", "init", tmpDir).Run())

	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(origDir)

	stateFile := filepath.Join(tmpDir, "test-state.yaml")
	defFile := filepath.Join(tmpDir, "test-defs.yaml")
	t.Setenv("CC_DECK_STATE_FILE", stateFile)
	t.Setenv("CC_DECK_WORKSPACES_FILE", defFile)

	cmd, cf := newTestNewCmd()
	require.NoError(t, cmd.Flags().Set("local", "true"))
	err := runWsNew(nil, "myenv", cf, cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no project-local definition found")
}

func TestRunWsNew_LocalFlagErrorsOnNameMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, exec.Command("git", "init", tmpDir).Run())

	// Create project-local definition with name "proj-env".
	projDef := &ws.WorkspaceDefinition{
		Name: "proj-env",
		Type: ws.WorkspaceTypeLocal,
	}
	require.NoError(t, ws.SaveProjectDefinition(tmpDir, projDef))

	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(origDir)

	stateFile := filepath.Join(tmpDir, "test-state.yaml")
	defFile := filepath.Join(tmpDir, "test-defs.yaml")
	t.Setenv("CC_DECK_STATE_FILE", stateFile)
	t.Setenv("CC_DECK_WORKSPACES_FILE", defFile)

	cmd, cf := newTestNewCmd()
	require.NoError(t, cmd.Flags().Set("local", "true"))
	// Explicit name differs from project-local definition name.
	err := runWsNew(nil, "other-name", cf, cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "project-local definition is")
}

func TestRunWsNew_GlobalAndLocalMutuallyExclusive(t *testing.T) {
	cmd, _ := newTestNewCmd()
	// Cobra validates mutual exclusion during Execute, but we can test
	// that the flags are registered and mutually exclusive.
	require.NoError(t, cmd.Flags().Set("global", "true"))
	err := cmd.Flags().Set("local", "true")
	// Both can be set on flags, but cobra validates during execution.
	// Let's verify by checking the ValidateFlagGroups method.
	_ = err
	cmd.SetArgs([]string{})
	cmd.RunE = func(cmd *cobra.Command, args []string) error { return nil }
	execErr := cmd.Execute()
	require.Error(t, execErr, "should reject mutually exclusive --global and --local")
}

func TestRunWsNew_GlobalWithTypeFlagOverridesType(t *testing.T) {
	ensureZellijStub(t)
	tmpDir := t.TempDir()
	require.NoError(t, exec.Command("git", "init", tmpDir).Run())

	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(origDir)

	stateFile := filepath.Join(tmpDir, "test-state.yaml")
	defFile := filepath.Join(tmpDir, "test-defs.yaml")
	t.Setenv("CC_DECK_STATE_FILE", stateFile)
	t.Setenv("CC_DECK_WORKSPACES_FILE", defFile)

	// Add a global definition.
	defs := ws.NewDefinitionStore(defFile)
	require.NoError(t, defs.Add(&ws.WorkspaceDefinition{
		Name: "global-env",
		Type: ws.WorkspaceTypeLocal,
	}))

	cmd, cf := newTestNewCmd()
	require.NoError(t, cmd.Flags().Set("global", "true"))
	require.NoError(t, cmd.Flags().Set("type", "local"))
	err := runWsNew(nil, "global-env", cf, cmd)
	require.NoError(t, err)

	store := ws.NewStateStore(stateFile)
	inst, findErr := store.FindInstanceByName("global-env")
	require.NoError(t, findErr)
	assert.Equal(t, ws.WorkspaceTypeLocal, inst.Type)
}

func TestRunWsNew_FailsWithoutNameOutsideGitRepo(t *testing.T) {
	tmpDir := t.TempDir()

	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(origDir)

	cmd, cf := newTestNewCmd()
	err := runWsNew(nil, "", cf, cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no workspace name specified")
}

func TestWriteEnvStructured_IncludesSourceField(t *testing.T) {
	tmpDir := t.TempDir()
	defFile := filepath.Join(tmpDir, "test-defs.yaml")
	t.Setenv("CC_DECK_WORKSPACES_FILE", defFile)

	defs := ws.NewDefinitionStore(defFile)
	require.NoError(t, defs.Add(&ws.WorkspaceDefinition{
		Name: "global-env",
		Type: ws.WorkspaceTypeSSH,
		Host: "user@host",
	}))

	instances := []*ws.WorkspaceInstance{
		{Name: "global-env", Type: ws.WorkspaceTypeSSH, State: ws.WorkspaceStateRunning,
			SSH: &ws.SSHFields{Host: "user@host"}},
	}
	instanceNames := map[string]bool{"global-env": true}

	// Capture JSON output.
	old := os.Stdout
	r, w, pipeErr := os.Pipe()
	require.NoError(t, pipeErr)
	os.Stdout = w

	err := writeWsStructured("json", instances, nil, instanceNames, "", defs, nil)
	require.NoError(t, err)

	w.Close()
	os.Stdout = old

	out, _ := io.ReadAll(r)
	output := string(out)

	assert.Contains(t, output, `"source"`)
	assert.Contains(t, output, `"global"`)
}

func TestWriteEnvStructured_ProjectSourceForProjectEnvs(t *testing.T) {
	tmpDir := t.TempDir()
	defFile := filepath.Join(tmpDir, "test-defs.yaml")
	t.Setenv("CC_DECK_WORKSPACES_FILE", defFile)

	defs := ws.NewDefinitionStore(defFile)

	projectWs := []projectListEntry{
		{Name: "proj-env", Type: ws.WorkspaceTypeCompose, Status: "not created", Path: "/some/path"},
	}

	// Capture JSON output.
	old := os.Stdout
	r, w, pipeErr := os.Pipe()
	require.NoError(t, pipeErr)
	os.Stdout = w

	err := writeWsStructured("json", nil, nil, map[string]bool{}, "", defs, projectWs)
	require.NoError(t, err)

	w.Close()
	os.Stdout = old

	out, _ := io.ReadAll(r)
	output := string(out)

	assert.Contains(t, output, `"source"`)
	assert.Contains(t, output, `"project"`)
}

func TestWriteEnvTableWithProjects_HasSourceNoPath(t *testing.T) {
	tmpDir := t.TempDir()
	defFile := filepath.Join(tmpDir, "test-defs.yaml")
	t.Setenv("CC_DECK_WORKSPACES_FILE", defFile)

	defs := ws.NewDefinitionStore(defFile)

	projectWs := []projectListEntry{
		{Name: "proj-env", Type: ws.WorkspaceTypeCompose, Status: "not created", Path: "/some/path"},
	}

	// Capture output.
	old := os.Stdout
	r, w, pipeErr := os.Pipe()
	require.NoError(t, pipeErr)
	os.Stdout = w

	err := writeWsTableWithProjects(nil, nil, map[string]bool{}, "", projectWs, nil, defs, false)
	require.NoError(t, err)

	w.Close()
	os.Stdout = old

	out, _ := io.ReadAll(r)
	output := string(out)

	assert.Contains(t, output, "SOURCE")
	assert.NotContains(t, output, "PATH")
}
