package oci

import (
	"archive/tar"
	"bytes"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTarLayer creates a v1.Layer from a map of file paths to contents.
func newTarLayer(t *testing.T, files map[string]string) v1.Layer {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for path, content := range files {
		hdr := &tar.Header{
			Name:     path,
			Size:     int64(len(content)),
			Mode:     0o644,
			Typeflag: tar.TypeReg,
		}
		require.NoError(t, tw.WriteHeader(hdr))
		_, err := tw.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())

	layer, err := tarball.LayerFromReader(&buf)
	require.NoError(t, err)
	return layer
}

// buildTestImage creates a test image with the given layers appended.
func buildTestImage(t *testing.T, layers ...v1.Layer) v1.Image {
	t.Helper()
	img := empty.Image
	var adds []mutate.Addendum
	for _, l := range layers {
		adds = append(adds, mutate.Addendum{Layer: l})
	}
	result, err := mutate.Append(img, adds...)
	require.NoError(t, err)
	return result
}

func TestFindLayerContaining_SingleLayer(t *testing.T) {
	layer := newTarLayer(t, map[string]string{
		"etc/openshell/policy.yaml": "policy: test",
	})
	img := buildTestImage(t, layer)

	diffID, err := layer.DiffID()
	require.NoError(t, err)

	found, err := FindLayerContaining(img, "/etc/openshell/policy.yaml")
	require.NoError(t, err)
	assert.Equal(t, diffID, found)
}

func TestFindLayerContaining_MultipleLayersReturnsTopmost(t *testing.T) {
	layer1 := newTarLayer(t, map[string]string{
		"etc/openshell/policy.yaml": "policy: old",
	})
	layer2 := newTarLayer(t, map[string]string{
		"etc/openshell/policy.yaml": "policy: new",
	})
	img := buildTestImage(t, layer1, layer2)

	topDiffID, err := layer2.DiffID()
	require.NoError(t, err)

	found, err := FindLayerContaining(img, "/etc/openshell/policy.yaml")
	require.NoError(t, err)
	assert.Equal(t, topDiffID, found, "should return the topmost layer containing the file")
}

func TestFindLayerContaining_FileNotFound(t *testing.T) {
	layer := newTarLayer(t, map[string]string{
		"etc/other/config.yaml": "config: test",
	})
	img := buildTestImage(t, layer)

	_, err := FindLayerContaining(img, "/etc/openshell/policy.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestFindLayerContaining_FileInMiddleLayer(t *testing.T) {
	layer1 := newTarLayer(t, map[string]string{
		"etc/base.conf": "base",
	})
	layer2 := newTarLayer(t, map[string]string{
		"etc/openshell/policy.yaml": "policy: middle",
	})
	layer3 := newTarLayer(t, map[string]string{
		"usr/bin/tool": "binary",
	})
	img := buildTestImage(t, layer1, layer2, layer3)

	middleDiffID, err := layer2.DiffID()
	require.NoError(t, err)

	found, err := FindLayerContaining(img, "/etc/openshell/policy.yaml")
	require.NoError(t, err)
	assert.Equal(t, middleDiffID, found)
}

func TestAddLabel_NewLabel(t *testing.T) {
	img := buildTestImage(t, newTarLayer(t, map[string]string{"test": "data"}))

	mutated, err := AddLabel(img, "dev.cc-deck.policy-layer", "sha256:abc123")
	require.NoError(t, err)

	cfg, err := mutated.ConfigFile()
	require.NoError(t, err)
	assert.Equal(t, "sha256:abc123", cfg.Config.Labels["dev.cc-deck.policy-layer"])
}

func TestAddLabel_OverwriteExisting(t *testing.T) {
	img := buildTestImage(t, newTarLayer(t, map[string]string{"test": "data"}))

	// Add initial label.
	img, err := AddLabel(img, "dev.cc-deck.policy-layer", "sha256:old")
	require.NoError(t, err)

	// Overwrite with new value.
	mutated, err := AddLabel(img, "dev.cc-deck.policy-layer", "sha256:new")
	require.NoError(t, err)

	cfg, err := mutated.ConfigFile()
	require.NoError(t, err)
	assert.Equal(t, "sha256:new", cfg.Config.Labels["dev.cc-deck.policy-layer"])
}

func TestStampPolicyLabel_InvalidRef(t *testing.T) {
	err := StampPolicyLabel(":::invalid", "/etc/openshell/policy.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing image reference")
}

func TestAddLabel_PreservesExistingLabels(t *testing.T) {
	base := buildTestImage(t, newTarLayer(t, map[string]string{"test": "data"}))

	// Add a pre-existing label.
	base, err := AddLabel(base, "existing.key", "existing-value")
	require.NoError(t, err)

	// Add the policy layer label.
	mutated, err := AddLabel(base, "dev.cc-deck.policy-layer", "sha256:abc")
	require.NoError(t, err)

	cfg, err := mutated.ConfigFile()
	require.NoError(t, err)
	assert.Equal(t, "existing-value", cfg.Config.Labels["existing.key"])
	assert.Equal(t, "sha256:abc", cfg.Config.Labels["dev.cc-deck.policy-layer"])
}
