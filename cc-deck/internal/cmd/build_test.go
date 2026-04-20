package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- T001: Target auto-detection tests ---

func TestDetectRunTarget_ContainerfileOnly(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "container"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "container", "Containerfile"), []byte("FROM fedora\n"), 0o644))

	target, err := detectRunTarget(dir, "")
	require.NoError(t, err)
	assert.Equal(t, "container", target)
}

func TestDetectRunTarget_SSHOnly(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "ssh"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ssh", "site.yml"), []byte("---\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ssh", "inventory.ini"), []byte("[all]\n"), 0o644))

	target, err := detectRunTarget(dir, "")
	require.NoError(t, err)
	assert.Equal(t, "ssh", target)
}

func TestDetectRunTarget_BothPresent_Error(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "container"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "container", "Containerfile"), []byte("FROM fedora\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "ssh"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ssh", "site.yml"), []byte("---\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ssh", "inventory.ini"), []byte("[all]\n"), 0o644))

	_, err := detectRunTarget(dir, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--target")
}

func TestDetectRunTarget_NeitherPresent_Error(t *testing.T) {
	dir := t.TempDir()

	_, err := detectRunTarget(dir, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no build artifacts")
}

func TestDetectRunTarget_ExplicitOverride(t *testing.T) {
	dir := t.TempDir()
	// Even with no artifacts, explicit --target should work
	target, err := detectRunTarget(dir, "container")
	require.NoError(t, err)
	assert.Equal(t, "container", target)

	target, err = detectRunTarget(dir, "ssh")
	require.NoError(t, err)
	assert.Equal(t, "ssh", target)
}

// --- T002: Flag validation tests ---

func TestValidateRunFlags_PushWithSSH_Error(t *testing.T) {
	err := validateRunFlags("ssh", true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--push is only valid for container targets")
}

func TestValidateRunFlags_PushWithContainer_OK(t *testing.T) {
	err := validateRunFlags("container", true)
	require.NoError(t, err)
}

func TestValidateRunFlags_NoPush_OK(t *testing.T) {
	err := validateRunFlags("ssh", false)
	require.NoError(t, err)

	err = validateRunFlags("container", false)
	require.NoError(t, err)
}

func TestDetectRunTarget_InvalidExplicit_Error(t *testing.T) {
	dir := t.TempDir()

	_, err := detectRunTarget(dir, "invalid")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid target")
}

func TestDetectRunTarget_PartialSSH_SiteYmlOnly(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "ssh"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ssh", "site.yml"), []byte("---\n"), 0o644))

	_, err := detectRunTarget(dir, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no build artifacts")
}

func TestDetectRunTarget_PartialSSH_InventoryOnly(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "ssh"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ssh", "inventory.ini"), []byte("[all]\n"), 0o644))

	_, err := detectRunTarget(dir, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no build artifacts")
}
