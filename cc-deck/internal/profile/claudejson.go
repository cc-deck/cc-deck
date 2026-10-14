package profile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Claude Code keeps its global state in a single .claude.json file: next to
// the default config directory (~/.claude.json) or, when CLAUDE_CONFIG_DIR is
// set, inside that directory. The file mixes account state (oauthAccount,
// userID, feature caches) with user configuration (user-scope MCP servers,
// per-project trust and permissions). A symlink would share the account, so
// the profile keeps its own file and sync merges only the configuration keys
// from the default file into it (FR-009a).

// sharedClaudeJSONProjectFields are the per-project fields that describe user
// configuration rather than session statistics. They are copied from the
// default file for every project it knows.
var sharedClaudeJSONProjectFields = []string{
	"allowedTools",
	"disabledMcpServers",
	"disabledMcpjsonServers",
	"enabledMcpjsonServers",
	"hasClaudeMdExternalIncludesApproved",
	"hasClaudeMdExternalIncludesWarningShown",
	"hasTrustDialogAccepted",
	"mcpContextUris",
	"mcpServers",
}

// seededClaudeJSONFields are copied only when the profile file does not have
// them yet, so a new profile skips the first-run onboarding without later
// syncs overriding what the profile session recorded.
var seededClaudeJSONFields = []string{
	"hasCompletedOnboarding",
	"lastOnboardingVersion",
}

// defaultClaudeJSONPath returns the .claude.json that belongs to the default
// config directory: ~/.claude.json for the conventional ~/.claude directory,
// otherwise the file inside the directory (a CLAUDE_CONFIG_DIR setup).
func defaultClaudeJSONPath(defaultDir string) string {
	if filepath.Base(defaultDir) == ".claude" {
		return filepath.Join(filepath.Dir(defaultDir), ".claude.json")
	}
	return filepath.Join(defaultDir, ".claude.json")
}

// mergeClaudeJSON merges the shared configuration from the default
// .claude.json into the profile's copy:
//   - user-scope mcpServers: every default server replaces the profile entry
//     of the same name; servers that exist only in the profile stay;
//   - projects: for every project in the default file, the fields in
//     sharedClaudeJSONProjectFields replace the profile's values; all other
//     project fields and profile-only projects stay;
//   - seededClaudeJSONFields are set only when the profile lacks them.
//
// Everything else in the profile file (account, caches, counters) is kept.
// The profile file is written atomically with mode 0600 and only when its
// content changes. A missing default file is not an error; an unparsable one
// returns a warning and leaves the profile file untouched.
func mergeClaudeJSON(defaultPath, profilePath string) (warning string, err error) {
	defaultData, err := os.ReadFile(defaultPath)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read %s: %w", defaultPath, err)
	}
	var def map[string]json.RawMessage
	if err := json.Unmarshal(defaultData, &def); err != nil {
		return fmt.Sprintf("%s is not valid JSON; MCP servers and project trust were not shared", defaultPath), nil
	}

	prof := map[string]json.RawMessage{}
	profileData, err := os.ReadFile(profilePath)
	switch {
	case err == nil:
		if err := json.Unmarshal(profileData, &prof); err != nil {
			return fmt.Sprintf("%s is not valid JSON; left unchanged", profilePath), nil
		}
	case os.IsNotExist(err):
		// First sync for this profile: start from an empty file.
	default:
		return "", fmt.Errorf("read %s: %w", profilePath, err)
	}

	if err := mergeNamedObjects(prof, def, "mcpServers"); err != nil {
		return "", err
	}
	if err := mergeProjects(prof, def); err != nil {
		return "", err
	}
	for _, key := range seededClaudeJSONFields {
		if _, ok := prof[key]; !ok {
			if v, ok := def[key]; ok {
				prof[key] = v
			}
		}
	}

	if len(prof) == 0 {
		// Nothing to share and no profile file yet: let the harness create it.
		return "", nil
	}

	out, err := json.MarshalIndent(prof, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode %s: %w", profilePath, err)
	}
	out = append(out, '\n')
	if bytes.Equal(out, profileData) {
		return "", nil
	}
	return "", writeFileAtomic(profilePath, out, 0o600)
}

// mergeNamedObjects copies every entry of the object def[key] into
// prof[key], replacing entries with the same name and keeping the rest.
func mergeNamedObjects(prof, def map[string]json.RawMessage, key string) error {
	raw, ok := def[key]
	if !ok {
		return nil
	}
	var defEntries map[string]json.RawMessage
	if err := json.Unmarshal(raw, &defEntries); err != nil {
		// Not an object in the default file: nothing sensible to merge.
		return nil
	}
	profEntries := map[string]json.RawMessage{}
	if existing, ok := prof[key]; ok {
		_ = json.Unmarshal(existing, &profEntries)
	}
	for name, v := range defEntries {
		profEntries[name] = v
	}
	merged, err := json.Marshal(profEntries)
	if err != nil {
		return fmt.Errorf("encode %s: %w", key, err)
	}
	prof[key] = merged
	return nil
}

// mergeProjects copies the shared per-project fields of every default project
// into the profile's project entry, creating the entry when needed.
func mergeProjects(prof, def map[string]json.RawMessage) error {
	raw, ok := def["projects"]
	if !ok {
		return nil
	}
	var defProjects map[string]map[string]json.RawMessage
	if err := json.Unmarshal(raw, &defProjects); err != nil {
		return nil
	}
	profProjects := map[string]map[string]json.RawMessage{}
	if existing, ok := prof["projects"]; ok {
		_ = json.Unmarshal(existing, &profProjects)
	}
	for path, defFields := range defProjects {
		entry := profProjects[path]
		if entry == nil {
			entry = map[string]json.RawMessage{}
		}
		for _, field := range sharedClaudeJSONProjectFields {
			if v, ok := defFields[field]; ok {
				entry[field] = v
			}
		}
		if len(entry) > 0 {
			profProjects[path] = entry
		}
	}
	merged, err := json.Marshal(profProjects)
	if err != nil {
		return fmt.Errorf("encode projects: %w", err)
	}
	prof["projects"] = merged
	return nil
}

// writeFileAtomic writes data to a temporary file next to path and renames it
// into place, so a concurrently starting harness never reads a partial file.
func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".claude.json.tmp-*")
	if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after a successful rename

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
