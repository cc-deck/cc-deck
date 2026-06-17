package build

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadBaseImageRegistry(t *testing.T) {
	content := `openshell:
  - name: nvidia-upstream
    ref: ghcr.io/nvidia/openshell-community/sandboxes/base:latest
    default: true
  - name: rh-ubi
    ref: quay.io/aipcc/openshell-base:latest
container:
  - name: fedora-41
    ref: registry.fedoraproject.org/fedora:41
    default: true
`
	dir := t.TempDir()
	path := filepath.Join(dir, "base-images.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	reg, err := LoadBaseImageRegistry(path)
	require.NoError(t, err)
	assert.Len(t, reg.OpenShell, 2)
	assert.Len(t, reg.Container, 1)
	assert.Equal(t, "nvidia-upstream", reg.OpenShell[0].Name)
	assert.Equal(t, "ghcr.io/nvidia/openshell-community/sandboxes/base:latest", reg.OpenShell[0].Ref)
	assert.True(t, reg.OpenShell[0].Default)
	assert.False(t, reg.OpenShell[1].Default)
}

func TestLoadBaseImageRegistry_FileNotFound(t *testing.T) {
	_, err := LoadBaseImageRegistry("/nonexistent/base-images.yaml")
	assert.Error(t, err)
}

func TestLoadBaseImageRegistry_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "base-images.yaml")
	require.NoError(t, os.WriteFile(path, []byte("not: [valid: yaml:"), 0o644))

	_, err := LoadBaseImageRegistry(path)
	assert.Error(t, err)
}

func TestBaseImageRegistry_DefaultRef(t *testing.T) {
	reg := &BaseImageRegistry{
		OpenShell: []BaseImageEntry{
			{Name: "a", Ref: "img-a:latest", Default: false},
			{Name: "b", Ref: "img-b:latest", Default: true},
		},
		Container: []BaseImageEntry{
			{Name: "c", Ref: "img-c:latest", Default: true},
		},
	}

	assert.Equal(t, "img-b:latest", reg.DefaultRef("openshell"))
	assert.Equal(t, "img-c:latest", reg.DefaultRef("container"))
	assert.Equal(t, "", reg.DefaultRef("unknown"))
}

func TestBaseImageRegistry_DefaultRef_NoDefault(t *testing.T) {
	reg := &BaseImageRegistry{
		OpenShell: []BaseImageEntry{
			{Name: "a", Ref: "img-a:latest"},
		},
	}
	assert.Equal(t, "", reg.DefaultRef("openshell"))
}

func TestBaseImageRegistry_EntriesForTarget(t *testing.T) {
	reg := &BaseImageRegistry{
		OpenShell: []BaseImageEntry{
			{Name: "a", Ref: "img-a:latest"},
			{Name: "b", Ref: "img-b:latest"},
		},
	}

	entries := reg.EntriesForTarget("openshell")
	assert.Len(t, entries, 2)

	entries = reg.EntriesForTarget("container")
	assert.Nil(t, entries)

	entries = reg.EntriesForTarget("unknown")
	assert.Nil(t, entries)
}
