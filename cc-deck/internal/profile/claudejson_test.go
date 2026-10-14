package profile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const defaultClaudeJSON = `{
  "oauthAccount": {"emailAddress": "work@example.com"},
  "userID": "default-user",
  "cachedGrowthBookFeatures": {"flag": true},
  "hasCompletedOnboarding": true,
  "lastOnboardingVersion": "2.1.0",
  "numStartups": 900,
  "mcpServers": {"excalidraw": {"command": "excalidraw-mcp"}},
  "projects": {
    "/src/cc-deck": {
      "hasTrustDialogAccepted": true,
      "allowedTools": ["Bash(make test)"],
      "mcpServers": {"local": {"command": "x"}},
      "enabledMcpjsonServers": ["jira"],
      "lastSessionId": "default-session",
      "lastCost": 12.5
    }
  }
}`

func writeJSON(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	return m
}

func TestMergeClaudeJSON_NewProfileGetsSharedKeysOnly(t *testing.T) {
	dir := t.TempDir()
	def := filepath.Join(dir, "home", ".claude.json")
	prof := filepath.Join(dir, "profile", ".claude.json")
	writeJSON(t, def, defaultClaudeJSON)
	require.NoError(t, os.MkdirAll(filepath.Dir(prof), 0o700))

	warning, err := mergeClaudeJSON(def, prof)
	require.NoError(t, err)
	assert.Empty(t, warning)

	got := readJSON(t, prof)
	assert.Equal(t, map[string]any{"excalidraw": map[string]any{"command": "excalidraw-mcp"}}, got["mcpServers"])
	assert.Equal(t, true, got["hasCompletedOnboarding"])
	assert.Equal(t, "2.1.0", got["lastOnboardingVersion"])
	for _, key := range []string{"oauthAccount", "userID", "cachedGrowthBookFeatures", "numStartups"} {
		assert.NotContains(t, got, key, "account and local state must not be copied")
	}

	project := got["projects"].(map[string]any)["/src/cc-deck"].(map[string]any)
	assert.Equal(t, true, project["hasTrustDialogAccepted"])
	assert.Equal(t, []any{"Bash(make test)"}, project["allowedTools"])
	assert.Equal(t, []any{"jira"}, project["enabledMcpjsonServers"])
	assert.NotContains(t, project, "lastSessionId", "session statistics stay per profile")
	assert.NotContains(t, project, "lastCost")

	info, err := os.Stat(prof)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestMergeClaudeJSON_ExistingProfileKeepsAccountAndOwnEntries(t *testing.T) {
	dir := t.TempDir()
	def := filepath.Join(dir, ".claude.json")
	prof := filepath.Join(dir, "profile", ".claude.json")
	writeJSON(t, def, defaultClaudeJSON)
	writeJSON(t, prof, `{
  "oauthAccount": {"emailAddress": "private@example.com"},
  "userID": "private-user",
  "hasCompletedOnboarding": false,
  "mcpServers": {
    "excalidraw": {"command": "stale"},
    "private-only": {"command": "mine"}
  },
  "projects": {
    "/src/cc-deck": {"hasTrustDialogAccepted": false, "lastSessionId": "private-session"},
    "/src/private": {"hasTrustDialogAccepted": true}
  }
}`)

	_, err := mergeClaudeJSON(def, prof)
	require.NoError(t, err)
	got := readJSON(t, prof)

	assert.Equal(t, "private@example.com", got["oauthAccount"].(map[string]any)["emailAddress"])
	assert.Equal(t, "private-user", got["userID"])
	assert.Equal(t, false, got["hasCompletedOnboarding"], "seeded fields never override the profile")

	servers := got["mcpServers"].(map[string]any)
	assert.Equal(t, "excalidraw-mcp", servers["excalidraw"].(map[string]any)["command"], "default wins by name")
	assert.Equal(t, "mine", servers["private-only"].(map[string]any)["command"], "profile-only servers stay")

	projects := got["projects"].(map[string]any)
	shared := projects["/src/cc-deck"].(map[string]any)
	assert.Equal(t, true, shared["hasTrustDialogAccepted"], "default trust wins")
	assert.Equal(t, "private-session", shared["lastSessionId"], "profile session state stays")
	assert.Contains(t, projects, "/src/private", "profile-only projects stay")
}

func TestMergeClaudeJSON_IdempotentAndNoRewriteWhenUnchanged(t *testing.T) {
	dir := t.TempDir()
	def := filepath.Join(dir, ".claude.json")
	prof := filepath.Join(dir, "profile", ".claude.json")
	writeJSON(t, def, defaultClaudeJSON)
	require.NoError(t, os.MkdirAll(filepath.Dir(prof), 0o700))

	_, err := mergeClaudeJSON(def, prof)
	require.NoError(t, err)
	first, err := os.ReadFile(prof)
	require.NoError(t, err)
	info1, err := os.Stat(prof)
	require.NoError(t, err)

	_, err = mergeClaudeJSON(def, prof)
	require.NoError(t, err)
	second, err := os.ReadFile(prof)
	require.NoError(t, err)
	info2, err := os.Stat(prof)
	require.NoError(t, err)

	assert.Equal(t, first, second)
	assert.True(t, os.SameFile(info1, info2), "an unchanged file must not be replaced")
}

func TestMergeClaudeJSON_MissingDefaultIsNoop(t *testing.T) {
	dir := t.TempDir()
	prof := filepath.Join(dir, ".claude.json")

	warning, err := mergeClaudeJSON(filepath.Join(dir, "absent.json"), prof)
	require.NoError(t, err)
	assert.Empty(t, warning)
	_, statErr := os.Stat(prof)
	assert.True(t, os.IsNotExist(statErr), "no profile file is created without a default")
}

func TestMergeClaudeJSON_InvalidJSONWarnsAndLeavesProfile(t *testing.T) {
	dir := t.TempDir()
	def := filepath.Join(dir, ".claude.json")
	prof := filepath.Join(dir, "profile", ".claude.json")

	writeJSON(t, def, "not json")
	writeJSON(t, prof, `{"userID": "p"}`)
	warning, err := mergeClaudeJSON(def, prof)
	require.NoError(t, err)
	assert.Contains(t, warning, "not valid JSON")
	data, _ := os.ReadFile(prof)
	assert.Equal(t, `{"userID": "p"}`, string(data))

	writeJSON(t, def, defaultClaudeJSON)
	writeJSON(t, prof, "{broken")
	warning, err = mergeClaudeJSON(def, prof)
	require.NoError(t, err)
	assert.True(t, strings.Contains(warning, prof), "warning names the profile file: %s", warning)
	data, _ = os.ReadFile(prof)
	assert.Equal(t, "{broken", string(data))
}

func TestDefaultClaudeJSONPath(t *testing.T) {
	assert.Equal(t, "/home/u/.claude.json", defaultClaudeJSONPath("/home/u/.claude"))
	assert.Equal(t, "/custom/cfg/.claude.json", defaultClaudeJSONPath("/custom/cfg"))
}

func TestClaudePrepareConfigDir_MergesClaudeJSONAndIsolatesBackups(t *testing.T) {
	home := t.TempDir()
	defaultDir := filepath.Join(home, ".claude")
	require.NoError(t, os.MkdirAll(filepath.Join(defaultDir, "backups"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(defaultDir, "settings.json"), []byte("{}"), 0o644))
	writeJSON(t, filepath.Join(home, ".claude.json"), defaultClaudeJSON)

	tr, ok := Lookup("claude")
	require.True(t, ok)
	rp := ResolvedProfile{Name: "private", ConfigDir: filepath.Join(home, "profiles", "private", "claude")}

	warnings, err := tr.PrepareConfigDir(rp, defaultDir)
	require.NoError(t, err)
	assert.Empty(t, warnings)

	_, err = os.Readlink(filepath.Join(rp.ConfigDir, "settings.json"))
	assert.NoError(t, err, "settings.json is still shared")
	_, err = os.Lstat(filepath.Join(rp.ConfigDir, "backups"))
	assert.True(t, os.IsNotExist(err), "backups must not be linked from the default directory")

	got := readJSON(t, filepath.Join(rp.ConfigDir, ".claude.json"))
	assert.Contains(t, got, "mcpServers")
	assert.NotContains(t, got, "oauthAccount")
}

func TestPrepareSharedDir_RemovesStaleLinkForIsolatedEntry(t *testing.T) {
	home := t.TempDir()
	defaultDir := filepath.Join(home, ".claude")
	require.NoError(t, os.MkdirAll(filepath.Join(defaultDir, "backups"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(defaultDir, "backups", "keep.json"), []byte("{}"), 0o644))
	configDir := filepath.Join(home, "profile")

	// Simulate an earlier sync that still shared backups.
	_, err := PrepareSharedDir(configDir, defaultDir, nil)
	require.NoError(t, err)
	_, err = os.Readlink(filepath.Join(configDir, "backups"))
	require.NoError(t, err)

	_, err = PrepareSharedDir(configDir, defaultDir, []string{"backups"})
	require.NoError(t, err)
	_, err = os.Lstat(filepath.Join(configDir, "backups"))
	assert.True(t, os.IsNotExist(err), "stale link removed")
	_, err = os.Stat(filepath.Join(defaultDir, "backups", "keep.json"))
	assert.NoError(t, err, "the link target is never touched")

	// A real directory under the isolated name is left alone.
	require.NoError(t, os.MkdirAll(filepath.Join(configDir, "backups"), 0o700))
	_, err = PrepareSharedDir(configDir, defaultDir, []string{"backups"})
	require.NoError(t, err)
	info, err := os.Lstat(filepath.Join(configDir, "backups"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}
