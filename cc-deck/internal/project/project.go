// Package project provides functions for discovering git repositories,
// project-local .cc-deck configurations, and git worktree information.
package project

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	ErrNotGitRepo      = errors.New("not inside a git repository")
	ErrNoProjectConfig = errors.New("no .cc-deck/environment.yaml found at git root")
)

const projectConfigPath = ".cc-deck/environment.yaml"

// FindGitRoot returns the git root directory for the given start path.
// Uses `git rev-parse --show-toplevel` for reliable detection.
// Returns the canonical (symlink-resolved) path.
func FindGitRoot(startDir string) (string, error) {
	cmd := exec.Command("git", "-C", startDir, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", ErrNotGitRepo
	}
	root := strings.TrimSpace(string(out))
	return CanonicalPath(root), nil
}

// FindProjectConfig looks for .cc-deck/environment.yaml at the git root
// of the given start directory. Returns the project root path (git root)
// and nil error if found.
func FindProjectConfig(startDir string) (string, error) {
	root, err := FindGitRoot(startDir)
	if err != nil {
		return "", err
	}

	configFile := filepath.Join(root, projectConfigPath)
	if _, err := os.Stat(configFile); err != nil {
		return "", ErrNoProjectConfig
	}

	return root, nil
}

// CanonicalPath returns the symlink-resolved absolute path.
// Falls back to the original path if resolution fails.
func CanonicalPath(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}
	return resolved
}

// ProjectName returns the directory basename of the given path,
// suitable for use as a default environment name.
func ProjectName(projectRoot string) string {
	return filepath.Base(projectRoot)
}
