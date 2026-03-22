package env

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsureCCDeckGitignore_CreatesNew(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, EnsureCCDeckGitignore(dir))

	content, err := os.ReadFile(filepath.Join(dir, ".cc-deck", ".gitignore"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "status.yaml")
	assert.Contains(t, string(content), "run/")
}

func TestEnsureCCDeckGitignore_Idempotent(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, EnsureCCDeckGitignore(dir))
	require.NoError(t, EnsureCCDeckGitignore(dir))

	content, err := os.ReadFile(filepath.Join(dir, ".cc-deck", ".gitignore"))
	require.NoError(t, err)

	// Count occurrences: each entry should appear exactly once.
	s := string(content)
	assert.Equal(t, 1, countOccurrences(s, "status.yaml"))
	assert.Equal(t, 1, countOccurrences(s, "run/"))
}

func TestEnsureCCDeckGitignore_PreservesExisting(t *testing.T) {
	dir := t.TempDir()
	ccDir := filepath.Join(dir, ".cc-deck")
	require.NoError(t, os.MkdirAll(ccDir, 0o755))

	// Write a gitignore with existing content.
	existing := "image/\n"
	require.NoError(t, os.WriteFile(filepath.Join(ccDir, ".gitignore"), []byte(existing), 0o644))

	require.NoError(t, EnsureCCDeckGitignore(dir))

	content, err := os.ReadFile(filepath.Join(ccDir, ".gitignore"))
	require.NoError(t, err)
	s := string(content)
	assert.Contains(t, s, "image/")
	assert.Contains(t, s, "status.yaml")
	assert.Contains(t, s, "run/")
}

func TestEnsureCCDeckGitignore_PartialExisting(t *testing.T) {
	dir := t.TempDir()
	ccDir := filepath.Join(dir, ".cc-deck")
	require.NoError(t, os.MkdirAll(ccDir, 0o755))

	// Already has status.yaml but not run/.
	existing := "status.yaml\n"
	require.NoError(t, os.WriteFile(filepath.Join(ccDir, ".gitignore"), []byte(existing), 0o644))

	require.NoError(t, EnsureCCDeckGitignore(dir))

	content, err := os.ReadFile(filepath.Join(ccDir, ".gitignore"))
	require.NoError(t, err)
	s := string(content)
	assert.Equal(t, 1, countOccurrences(s, "status.yaml"))
	assert.Equal(t, 1, countOccurrences(s, "run/"))
}

func countOccurrences(s, substr string) int {
	count := 0
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			count++
		}
	}
	return count
}
