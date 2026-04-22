package project

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "init", dir)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
	require.NoError(t, cmd.Run())
}

func createCCDeckDir(t *testing.T, dir string) {
	t.Helper()
	ccDeckDir := filepath.Join(dir, ".cc-deck")
	require.NoError(t, os.MkdirAll(ccDeckDir, 0o755))
}

func TestFindGitRoot_InGitRepo(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	root, err := FindGitRoot(dir)
	require.NoError(t, err)
	assert.Equal(t, CanonicalPath(dir), root)
}

func TestFindGitRoot_Subdirectory(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	sub := filepath.Join(dir, "src", "pkg")
	require.NoError(t, os.MkdirAll(sub, 0o755))

	root, err := FindGitRoot(sub)
	require.NoError(t, err)
	assert.Equal(t, CanonicalPath(dir), root)
}

func TestFindGitRoot_NotGitRepo(t *testing.T) {
	dir := t.TempDir()

	_, err := FindGitRoot(dir)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNotGitRepo)
}

func TestFindProjectRoot_Found(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	createCCDeckDir(t, dir)

	root, err := FindProjectRoot(dir)
	require.NoError(t, err)
	assert.Equal(t, CanonicalPath(dir), root)
}

func TestFindProjectRoot_NotFound(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	_, err := FindProjectRoot(dir)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNoProjectRoot)
}

func TestFindProjectRoot_NotGitRepo(t *testing.T) {
	dir := t.TempDir()

	_, err := FindProjectRoot(dir)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNoProjectRoot)
}

func TestFindProjectRoot_FromSubdirectory(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	createCCDeckDir(t, dir)

	sub := filepath.Join(dir, "src", "pkg")
	require.NoError(t, os.MkdirAll(sub, 0o755))

	root, err := FindProjectRoot(sub)
	require.NoError(t, err)
	assert.Equal(t, CanonicalPath(dir), root)
}

func TestFindWorkspaceRoot_Found(t *testing.T) {
	dir := t.TempDir()
	createCCDeckDir(t, dir)

	root, err := FindWorkspaceRoot(dir)
	require.NoError(t, err)
	assert.Equal(t, CanonicalPath(dir), root)
}

func TestFindWorkspaceRoot_FromSubdirectory(t *testing.T) {
	dir := t.TempDir()
	createCCDeckDir(t, dir)

	sub := filepath.Join(dir, "repo-a", "src", "pkg")
	require.NoError(t, os.MkdirAll(sub, 0o755))

	root, err := FindWorkspaceRoot(sub)
	require.NoError(t, err)
	assert.Equal(t, CanonicalPath(dir), root)
}

func TestFindWorkspaceRoot_NotFound(t *testing.T) {
	dir := t.TempDir()

	_, err := FindWorkspaceRoot(dir)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNoWorkspaceRoot)
}

func TestFindProjectRoot_WorkspaceWithoutGit(t *testing.T) {
	dir := t.TempDir()
	createCCDeckDir(t, dir)

	sub := filepath.Join(dir, "some-subdir")
	require.NoError(t, os.MkdirAll(sub, 0o755))

	root, err := FindProjectRoot(sub)
	require.NoError(t, err)
	assert.Equal(t, CanonicalPath(dir), root)
}

func TestFindProjectRoot_GitRepoInsideWorkspace(t *testing.T) {
	workspace := t.TempDir()
	createCCDeckDir(t, workspace)

	gitRepo := filepath.Join(workspace, "repo-a")
	require.NoError(t, os.MkdirAll(gitRepo, 0o755))
	initGitRepo(t, gitRepo)

	root, err := FindProjectRoot(gitRepo)
	require.NoError(t, err)
	assert.Equal(t, CanonicalPath(workspace), root)
}

func TestFindProjectRoot_GitRepoWithOwnCCDeck(t *testing.T) {
	workspace := t.TempDir()
	createCCDeckDir(t, workspace)

	gitRepo := filepath.Join(workspace, "repo-b")
	require.NoError(t, os.MkdirAll(gitRepo, 0o755))
	initGitRepo(t, gitRepo)
	createCCDeckDir(t, gitRepo)

	root, err := FindProjectRoot(gitRepo)
	require.NoError(t, err)
	assert.Equal(t, CanonicalPath(gitRepo), root,
		"git repo with own .cc-deck/ should take precedence over workspace")
}

func TestCanonicalPath_RegularPath(t *testing.T) {
	dir := t.TempDir()
	result := CanonicalPath(dir)
	assert.Equal(t, result, CanonicalPath(result))
}

func TestCanonicalPath_Symlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	require.NoError(t, os.MkdirAll(target, 0o755))

	link := filepath.Join(dir, "link")
	require.NoError(t, os.Symlink(target, link))

	resolved := CanonicalPath(link)
	assert.Equal(t, CanonicalPath(target), resolved)
}

func TestCanonicalPath_NonexistentPath(t *testing.T) {
	path := "/nonexistent/path/that/does/not/exist"
	result := CanonicalPath(path)
	assert.Equal(t, path, result)
}

func TestProjectName(t *testing.T) {
	assert.Equal(t, "my-project", ProjectName("/home/user/projects/my-project"))
	assert.Equal(t, "repo", ProjectName("/tmp/repo"))
}
