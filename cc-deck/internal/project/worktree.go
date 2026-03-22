package project

import (
	"os/exec"
	"strings"
)

// WorktreeInfo describes a single git worktree.
type WorktreeInfo struct {
	Path   string // Absolute path to the worktree
	Branch string // Branch name (empty for detached HEAD)
	Bare   bool   // Whether this is a bare worktree
}

// ListWorktrees returns all worktrees for the git repository at gitRoot.
// Parses output of `git worktree list --porcelain`.
// Returns an empty list (not error) if the command fails (e.g., old git).
func ListWorktrees(gitRoot string) ([]WorktreeInfo, error) {
	cmd := exec.Command("git", "-C", gitRoot, "worktree", "list", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return nil, nil
	}
	return parseWorktreePorcelain(string(out)), nil
}

// parseWorktreePorcelain parses the porcelain output of `git worktree list`.
// Each worktree block is separated by a blank line. Fields are:
//
//	worktree <path>
//	HEAD <sha>
//	branch refs/heads/<name>
//	bare
func parseWorktreePorcelain(output string) []WorktreeInfo {
	var worktrees []WorktreeInfo
	var current *WorktreeInfo

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")

		if line == "" {
			if current != nil {
				worktrees = append(worktrees, *current)
				current = nil
			}
			continue
		}

		if strings.HasPrefix(line, "worktree ") {
			current = &WorktreeInfo{
				Path: strings.TrimPrefix(line, "worktree "),
			}
		} else if current != nil {
			if strings.HasPrefix(line, "branch ") {
				ref := strings.TrimPrefix(line, "branch ")
				current.Branch = strings.TrimPrefix(ref, "refs/heads/")
			} else if line == "bare" {
				current.Bare = true
			}
		}
	}

	// Handle final block without trailing newline.
	if current != nil {
		worktrees = append(worktrees, *current)
	}

	return worktrees
}
