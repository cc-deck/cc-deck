package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name      string
		version   string
		wantMajor int
		wantMinor int
		wantOk    bool
	}{
		{"standard version", "0.43.1", 0, 43, true},
		{"major only, no minor", "0", 0, 0, false},
		{"empty string", "", 0, 0, false},
		{"two-part version", "1.2", 1, 2, true},
		{"whitespace padded", "  0.40.0  ", 0, 40, true},
		{"non-numeric major", "a.40", 0, 0, false},
		{"non-numeric minor", "0.b", 0, 0, false},
		{"large numbers", "12.34.56", 12, 34, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			major, minor, ok := parseVersion(tt.version)
			if ok != tt.wantOk {
				t.Fatalf("parseVersion(%q) ok = %v, want %v", tt.version, ok, tt.wantOk)
			}
			if !tt.wantOk {
				return
			}
			if major != tt.wantMajor || minor != tt.wantMinor {
				t.Errorf("parseVersion(%q) = (%d, %d), want (%d, %d)", tt.version, major, minor, tt.wantMajor, tt.wantMinor)
			}
		})
	}
}

func TestCheckCompatibility(t *testing.T) {
	tests := []struct {
		name          string
		zellijVersion string
		sdkVersion    string
		want          string
	}{
		{"below minimum supported", "0.39.0", "0.44", "incompatible"},
		{"exactly minimum supported", "0.40.0", "0.44", "compatible"},
		{"equal to sdk version", "0.44.0", "0.44", "compatible"},
		{"below sdk version", "0.42.0", "0.44", "compatible"},
		{"above sdk version", "0.45.0", "0.44", "untested"},
		{"unparsable zellij version", "not-a-version", "0.44", "untested"},
		{"unparsable sdk version", "0.44.0", "not-a-version", "untested"},
		{"lower major than sdk", "0.44.0", "1.0", "compatible"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckCompatibility(tt.zellijVersion, tt.sdkVersion)
			if got != tt.want {
				t.Errorf("CheckCompatibility(%q, %q) = %q, want %q", tt.zellijVersion, tt.sdkVersion, got, tt.want)
			}
		})
	}
}

func TestPluginLocation(t *testing.T) {
	got := PluginLocation("/home/user/.config/zellij/plugins")
	want := "file:/home/user/.config/zellij/plugins/cc_deck.wasm"
	if got != want {
		t.Errorf("PluginLocation() = %q, want %q", got, want)
	}
}

func TestResolveZellijConfigDir_EnvOverride(t *testing.T) {
	t.Setenv("ZELLIJ_CONFIG_DIR", "/custom/zellij/config")
	got := resolveZellijConfigDir()
	if got != "/custom/zellij/config" {
		t.Errorf("resolveZellijConfigDir() = %q, want %q", got, "/custom/zellij/config")
	}
}

func TestResolveZellijConfigDir_DefaultsToHomeConfig(t *testing.T) {
	t.Setenv("ZELLIJ_CONFIG_DIR", "")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory in this environment")
	}
	got := resolveZellijConfigDir()
	want := filepath.Join(home, ".config", "zellij")
	if got != want {
		t.Errorf("resolveZellijConfigDir() = %q, want %q", got, want)
	}
}

func TestResolveZellijCacheDir_XDGOverride(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "/custom/cache")
	got := resolveZellijCacheDir()
	want := filepath.Join("/custom/cache", "zellij")
	if got != want {
		t.Errorf("resolveZellijCacheDir() = %q, want %q", got, want)
	}
}

func TestDetectZellij_NotInstalled(t *testing.T) {
	t.Setenv("PATH", "")
	info := DetectZellij()
	if info.Installed {
		t.Error("expected Installed to be false when zellij is not on PATH")
	}
	if info.Version != "" {
		t.Errorf("expected empty Version, got %q", info.Version)
	}
	if info.BinaryPath != "" {
		t.Errorf("expected empty BinaryPath, got %q", info.BinaryPath)
	}
}

func TestEnsurePluginPermissions_CreatesNewFile(t *testing.T) {
	dir := t.TempDir()
	pluginsDir := filepath.Join(dir, "plugins")

	if err := EnsurePluginPermissions(dir, pluginsDir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "permissions.kdl"))
	if err != nil {
		t.Fatalf("expected permissions.kdl to be created: %v", err)
	}
	pluginPath := filepath.Join(pluginsDir, "cc_deck.wasm")
	if !strings.Contains(string(content), pluginPath) {
		t.Errorf("expected permissions content to reference %q, got %q", pluginPath, content)
	}
}

func TestEnsurePluginPermissions_ReplacesStaleEntry(t *testing.T) {
	dir := t.TempDir()
	pluginsDir := filepath.Join(dir, "plugins")
	permPath := filepath.Join(dir, "permissions.kdl")
	pluginPath := filepath.Join(pluginsDir, "cc_deck.wasm")

	stale := "\"" + pluginPath + "\" {\n    OldPermission\n}\n"
	if err := os.WriteFile(permPath, []byte(stale), 0644); err != nil {
		t.Fatalf("failed to seed permissions.kdl: %v", err)
	}

	if err := EnsurePluginPermissions(dir, pluginsDir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(permPath)
	if err != nil {
		t.Fatalf("failed to read updated permissions.kdl: %v", err)
	}
	if strings.Contains(string(content), "OldPermission") {
		t.Errorf("expected stale entry to be removed, got %q", content)
	}
	if !strings.Contains(string(content), "MessageAndLaunchOtherPlugins") {
		t.Errorf("expected new entry to be present, got %q", content)
	}
}

func TestEnsurePluginPermissions_AppendsToExistingContent(t *testing.T) {
	dir := t.TempDir()
	pluginsDir := filepath.Join(dir, "plugins")
	permPath := filepath.Join(dir, "permissions.kdl")

	existing := "\"file:/some/other.wasm\" {\n    ReadApplicationState\n}"
	if err := os.WriteFile(permPath, []byte(existing), 0644); err != nil {
		t.Fatalf("failed to seed permissions.kdl: %v", err)
	}

	if err := EnsurePluginPermissions(dir, pluginsDir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(permPath)
	if err != nil {
		t.Fatalf("failed to read updated permissions.kdl: %v", err)
	}
	if !strings.Contains(string(content), "/some/other.wasm") {
		t.Errorf("expected existing entry to be preserved, got %q", content)
	}
	pluginPath := filepath.Join(pluginsDir, "cc_deck.wasm")
	if !strings.Contains(string(content), pluginPath) {
		t.Errorf("expected new entry to be appended, got %q", content)
	}
}

