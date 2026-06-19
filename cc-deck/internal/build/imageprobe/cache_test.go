package imageprobe

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadCache_NoFile(t *testing.T) {
	dir := t.TempDir()
	cache, err := LoadCache(dir)
	require.NoError(t, err)
	assert.Equal(t, cacheVersion, cache.Version)
	assert.NotNil(t, cache.Entries)
	assert.Empty(t, cache.Entries)
}

func TestSaveAndLoadCache_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	cache := &ProbeCache{
		Version: cacheVersion,
		Entries: map[string]ProbeResult{
			"fedora:41": {
				ImageRef:       "fedora:41",
				ImageDigest:    "sha256:abc123",
				Timestamp:      time.Date(2026, 6, 17, 10, 0, 0, 0, time.UTC),
				PackageManager: "dnf",
				OS:             OSInfo{ID: "fedora"},
				Tools:          map[string]ToolInfo{"git": {Name: "git", Present: true}},
			},
		},
	}

	require.NoError(t, SaveCache(dir, cache))

	loaded, err := LoadCache(dir)
	require.NoError(t, err)
	assert.Len(t, loaded.Entries, 1)

	entry := loaded.Entries["fedora:41"]
	assert.Equal(t, "sha256:abc123", entry.ImageDigest)
	assert.Equal(t, "dnf", entry.PackageManager)
}

func TestLookupCache_Hit(t *testing.T) {
	cache := &ProbeCache{
		Version: cacheVersion,
		Entries: map[string]ProbeResult{
			"fedora:41": {
				ImageRef:    "fedora:41",
				ImageDigest: "sha256:abc123",
			},
		},
	}

	result, hit := LookupCache(cache, "fedora:41", "sha256:abc123")
	assert.True(t, hit)
	assert.Equal(t, "fedora:41", result.ImageRef)
}

func TestLookupCache_MissDifferentDigest(t *testing.T) {
	cache := &ProbeCache{
		Version: cacheVersion,
		Entries: map[string]ProbeResult{
			"fedora:41": {
				ImageRef:    "fedora:41",
				ImageDigest: "sha256:abc123",
			},
		},
	}

	_, hit := LookupCache(cache, "fedora:41", "sha256:def456")
	assert.False(t, hit)
}

func TestLookupCache_MissNoEntry(t *testing.T) {
	cache := &ProbeCache{
		Version: cacheVersion,
		Entries: map[string]ProbeResult{},
	}

	_, hit := LookupCache(cache, "fedora:41", "sha256:abc123")
	assert.False(t, hit)
}

func TestStoreResult(t *testing.T) {
	cache := &ProbeCache{
		Version: cacheVersion,
		Entries: map[string]ProbeResult{},
	}

	result := &ProbeResult{
		ImageRef:    "ubi9:latest",
		ImageDigest: "sha256:xyz789",
	}

	StoreResult(cache, result)
	assert.Len(t, cache.Entries, 1)
	assert.Equal(t, "sha256:xyz789", cache.Entries["ubi9:latest"].ImageDigest)
}

func TestCachePath(t *testing.T) {
	path := CachePath("/some/setup/dir")
	assert.Equal(t, filepath.Join("/some/setup/dir", "probe-cache.json"), path)
}

func TestLoadCache_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, cacheFileName)
	require.NoError(t, os.WriteFile(path, []byte("not json"), 0o644))

	_, err := LoadCache(dir)
	assert.Error(t, err)
}
