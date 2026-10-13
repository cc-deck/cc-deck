package profile

import (
	"fmt"
	"os"
	"path/filepath"
)

// PrepareSharedDir creates configDir (mode 0700) and symlinks non-isolated
// entries from defaultDir into it. Returns a list of warnings for entries
// that could not be linked (e.g., real files already exist).
func PrepareSharedDir(configDir, defaultDir string, isolated []string) (warnings []string, err error) {
	// If the default config directory doesn't exist yet, there is nothing
	// to share. Return early without error so callers that run before first
	// harness launch are unaffected.
	if _, statErr := os.Stat(defaultDir); os.IsNotExist(statErr) {
		// Still create the config dir itself so the caller can proceed.
		if mkErr := os.MkdirAll(configDir, 0700); mkErr != nil {
			return nil, fmt.Errorf("create config dir: %w", mkErr)
		}
		return nil, nil
	}

	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, fmt.Errorf("create config dir: %w", err)
	}

	entries, err := os.ReadDir(defaultDir)
	if err != nil {
		return nil, fmt.Errorf("read default dir: %w", err)
	}

	// Compute relative path from configDir to defaultDir so symlinks are
	// portable across renames of a common ancestor.
	relBase, err := filepath.Rel(configDir, defaultDir)
	if err != nil {
		// Fall back to absolute if Rel fails (different volumes, etc.).
		relBase = defaultDir
	}

	for _, entry := range entries {
		name := entry.Name()
		if isIsolated(name, isolated) {
			continue
		}

		linkPath := filepath.Join(configDir, name)
		target := filepath.Join(relBase, name)

		info, lstatErr := os.Lstat(linkPath)
		if os.IsNotExist(lstatErr) {
			// Entry absent in configDir: create the symlink.
			if symlinkErr := os.Symlink(target, linkPath); symlinkErr != nil {
				return warnings, fmt.Errorf("symlink %s: %w", name, symlinkErr)
			}
			continue
		}
		if lstatErr != nil {
			return warnings, fmt.Errorf("stat %s: %w", name, lstatErr)
		}

		// Entry exists. If it is a symlink, check whether it already
		// points at the right target (idempotent) or needs repointing.
		if info.Mode()&os.ModeSymlink != 0 {
			existing, readErr := os.Readlink(linkPath)
			if readErr != nil {
				return warnings, fmt.Errorf("readlink %s: %w", name, readErr)
			}
			if existing == target {
				// Already correct, nothing to do.
				continue
			}
			// Different target: repoint by removing and recreating.
			if removeErr := os.Remove(linkPath); removeErr != nil {
				return warnings, fmt.Errorf("repoint %s: remove: %w", name, removeErr)
			}
			if symlinkErr := os.Symlink(target, linkPath); symlinkErr != nil {
				return warnings, fmt.Errorf("repoint %s: symlink: %w", name, symlinkErr)
			}
			continue
		}

		// Regular file or directory: leave it in place, record a warning.
		warnings = append(warnings, fmt.Sprintf("%s: not a symlink, left unchanged", name))
	}

	return warnings, nil
}

// isIsolated reports whether name appears in the isolated list.
func isIsolated(name string, isolated []string) bool {
	for _, iso := range isolated {
		if iso == name {
			return true
		}
	}
	return false
}
