package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/cc-deck/cc-deck/internal/ws"
)

func ensureZellijStub(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("zellij"); err == nil {
		return
	}
	binDir := filepath.Join(t.TempDir(), "bin")
	require.NoError(t, os.MkdirAll(binDir, 0o755))
	stub := filepath.Join(binDir, "zellij")
	require.NoError(t, os.WriteFile(stub, []byte("#!/bin/sh\nexit 0\n"), 0o755))
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))
}

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
	cmd.Flags().StringVar(&cf.namespace, "namespace", "", "")
	cmd.Flags().StringVar(&cf.kubeconfig, "kubeconfig", "", "")
	cmd.Flags().StringVar(&cf.k8sContext, "context", "", "")
	cmd.Flags().StringVar(&cf.storageSize, "storage-size", "", "")
	cmd.Flags().StringVar(&cf.storageClass, "storage-class", "", "")
	return cmd, &cf
}

func TestRunWsNew_ExplicitNameCreatesInCentralStore(t *testing.T) {
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
	err := runWsNew(nil, "my-ws", cf, cmd)
	require.NoError(t, err)

	// Verify definition was stored centrally.
	defs := ws.NewDefinitionStore(defFile)
	found, findErr := defs.FindByName("my-ws")
	require.NoError(t, findErr)
	assert.Equal(t, ws.WorkspaceTypeLocal, found.Type)
	assert.NotEmpty(t, found.ProjectDir)
}

func TestRunWsNew_TemplateBasedCreation(t *testing.T) {
	ensureZellijStub(t)
	tmpDir := t.TempDir()
	require.NoError(t, exec.Command("git", "init", tmpDir).Run())

	// Create a template with a single variant.
	ccDir := filepath.Join(tmpDir, ".cc-deck")
	require.NoError(t, os.MkdirAll(ccDir, 0o755))
	tmplContent := `name: template-project
variants:
  container:
    image: test:latest
    auth: none
`
	require.NoError(t, os.WriteFile(filepath.Join(ccDir, "workspace-template.yaml"), []byte(tmplContent), 0o644))

	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(origDir)

	stateFile := filepath.Join(tmpDir, "test-state.yaml")
	defFile := filepath.Join(tmpDir, "test-defs.yaml")
	t.Setenv("CC_DECK_STATE_FILE", stateFile)
	t.Setenv("CC_DECK_WORKSPACES_FILE", defFile)

	// Stub podman for container workspace creation.
	binDir := filepath.Join(t.TempDir(), "bin")
	require.NoError(t, os.MkdirAll(binDir, 0o755))
	for _, b := range []string{"podman"} {
		stub := filepath.Join(binDir, b)
		require.NoError(t, os.WriteFile(stub, []byte("#!/bin/sh\nexit 0\n"), 0o755))
	}
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))

	cmd, cf := newTestNewCmd()
	err := runWsNew(nil, "", cf, cmd)
	require.NoError(t, err)

	defs := ws.NewDefinitionStore(defFile)
	found, findErr := defs.FindByName("template-project")
	require.NoError(t, findErr)
	assert.Equal(t, ws.WorkspaceTypeContainer, found.Type)
	assert.Equal(t, "none", found.Auth)
}

func TestRunWsNew_ExplicitNameOverridesTemplate(t *testing.T) {
	ensureZellijStub(t)
	tmpDir := t.TempDir()
	require.NoError(t, exec.Command("git", "init", tmpDir).Run())

	ccDir := filepath.Join(tmpDir, ".cc-deck")
	require.NoError(t, os.MkdirAll(ccDir, 0o755))
	tmplContent := `name: template-name
variants:
  container:
    image: test:latest
`
	require.NoError(t, os.WriteFile(filepath.Join(ccDir, "workspace-template.yaml"), []byte(tmplContent), 0o644))

	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(origDir)

	stateFile := filepath.Join(tmpDir, "test-state.yaml")
	defFile := filepath.Join(tmpDir, "test-defs.yaml")
	t.Setenv("CC_DECK_STATE_FILE", stateFile)
	t.Setenv("CC_DECK_WORKSPACES_FILE", defFile)

	// Stub podman for container workspace.
	binDir := filepath.Join(t.TempDir(), "bin")
	require.NoError(t, os.MkdirAll(binDir, 0o755))
	stub := filepath.Join(binDir, "podman")
	require.NoError(t, os.WriteFile(stub, []byte("#!/bin/sh\nexit 0\n"), 0o755))
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))

	cmd, cf := newTestNewCmd()
	require.NoError(t, cmd.Flags().Set("type", "container"))
	err := runWsNew(nil, "custom-name", cf, cmd)
	require.NoError(t, err)

	defs := ws.NewDefinitionStore(defFile)
	_, findErr := defs.FindByName("custom-name")
	require.NoError(t, findErr)
}

