package build

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadManifest_WithNetworkSection(t *testing.T) {
	content := `
version: 1
image:
  name: my-image
  tag: latest
network:
  allowed_domains:
    - python
    - golang
    - custom.example.com
`
	dir := t.TempDir()
	path := filepath.Join(dir, "cc-deck-build.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	m, err := LoadManifest(path)
	require.NoError(t, err)

	require.NotNil(t, m.Network)
	assert.Equal(t, []string{"python", "golang", "custom.example.com"}, m.Network.AllowedDomains)
}

func TestLoadManifest_WithoutNetworkSection(t *testing.T) {
	content := `
version: 1
image:
  name: my-image
`
	dir := t.TempDir()
	path := filepath.Join(dir, "cc-deck-build.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	m, err := LoadManifest(path)
	require.NoError(t, err)
	assert.Nil(t, m.Network)
}

func TestManifest_Validate(t *testing.T) {
	m := &Manifest{Version: 1, Image: ImageConfig{Name: "test"}}
	assert.NoError(t, m.Validate())

	m2 := &Manifest{Version: 0}
	assert.Error(t, m2.Validate())
}
