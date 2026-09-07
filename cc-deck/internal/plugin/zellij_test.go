package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckCompatibility(t *testing.T) {
	cases := []struct {
		version string
		want    string
	}{
		{"0.44.3", "incompatible"}, // below the minimum: could load the controller twice
		{"0.40.0", "incompatible"},
		{"0.45.0", "compatible"},
		{"0.45.1", "compatible"},
		{"0.46.0", "untested"}, // newer than anything verified
		{"1.0.0", "untested"},
		{"garbage", "untested"},
		{"", "untested"},
	}
	for _, c := range cases {
		if got := CheckCompatibility(c.version, MaxTestedZellijVersion); got != c.want {
			t.Errorf("CheckCompatibility(%q) = %q, want %q", c.version, got, c.want)
		}
	}
}

func TestEmbeddedPluginVersionBounds(t *testing.T) {
	info := EmbeddedPlugin()
	if info.MinZellij != MinZellijVersion {
		t.Errorf("MinZellij = %q, want %q", info.MinZellij, MinZellijVersion)
	}
	if info.MaxTested != MaxTestedZellijVersion {
		t.Errorf("MaxTested = %q, want %q", info.MaxTested, MaxTestedZellijVersion)
	}
	if CheckCompatibility(info.MinZellij+".0", info.MaxTested) != "compatible" {
		t.Errorf("the minimum version must itself be compatible")
	}
}

func readPermissions(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "permissions.kdl"))
	if err != nil {
		t.Fatalf("reading permissions.kdl: %v", err)
	}
	return string(data)
}

func TestEnsurePluginPermissionsCreatesAndIsIdempotent(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "zellij-cache") // does not exist yet
	plugins := "/home/u/.config/zellij/plugins"

	changed, err := EnsurePluginPermissions(cache, plugins)
	if err != nil || !changed {
		t.Fatalf("first call: changed=%v err=%v, want true, nil", changed, err)
	}
	got := readPermissions(t, cache)
	for _, p := range RequiredPermissions {
		if !strings.Contains(got, "    "+p+"\n") {
			t.Errorf("missing permission %s in:\n%s", p, got)
		}
	}
	if !strings.Contains(got, `"/home/u/.config/zellij/plugins/cc_deck.wasm" {`) {
		t.Errorf("missing plugin entry in:\n%s", got)
	}

	changed, err = EnsurePluginPermissions(cache, plugins)
	if err != nil || changed {
		t.Fatalf("second call: changed=%v err=%v, want false, nil (nothing to repair)", changed, err)
	}
	if readPermissions(t, cache) != got {
		t.Errorf("idempotent call rewrote the file")
	}
}

func TestEnsurePluginPermissionsRepairsStaleEntryAndKeepsOthers(t *testing.T) {
	cache := t.TempDir()
	plugins := "/p"
	stale := `"file:/other/plugin.wasm" {
    ReadApplicationState
}
"/p/cc_deck.wasm" {
    ReadApplicationState
}
`
	if err := os.WriteFile(filepath.Join(cache, "permissions.kdl"), []byte(stale), 0644); err != nil {
		t.Fatal(err)
	}

	changed, err := EnsurePluginPermissions(cache, plugins)
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v, want true, nil", changed, err)
	}
	got := readPermissions(t, cache)
	if !strings.Contains(got, `"file:/other/plugin.wasm" {`) {
		t.Errorf("foreign plugin entry was dropped:\n%s", got)
	}
	if strings.Count(got, "/p/cc_deck.wasm") != 1 {
		t.Errorf("expected exactly one cc-deck entry:\n%s", got)
	}
	if !strings.Contains(got, "    WriteToStdin\n") {
		t.Errorf("stale entry was not upgraded:\n%s", got)
	}
}

// The plugin asks for exactly the permissions the CLI seeds. If the two
// drift, the controller blocks on a grant that never comes.
func TestRequiredPermissionsMatchPluginSource(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "..", "cc-zellij-plugin", "src", "lib.rs"))
	if err != nil {
		t.Skipf("plugin source not available: %v", err)
	}
	text := string(src)
	start := strings.Index(text, "pub const REQUIRED_PERMISSIONS")
	if start < 0 {
		t.Fatal("REQUIRED_PERMISSIONS not found in lib.rs")
	}
	end := strings.Index(text[start:], "];")
	block := text[start : start+end]
	var fromRust []string
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "PermissionType::") {
			fromRust = append(fromRust, strings.TrimSuffix(strings.TrimPrefix(line, "PermissionType::"), ","))
		}
	}
	if !sameStringSet(fromRust, RequiredPermissions) {
		t.Errorf("permission sets differ:\n rust: %v\n go:   %v", fromRust, RequiredPermissions)
	}
}