func TestRunWsNew_CollisionHandling(t *testing.T) {
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

	// First: create "my-ws" as local.
	cmd1, cf1 := newTestNewCmd()
	require.NoError(t, runWsNew(nil, "my-ws", cf1, cmd1))

	// Second: create "my-ws" as local again (same type) -> error.
	cmd2, cf2 := newTestNewCmd()
	err := runWsNew(nil, "my-ws", cf2, cmd2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
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

func TestRunWsNew_SetsProjectDir(t *testing.T) {
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
	require.NoError(t, runWsNew(nil, "proj-ws", cf, cmd))

	defs := ws.NewDefinitionStore(defFile)
	found, err := defs.FindByName("proj-ws")
	require.NoError(t, err)
	assert.NotEmpty(t, found.ProjectDir)
}

func TestRunWsNew_MultiVariantWithoutTypeFails(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, exec.Command("git", "init", tmpDir).Run())

	ccDir := filepath.Join(tmpDir, ".cc-deck")
	require.NoError(t, os.MkdirAll(ccDir, 0o755))
	tmplContent := `name: multi
variants:
  ssh:
    host: user@host
  container:
    image: test:latest
`
	require.NoError(t, os.WriteFile(filepath.Join(ccDir, "workspace-template.yaml"), []byte(tmplContent), 0o644))

	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(origDir)

	stateFile := filepath.Join(tmpDir, "test-state.yaml")
	defFile := filepath.Join(tmpDir, "test-defs.yaml")
	t.Setenv("CC_DECK_STATE_FILE", stateFile)
	t.Setenv("CC_DECK_WORKSPACES_FILE", defFile)

	cmd, cf := newTestNewCmd()
	err := runWsNew(nil, "", cf, cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "template defines multiple variants")
}

func TestWriteWsStructured_IncludesProjectField(t *testing.T) {
	projectMap := map[string]string{"global-ws": "my-project"}

	instances := []*ws.WorkspaceInstance{
		{Name: "global-ws", Type: ws.WorkspaceTypeSSH, State: ws.WorkspaceStateRunning,
			SSH: &ws.SSHFields{Host: "user@host"}},
	}
	instanceNames := map[string]bool{"global-ws": true}

	old := os.Stdout
	r, w, pipeErr := os.Pipe()
	require.NoError(t, pipeErr)
	os.Stdout = w

	err := writeWsStructured("json", instances, nil, instanceNames, "", projectMap)
	require.NoError(t, err)

	w.Close()
	os.Stdout = old

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	assert.Contains(t, output, `"project"`)
	assert.Contains(t, output, `"my-project"`)
}

func TestWriteWsTable_HasProjectColumn(t *testing.T) {
	projectMap := map[string]string{"proj-ws": "my-project"}

	old := os.Stdout
	r, w, pipeErr := os.Pipe()
	require.NoError(t, pipeErr)
	os.Stdout = w

	err := writeWsTableWithProjects(nil, nil, map[string]bool{}, "", projectMap, false)
	require.NoError(t, err)

	w.Close()
	os.Stdout = old

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	assert.Contains(t, output, "PROJECT")
	assert.NotContains(t, output, "SOURCE")
}

func TestRunWsNew_DifferentTypeAutoSuffix(t *testing.T) {
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

	// Create "my-ws" as local.
	cmd1, cf1 := newTestNewCmd()
	require.NoError(t, runWsNew(nil, "my-ws", cf1, cmd1))

	// Create "my-ws" as container -> should auto-suffix to "my-ws-container".
	binDir := filepath.Join(t.TempDir(), "bin")
	require.NoError(t, os.MkdirAll(binDir, 0o755))
	stub := filepath.Join(binDir, "podman")
	require.NoError(t, os.WriteFile(stub, []byte("#!/bin/sh\nexit 0\n"), 0o755))
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))

	cmd2, cf2 := newTestNewCmd()
	require.NoError(t, cmd2.Flags().Set("type", "container"))
	require.NoError(t, cmd2.Flags().Set("image", "test:latest"))
	err := runWsNew(nil, "my-ws", cf2, cmd2)
	require.NoError(t, err)

	defs := ws.NewDefinitionStore(defFile)
	found, findErr := defs.FindByName("my-ws-container")
	require.NoError(t, findErr)
	assert.Equal(t, ws.WorkspaceTypeContainer, found.Type)
}

func TestWsPrune_IsNoOp(t *testing.T) {
	cmd := newWsPruneCmd()
	err := cmd.RunE(cmd, nil)
	require.NoError(t, err)
}
