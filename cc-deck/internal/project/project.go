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
	ErrNotGitRepo        = errors.New("not inside a git repository")
	ErrNoProjectConfig   = errors.New("no .cc-deck/workspace.yaml found in project hierarchy")
	ErrNoWorkspaceConfig = errors.New("no .cc-deck/workspace.yaml found in directory hierarchy")
)

const projectConfigPath = ".cc-deck/workspace.yaml"

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

// FindProjectConfig looks for .cc-deck/workspace.yaml using two strategies:
//  1. Check at the git root (fast, deterministic for single-repo projects).
//  2. Walk up the directory tree (supports workspace directories without .git).
//
// Returns the project or workspace root path and nil error if found.
func FindProjectConfig(startDir string) (string, error) {
	// Strategy 1: Check at the git root (existing behavior, fast path).
	if root, err := FindGitRoot(startDir); err == nil {
		configFile := filepath.Join(root, projectConfigPath)
		if _, statErr := os.Stat(configFile); statErr == nil {
			return root, nil
		}
	}

	// Strategy 2: Walk up for workspace detection.
	if root, err := FindWorkspaceRoot(startDir); err == nil {
		return root, nil
	}

	return "", ErrNoProjectConfig
}

// FindWorkspaceRoot walks up from startDir looking for a directory
// containing .cc-deck/workspace.yaml. Unlike FindProjectConfig's
// git-root strategy, this does not require a git repository.
// Returns the canonical path of the directory containing .cc-deck/.
func FindWorkspaceRoot(startDir string) (string, error) {
	absDir, err := filepath.Abs(startDir)
	if err != nil {
		return "", ErrNoWorkspaceConfig
	}
	dir := absDir
	for {
		configFile := filepath.Join(dir, projectConfigPath)
		if _, err := os.Stat(configFile); err == nil {
			return CanonicalPath(dir), nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", ErrNoWorkspaceConfig
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
