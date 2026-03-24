package project

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// initGitRepo creates a bare git init in dir for testing.
func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "init", dir)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
	require.NoError(t, cmd.Run())
}

func TestFindGitRoot_InGitRepo(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	root, err := FindGitRoot(dir)
	require.NoError(t, err)
	// Compare canonical paths since TempDir may involve symlinks.
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

func TestFindProjectConfig_Found(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	ccDeckDir := filepath.Join(dir, ".cc-deck")
	require.NoError(t, os.MkdirAll(ccDeckDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(ccDeckDir, "environment.yaml"), []byte("name: test\n"), 0o644))

	root, err := FindProjectConfig(dir)
	require.NoError(t, err)
	assert.Equal(t, CanonicalPath(dir), root)
}

func TestFindProjectConfig_NotFound(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	_, err := FindProjectConfig(dir)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNoProjectConfig)
}

func TestFindProjectConfig_NotGitRepo(t *testing.T) {
	dir := t.TempDir()

	_, err := FindProjectConfig(dir)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNoProjectConfig)
}

func TestFindProjectConfig_FromSubdirectory(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	ccDeckDir := filepath.Join(dir, ".cc-deck")
	require.NoError(t, os.MkdirAll(ccDeckDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(ccDeckDir, "environment.yaml"), []byte("name: test\n"), 0o644))

	sub := filepath.Join(dir, "src", "pkg")
	require.NoError(t, os.MkdirAll(sub, 0o755))

	root, err := FindProjectConfig(sub)
	require.NoError(t, err)
	assert.Equal(t, CanonicalPath(dir), root)
}

// --- Workspace detection tests ---

func createWorkspaceConfig(t *testing.T, dir string) {
	t.Helper()
	ccDeckDir := filepath.Join(dir, ".cc-deck")
	require.NoError(t, os.MkdirAll(ccDeckDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(ccDeckDir, "environment.yaml"),
		[]byte("name: workspace-test\ntype: compose\n"), 0o644))
}

func TestFindWorkspaceRoot_Found(t *testing.T) {
	dir := t.TempDir()
	createWorkspaceConfig(t, dir)

	root, err := FindWorkspaceRoot(dir)
	require.NoError(t, err)
	assert.Equal(t, CanonicalPath(dir), root)
}

func TestFindWorkspaceRoot_FromSubdirectory(t *testing.T) {
	dir := t.TempDir()
	createWorkspaceConfig(t, dir)

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
	assert.ErrorIs(t, err, ErrNoWorkspaceConfig)
}

func TestFindProjectConfig_WorkspaceWithoutGit(t *testing.T) {
	// Workspace directory (no .git/) with .cc-deck/environment.yaml.
	dir := t.TempDir()
	createWorkspaceConfig(t, dir)

	sub := filepath.Join(dir, "some-subdir")
	require.NoError(t, os.MkdirAll(sub, 0o755))

	root, err := FindProjectConfig(sub)
	require.NoError(t, err)
	assert.Equal(t, CanonicalPath(dir), root)
}

func TestFindProjectConfig_GitRepoInsideWorkspace(t *testing.T) {
	// Workspace has .cc-deck/, child git repo does not.
	// Should find the workspace config via walk-up.
	workspace := t.TempDir()
	createWorkspaceConfig(t, workspace)

	gitRepo := filepath.Join(workspace, "repo-a")
	require.NoError(t, os.MkdirAll(gitRepo, 0o755))
	initGitRepo(t, gitRepo)

	root, err := FindProjectConfig(gitRepo)
	require.NoError(t, err)
	assert.Equal(t, CanonicalPath(workspace), root)
}

func TestFindProjectConfig_GitRepoWithOwnConfig(t *testing.T) {
	// Both workspace and git repo have .cc-deck/environment.yaml.
	// The git repo's own config should win (strategy 1).
	workspace := t.TempDir()
	createWorkspaceConfig(t, workspace)

	gitRepo := filepath.Join(workspace, "repo-b")
	require.NoError(t, os.MkdirAll(gitRepo, 0o755))
	initGitRepo(t, gitRepo)
	createWorkspaceConfig(t, gitRepo)

	root, err := FindProjectConfig(gitRepo)
	require.NoError(t, err)
	assert.Equal(t, CanonicalPath(gitRepo), root,
		"git repo with own .cc-deck/ should take precedence over workspace")
}

func TestCanonicalPath_RegularPath(t *testing.T) {
	dir := t.TempDir()
	result := CanonicalPath(dir)
	// Should be idempotent.
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
