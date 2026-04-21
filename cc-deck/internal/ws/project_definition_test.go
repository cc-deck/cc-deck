package ws

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadProjectDefinition_Success(t *testing.T) {
	dir := t.TempDir()
	ccDir := filepath.Join(dir, ".cc-deck")
	require.NoError(t, os.MkdirAll(ccDir, 0o755))

	content := `name: my-api
type: compose
image: quay.io/my-image:latest
env:
  FOO: bar
  BAZ: qux
`
	require.NoError(t, os.WriteFile(filepath.Join(ccDir, "workspace.yaml"), []byte(content), 0o644))

	def, err := LoadProjectDefinition(dir)
	require.NoError(t, err)
	assert.Equal(t, "my-api", def.Name)
	assert.Equal(t, WorkspaceTypeCompose, def.Type)
	assert.Equal(t, "quay.io/my-image:latest", def.Image)
	assert.Equal(t, map[string]string{"FOO": "bar", "BAZ": "qux"}, def.Env)
}

func TestLoadProjectDefinition_NotFound(t *testing.T) {
	dir := t.TempDir()

	_, err := LoadProjectDefinition(dir)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotFound))
}

func TestSaveProjectDefinition_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()

	def := &WorkspaceDefinition{
		Name:  "my-project",
		Type:  WorkspaceTypeContainer,
		Image: "test:latest",
	}

	require.NoError(t, SaveProjectDefinition(dir, def))

	// Verify the file was created.
	loaded, err := LoadProjectDefinition(dir)
	require.NoError(t, err)
	assert.Equal(t, "my-project", loaded.Name)
	assert.Equal(t, WorkspaceTypeContainer, loaded.Type)
	assert.Equal(t, "test:latest", loaded.Image)
}

func TestSaveProjectDefinition_CreatesGitignore(t *testing.T) {
	dir := t.TempDir()

	def := &WorkspaceDefinition{
		Name: "test",
		Type: WorkspaceTypeCompose,
	}

	require.NoError(t, SaveProjectDefinition(dir, def))

	gitignorePath := filepath.Join(dir, ".cc-deck", ".gitignore")
	content, err := os.ReadFile(gitignorePath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "status.yaml")
	assert.Contains(t, string(content), "run/")
}

func TestSaveProjectDefinition_AtomicWrite(t *testing.T) {
	dir := t.TempDir()

	def := &WorkspaceDefinition{
		Name: "atomic-test",
		Type: WorkspaceTypeContainer,
	}

	require.NoError(t, SaveProjectDefinition(dir, def))

	tmpPath := filepath.Join(dir, ".cc-deck", "workspace.yaml.tmp")
	_, err := os.Stat(tmpPath)
	assert.True(t, os.IsNotExist(err), ".tmp file should not linger")
}

func TestSaveProjectDefinition_WithEnvVars(t *testing.T) {
	dir := t.TempDir()

	def := &WorkspaceDefinition{
		Name:  "env-test",
		Type:  WorkspaceTypeCompose,
		Image: "my-image:latest",
		Env:   map[string]string{"MY_VAR": "value"},
	}

	require.NoError(t, SaveProjectDefinition(dir, def))

	loaded, err := LoadProjectDefinition(dir)
	require.NoError(t, err)
	assert.Equal(t, "value", loaded.Env["MY_VAR"])
}

func TestSaveProjectDefinition_Roundtrip(t *testing.T) {
	dir := t.TempDir()

	def := &WorkspaceDefinition{
		Name:           "roundtrip",
		Type:           WorkspaceTypeCompose,
		Image:          "img:v1",
		Auth:           "vertex",
		Ports:          []string{"8080:8080"},
		AllowedDomains: []string{"gcp"},
		Env:            map[string]string{"X": "Y"},
	}

	require.NoError(t, SaveProjectDefinition(dir, def))

	loaded, err := LoadProjectDefinition(dir)
	require.NoError(t, err)
	assert.Equal(t, def.Name, loaded.Name)
	assert.Equal(t, def.Type, loaded.Type)
	assert.Equal(t, def.Image, loaded.Image)
	assert.Equal(t, def.Auth, loaded.Auth)
	assert.Equal(t, def.Ports, loaded.Ports)
	assert.Equal(t, def.AllowedDomains, loaded.AllowedDomains)
	assert.Equal(t, def.Env, loaded.Env)
}
