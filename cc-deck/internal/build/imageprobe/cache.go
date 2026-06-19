package imageprobe

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const cacheFileName = "probe-cache.json"
const cacheVersion = 1

// CachePath returns the path to the probe cache file within a setup directory.
func CachePath(setupDir string) string {
	return filepath.Join(setupDir, cacheFileName)
}

// LoadCache reads the probe cache from disk. Returns an empty cache
// if the file does not exist.
func LoadCache(setupDir string) (*ProbeCache, error) {
	path := CachePath(setupDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ProbeCache{
				Version: cacheVersion,
				Entries: make(map[string]ProbeResult),
			}, nil
		}
		return nil, fmt.Errorf("reading probe cache: %w", err)
	}

	var cache ProbeCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, fmt.Errorf("parsing probe cache: %w", err)
	}

	if cache.Entries == nil {
		cache.Entries = make(map[string]ProbeResult)
	}

	return &cache, nil
}

// SaveCache writes the probe cache to disk.
func SaveCache(setupDir string, cache *ProbeCache) error {
	path := CachePath(setupDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating cache directory: %w", err)
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling probe cache: %w", err)
	}

	return os.WriteFile(path, data, 0o644)
}

// LookupCache checks for a cached probe result matching the given image
// reference and digest. Returns the result and true on a hit, or
// zero-value and false on a miss.
func LookupCache(cache *ProbeCache, imageRef, digest string) (ProbeResult, bool) {
	entry, ok := cache.Entries[imageRef]
	if !ok {
		return ProbeResult{}, false
	}
	if entry.ImageDigest != digest {
		return ProbeResult{}, false
	}
	return entry, true
}

// StoreResult adds or updates a cached probe result.
func StoreResult(cache *ProbeCache, result *ProbeResult) {
	cache.Entries[result.ImageRef] = *result
}
