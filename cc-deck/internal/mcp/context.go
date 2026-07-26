package mcp

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

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
	if len(s) > 500 {
		s = s[:500]
	}
	return s
}

// ReadGitBranch returns the current git branch name for the given directory.
// Returns an empty string if git is not available or the directory is not
// a git repository.
func ReadGitBranch(cwd string) string {
	if cwd == "" {
		return ""
	}
	out, err := exec.Command("git", "-C", cwd, "branch", "--show-current").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// ReadModifiedFiles returns a list of files with uncommitted changes in the
// given directory. Returns nil if git is not available or there are no changes.
func ReadModifiedFiles(cwd string) []string {
	if cwd == "" {
		return nil
	}
	out, err := exec.Command("git", "-C", cwd, "diff", "--name-only").Output()
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
func ReadGitDiffStat(cwd string) string {
	if cwd == "" {
		return ""
	}
	out, err := exec.Command("git", "-C", cwd, "diff", "--stat").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// ReadRecentCommits returns the last n commit messages (oneline format)
// for the given directory. Returns nil if git is not available.
func ReadRecentCommits(cwd string, n int) []string {
	if cwd == "" {
		return nil
	}
	out, err := exec.Command("git", "-C", cwd, "log", "--oneline", fmt.Sprintf("-%d", n)).Output()
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
