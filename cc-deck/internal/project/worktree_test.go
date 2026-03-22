package project

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseWorktreePorcelain_SingleWorktree(t *testing.T) {
	input := "worktree /home/user/project\nHEAD abc123\nbranch refs/heads/main\n\n"
	wts := parseWorktreePorcelain(input)
	require.Len(t, wts, 1)
	assert.Equal(t, "/home/user/project", wts[0].Path)
	assert.Equal(t, "main", wts[0].Branch)
	assert.False(t, wts[0].Bare)
}

func TestParseWorktreePorcelain_MultipleWorktrees(t *testing.T) {
	input := `worktree /home/user/project
HEAD abc123
branch refs/heads/main

worktree /home/user/project-feature
HEAD def456
branch refs/heads/feature/auth

`
	wts := parseWorktreePorcelain(input)
	require.Len(t, wts, 2)
	assert.Equal(t, "/home/user/project", wts[0].Path)
	assert.Equal(t, "main", wts[0].Branch)
	assert.Equal(t, "/home/user/project-feature", wts[1].Path)
	assert.Equal(t, "feature/auth", wts[1].Branch)
}

func TestParseWorktreePorcelain_BareWorktree(t *testing.T) {
	input := "worktree /home/user/bare-repo\nHEAD abc123\nbare\n\n"
	wts := parseWorktreePorcelain(input)
	require.Len(t, wts, 1)
	assert.Equal(t, "/home/user/bare-repo", wts[0].Path)
	assert.True(t, wts[0].Bare)
	assert.Empty(t, wts[0].Branch)
}

func TestParseWorktreePorcelain_DetachedHead(t *testing.T) {
	input := "worktree /home/user/project\nHEAD abc123\ndetached\n\n"
	wts := parseWorktreePorcelain(input)
	require.Len(t, wts, 1)
	assert.Empty(t, wts[0].Branch)
	assert.False(t, wts[0].Bare)
}

func TestParseWorktreePorcelain_Empty(t *testing.T) {
	wts := parseWorktreePorcelain("")
	assert.Empty(t, wts)
}

func TestParseWorktreePorcelain_NoTrailingNewline(t *testing.T) {
	input := "worktree /home/user/project\nHEAD abc123\nbranch refs/heads/main"
	wts := parseWorktreePorcelain(input)
	require.Len(t, wts, 1)
	assert.Equal(t, "main", wts[0].Branch)
}

func TestListWorktrees_Integration(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	// Create an initial commit so worktree list works.
	cmd := exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "init")
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
	require.NoError(t, cmd.Run())

	wts, err := ListWorktrees(dir)
	require.NoError(t, err)
	require.Len(t, wts, 1)
	assert.Equal(t, CanonicalPath(dir), CanonicalPath(wts[0].Path))
}

func TestListWorktrees_WithLinkedWorktree(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	// Create an initial commit.
	cmd := exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "init")
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
	require.NoError(t, cmd.Run())

	// Add a linked worktree.
	wtPath := filepath.Join(dir, "worktree-feature")
	cmd = exec.Command("git", "-C", dir, "worktree", "add", wtPath, "-b", "feature")
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
	require.NoError(t, cmd.Run())

	wts, err := ListWorktrees(dir)
	require.NoError(t, err)
	require.Len(t, wts, 2)

	// Find the linked worktree.
	var found bool
	for _, wt := range wts {
		if wt.Branch == "feature" {
			found = true
			assert.Equal(t, CanonicalPath(wtPath), CanonicalPath(wt.Path))
		}
	}
	assert.True(t, found, "expected to find worktree with branch 'feature'")
}

func TestListWorktrees_NotGitRepo(t *testing.T) {
	dir := t.TempDir()

	wts, err := ListWorktrees(dir)
	require.NoError(t, err)
	assert.Empty(t, wts)
}
