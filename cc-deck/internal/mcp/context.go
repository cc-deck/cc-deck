package mcp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// gitTimeout is the per-command timeout for git subprocesses.
const gitTimeout = 3 * time.Second

// ReadProjectSummary reads the first 500 characters of the CLAUDE.md file
// in the given directory. Returns an empty string if the file is not found
// or cannot be read.
func ReadProjectSummary(cwd string) string {
	if cwd == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(cwd, "CLAUDE.md"))
	if err != nil {
		return ""
	}
	s := string(data)
	runes := []rune(s)
	if len(runes) > 500 {
		s = string(runes[:500])
	}
	return s
}

// ReadMemoryIndex reads the first 500 characters of the MEMORY.md file
// from the Claude Code project-specific memory directory for the given CWD.
// Returns an empty string if the file is not found or cannot be read.
func ReadMemoryIndex(cwd string) string {
	if cwd == "" {
		return ""
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	// Claude Code stores project memory at ~/.claude/projects/{path-encoded}/memory/MEMORY.md
	// The path encoding replaces / with - and prepends -
	encoded := strings.ReplaceAll(cwd, "/", "-")
	memoryPath := filepath.Join(home, ".claude", "projects", encoded, "memory", "MEMORY.md")
	data, err := os.ReadFile(memoryPath)
	if err != nil {
		return ""
	}
	s := string(data)
	runes := []rune(s)
	if len(runes) > 500 {
		s = string(runes[:500])
	}
	return s
}

// ReadGitBranch returns the current git branch name for the given directory.
// Returns an empty string if git is not available or the directory is not
// a git repository.
func ReadGitBranch(ctx context.Context, cwd string) string {
	if cwd == "" {
		return ""
	}
	gitCtx, cancel := context.WithTimeout(ctx, gitTimeout)
	defer cancel()
	out, err := exec.CommandContext(gitCtx, "git", "-C", cwd, "branch", "--show-current").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// ReadModifiedFiles returns a list of files with uncommitted changes in the
// given directory. Returns nil if git is not available or there are no changes.
func ReadModifiedFiles(ctx context.Context, cwd string) []string {
	if cwd == "" {
		return nil
	}
	gitCtx, cancel := context.WithTimeout(ctx, gitTimeout)
	defer cancel()
	out, err := exec.CommandContext(gitCtx, "git", "-C", cwd, "diff", "--name-only").Output()
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var result []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

// ReadGitDiffStat returns the `git diff --stat` output for the given directory.
// Returns an empty string if git is not available or there are no changes.
func ReadGitDiffStat(ctx context.Context, cwd string) string {
	if cwd == "" {
		return ""
	}
	gitCtx, cancel := context.WithTimeout(ctx, gitTimeout)
	defer cancel()
	out, err := exec.CommandContext(gitCtx, "git", "-C", cwd, "diff", "--stat").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// ReadRecentCommits returns the last n commit messages (oneline format)
// for the given directory. Returns nil if git is not available.
func ReadRecentCommits(ctx context.Context, cwd string, n int) []string {
	if cwd == "" {
		return nil
	}
	gitCtx, cancel := context.WithTimeout(ctx, gitTimeout)
	defer cancel()
	out, err := exec.CommandContext(gitCtx, "git", "-C", cwd, "log", "--oneline", fmt.Sprintf("-%d", n)).Output()
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var result []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}
