package share

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnsureZellijWebSharing_AlreadyEnabled(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.kdl")
	require.NoError(t, os.WriteFile(p, []byte("web_sharing \"on\"\n"), 0644))
	changed, err := EnsureZellijWebSharing(p)
	require.NoError(t, err)
	require.False(t, changed)
}

func TestEnsureZellijWebSharing_CommentedOut(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.kdl")
	require.NoError(t, os.WriteFile(p, []byte("// web_sharing \"on\"  // some comment\n"), 0644))
	changed, err := EnsureZellijWebSharing(p)
	require.NoError(t, err)
	require.True(t, changed)
	data, _ := os.ReadFile(p)
	require.Contains(t, string(data), "web_sharing \"on\"")
	require.NotContains(t, string(data), "// web_sharing \"on\"")
}

func TestEnsureZellijWebSharing_Missing(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.kdl")
	require.NoError(t, os.WriteFile(p, []byte("theme \"default\"\n"), 0644))
	changed, err := EnsureZellijWebSharing(p)
	require.NoError(t, err)
	require.True(t, changed)
	data, _ := os.ReadFile(p)
	require.Contains(t, string(data), "web_sharing \"on\"")
}

func TestEnsureZellijWebSharing_NoFile(t *testing.T) {
	_, err := EnsureZellijWebSharing(filepath.Join(t.TempDir(), "missing.kdl"))
	require.ErrorContains(t, err, "config not found")
}

func TestEnsureZellijWebSharing_CommentedWithDifferentValue(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.kdl")
	require.NoError(t, os.WriteFile(p, []byte("// web_sharing \"off\"\n"), 0644))
	changed, err := EnsureZellijWebSharing(p)
	require.NoError(t, err)
	require.True(t, changed)
	data, _ := os.ReadFile(p)
	require.Contains(t, string(data), "web_sharing \"on\"")
}
