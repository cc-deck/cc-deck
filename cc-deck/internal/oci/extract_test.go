package oci

import (
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// labelImage adds a label to a test image's config.
func labelImage(t *testing.T, img v1.Image, key, value string) v1.Image {
	t.Helper()
	result, err := AddLabel(img, key, value)
	require.NoError(t, err)
	return result
}

func TestExtractViaLabel_Success(t *testing.T) {
	policyContent := "policy:\n  sandbox: restricted\n"
	layer := newTarLayer(t, map[string]string{
		"etc/openshell/policy.yaml": policyContent,
	})
	img := buildTestImage(t, layer)

	diffID, err := layer.DiffID()
	require.NoError(t, err)

	img = labelImage(t, img, PolicyLayerLabel, diffID.String())

	data, err := extractViaLabel(img, "/etc/openshell/policy.yaml")
	require.NoError(t, err)
	assert.Equal(t, policyContent, string(data))
}

func TestExtractViaLabel_NoLabel(t *testing.T) {
	layer := newTarLayer(t, map[string]string{
		"etc/openshell/policy.yaml": "policy: test",
	})
	img := buildTestImage(t, layer)

	_, err := extractViaLabel(img, "/etc/openshell/policy.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no dev.cc-deck.policy-layer label")
}

func TestExtractViaLabel_StaleLabel(t *testing.T) {
	layer := newTarLayer(t, map[string]string{
		"etc/openshell/policy.yaml": "policy: test",
	})
	img := buildTestImage(t, layer)

	// Label points to a nonexistent layer.
	img = labelImage(t, img, PolicyLayerLabel, "sha256:0000000000000000000000000000000000000000000000000000000000000000")

	_, err := extractViaLabel(img, "/etc/openshell/policy.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "stale label")
}

func TestExtractViaLabel_FileNotInLabeledLayer(t *testing.T) {
	layer1 := newTarLayer(t, map[string]string{
		"etc/openshell/policy.yaml": "policy: here",
	})
	layer2 := newTarLayer(t, map[string]string{
		"usr/bin/tool": "binary",
	})
	img := buildTestImage(t, layer1, layer2)

	// Label points to layer2 which does not contain the policy file.
	diffID2, err := layer2.DiffID()
	require.NoError(t, err)
	img = labelImage(t, img, PolicyLayerLabel, diffID2.String())

	_, err = extractViaLabel(img, "/etc/openshell/policy.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not in labeled layer")
}

func TestExtractViaLayerScan_Success(t *testing.T) {
	policyContent := "policy:\n  sandbox: open\n"
	layer := newTarLayer(t, map[string]string{
		"etc/openshell/policy.yaml": policyContent,
	})
	img := buildTestImage(t, layer)

	data, err := extractViaLayerScan(img, "/etc/openshell/policy.yaml")
	require.NoError(t, err)
	assert.Equal(t, policyContent, string(data))
}

func TestExtractViaLayerScan_MultipleLayersReturnsTopmost(t *testing.T) {
	layer1 := newTarLayer(t, map[string]string{
		"etc/openshell/policy.yaml": "policy: old",
	})
	layer2 := newTarLayer(t, map[string]string{
		"etc/openshell/policy.yaml": "policy: new",
	})
	img := buildTestImage(t, layer1, layer2)

	data, err := extractViaLayerScan(img, "/etc/openshell/policy.yaml")
	require.NoError(t, err)
	assert.Equal(t, "policy: new", string(data))
}

func TestExtractViaLayerScan_FileNotFound(t *testing.T) {
	layer := newTarLayer(t, map[string]string{
		"etc/other/config.yaml": "config: test",
	})
	img := buildTestImage(t, layer)

	_, err := extractViaLayerScan(img, "/etc/openshell/policy.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestExtractFileFromLayer_Success(t *testing.T) {
	content := "hello world"
	layer := newTarLayer(t, map[string]string{
		"etc/openshell/policy.yaml": content,
	})

	data, err := extractFileFromLayer(layer, "/etc/openshell/policy.yaml")
	require.NoError(t, err)
	assert.Equal(t, content, string(data))
}

func TestExtractFileFromLayer_NotFound(t *testing.T) {
	layer := newTarLayer(t, map[string]string{
		"etc/other.yaml": "other",
	})

	_, err := extractFileFromLayer(layer, "/etc/openshell/policy.yaml")
	assert.Error(t, err)
}

// TestExtractViaLabel_FallbackToScan tests the full extraction flow: label
// extraction fails (stale label), then fallback scan succeeds.
func TestExtractFullFlow_StaleLabelFallsBackToScan(t *testing.T) {
	policyContent := "policy:\n  sandbox: fallback\n"
	layer := newTarLayer(t, map[string]string{
		"etc/openshell/policy.yaml": policyContent,
	})
	img := buildTestImage(t, layer)

	// Set a stale label.
	img = labelImage(t, img, PolicyLayerLabel, "sha256:0000000000000000000000000000000000000000000000000000000000000000")

	// extractViaLabel should fail.
	_, err := extractViaLabel(img, "/etc/openshell/policy.yaml")
	assert.Error(t, err)

	// extractViaLayerScan should succeed as fallback.
	data, err := extractViaLayerScan(img, "/etc/openshell/policy.yaml")
	require.NoError(t, err)
	assert.Equal(t, policyContent, string(data))
}

// TestExtractViaLayerScan_UnlabeledMultiLayerImage verifies fallback behavior
// for images built before label stamping was introduced (User Story 3).
func TestExtractViaLayerScan_UnlabeledMultiLayerImage(t *testing.T) {
	baseLayer := newTarLayer(t, map[string]string{
		"usr/bin/bash": "#!/bin/bash",
		"etc/os-release": "ID=fedora",
	})
	toolsLayer := newTarLayer(t, map[string]string{
		"usr/bin/go":    "go binary",
		"usr/bin/rustc": "rustc binary",
	})
	policyLayer := newTarLayer(t, map[string]string{
		"etc/openshell/policy.yaml": "policy:\n  sandbox: production\n",
	})
	topLayer := newTarLayer(t, map[string]string{
		"usr/local/bin/entrypoint.sh": "#!/bin/bash\nexec $@",
	})

	img := buildTestImage(t, baseLayer, toolsLayer, policyLayer, topLayer)

	// Verify: no labels on the image, simulating a pre-labeling build.
	cfg, err := img.ConfigFile()
	require.NoError(t, err)
	assert.Empty(t, cfg.Config.Labels)

	data, err := extractViaLayerScan(img, "/etc/openshell/policy.yaml")
	require.NoError(t, err)
	assert.Equal(t, "policy:\n  sandbox: production\n", string(data))
}

// TestBackwardCompatibility_UnlabeledImageExtraction is the dedicated test for
// User Story 3: images built before the labeling feature was introduced should
// still work for sandbox creation via the fallback layer scan. This test creates
// a realistic multi-layer image without any labels and verifies the correct
// policy file is extracted from the topmost layer that contains it.
func TestBackwardCompatibility_UnlabeledImageExtraction(t *testing.T) {
	// Simulate a typical openshell image with multiple layers:
	// Layer 0: base OS files
	// Layer 1: development tools
	// Layer 2: policy file (COPY'd during build)
	// Layer 3: entrypoint and runtime config

	osLayer := newTarLayer(t, map[string]string{
		"etc/os-release":   "ID=fedora\nVERSION_ID=41\n",
		"usr/bin/bash":     "#!/bin/bash",
		"usr/bin/coreutils": "coreutils binary",
	})
	toolsLayer := newTarLayer(t, map[string]string{
		"usr/bin/go":      "go binary",
		"usr/bin/node":    "node binary",
		"usr/bin/python3": "python3 binary",
	})
	policyLayer := newTarLayer(t, map[string]string{
		"etc/openshell/policy.yaml": "version: 1\npolicy:\n  sandbox:\n    allow_network: false\n    allow_fs_write: [/sandbox]\n",
	})
	entrypointLayer := newTarLayer(t, map[string]string{
		"usr/local/bin/entrypoint.sh": "#!/bin/bash\nexec \"$@\"",
		"etc/openshell/config.yaml":   "runtime:\n  shell: /usr/bin/bash\n",
	})

	img := buildTestImage(t, osLayer, toolsLayer, policyLayer, entrypointLayer)

	// Verify: no labels on the image (pre-labeling build).
	cfg, err := img.ConfigFile()
	require.NoError(t, err)
	assert.Empty(t, cfg.Config.Labels, "unlabeled image should have no labels")

	// The fallback scan should find the policy in layer 2 (policyLayer).
	data, err := extractViaLayerScan(img, "/etc/openshell/policy.yaml")
	require.NoError(t, err)
	assert.Contains(t, string(data), "allow_network: false")
	assert.Contains(t, string(data), "allow_fs_write:")
}

// Ensure mutate import is used (needed for buildTestImage via label_test.go,
// but this file also uses it indirectly through labelImage).
var _ = mutate.Config
