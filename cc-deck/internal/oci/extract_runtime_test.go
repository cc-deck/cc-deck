package oci

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeFakeExecutable creates an executable file named `name` inside dir so
// that exec.LookPath can find it when PATH is restricted to dir.
func writeFakeExecutable(t *testing.T, dir, name string) {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755))
}

func TestDetectRuntime_NotFound(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PATH manipulation test is unix-specific")
	}
	emptyDir := t.TempDir()
	t.Setenv("PATH", emptyDir)

	_, err := detectRuntime()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "neither podman nor docker found")
}

func TestDetectRuntime_FindsPodman(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PATH manipulation test is unix-specific")
	}
	dir := t.TempDir()
	writeFakeExecutable(t, dir, "podman")
	t.Setenv("PATH", dir)

	rt, err := detectRuntime()
	require.NoError(t, err)
	assert.Equal(t, "podman", rt)
}

func TestDetectRuntime_FindsDockerWhenNoPodman(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PATH manipulation test is unix-specific")
	}
	dir := t.TempDir()
	writeFakeExecutable(t, dir, "docker")
	t.Setenv("PATH", dir)

	rt, err := detectRuntime()
	require.NoError(t, err)
	assert.Equal(t, "docker", rt)
}

func TestDetectRuntime_PrefersPodmanOverDocker(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PATH manipulation test is unix-specific")
	}
	dir := t.TempDir()
	writeFakeExecutable(t, dir, "podman")
	writeFakeExecutable(t, dir, "docker")
	t.Setenv("PATH", dir)

	rt, err := detectRuntime()
	require.NoError(t, err)
	assert.Equal(t, "podman", rt)
}

func TestExtractLocal_NoRuntimeFound(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PATH manipulation test is unix-specific")
	}
	emptyDir := t.TempDir()
	t.Setenv("PATH", emptyDir)

	_, err := extractLocal("some-image:latest", "/etc/openshell/policy.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "neither podman nor docker found")
}

func TestExtractRemote_InvalidRef(t *testing.T) {
	_, err := extractRemote(":::invalid", "/etc/openshell/policy.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing image reference")
}

func TestExtractRemote_RegistryUnreachable(t *testing.T) {
	// Port 1 is a reserved/unassigned port that should refuse connections
	// immediately, giving a fast, deterministic "not found" failure without
	// depending on network access to a real registry.
	_, err := extractRemote("127.0.0.1:1/does-not-exist:latest", "/etc/openshell/policy.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found in remote registry")
}

func TestExtractFileFromImage_LocalFailsRemoteFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PATH manipulation test is unix-specific")
	}
	emptyDir := t.TempDir()
	t.Setenv("PATH", emptyDir)

	_, err := ExtractFileFromImage(":::invalid", "/etc/openshell/policy.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing image reference")
}
