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
	ErrNoProjectRoot   = errors.New("no .cc-deck/ directory found in project hierarchy")
	ErrNoWorkspaceRoot = errors.New("no .cc-deck/ directory found in directory hierarchy")
)

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

// FindProjectRoot looks for a .cc-deck/ directory using two strategies:
//  1. Check at the git root (fast, deterministic for single-repo projects).
//  2. Walk up the directory tree (supports workspace directories without .git).
//
// Returns the project root path and nil error if found.
func FindProjectRoot(startDir string) (string, error) {
	if root, err := FindGitRoot(startDir); err == nil {
		ccDeckDir := filepath.Join(root, ".cc-deck")
		if info, statErr := os.Stat(ccDeckDir); statErr == nil && info.IsDir() {
			return root, nil
		}
	}

	if root, err := FindWorkspaceRoot(startDir); err == nil {
		return root, nil
	}

	return "", ErrNoProjectRoot
}

// FindWorkspaceRoot walks up from startDir looking for a directory
// containing a .cc-deck/ subdirectory. Does not require a git repository.
// Returns the canonical path of the directory containing .cc-deck/.
func FindWorkspaceRoot(startDir string) (string, error) {
	absDir, err := filepath.Abs(startDir)
	if err != nil {
		return "", ErrNoWorkspaceRoot
	}
	dir := absDir
	for {
		ccDeckDir := filepath.Join(dir, ".cc-deck")
		if info, statErr := os.Stat(ccDeckDir); statErr == nil && info.IsDir() {
			return CanonicalPath(dir), nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", ErrNoWorkspaceRoot
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
// suitable for use as a default workspace name.
func ProjectName(projectRoot string) string {
	return filepath.Base(projectRoot)
}
