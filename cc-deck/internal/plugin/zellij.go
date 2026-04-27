package plugin

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// ZellijInfo describes the local Zellij installation.
type ZellijInfo struct {
	Installed  bool
	Version    string
	BinaryPath string
	ConfigDir  string
	PluginsDir string
	LayoutsDir string
	CacheDir   string
}

// DetectZellij probes the system for a Zellij installation.
// It runs "zellij --version" to determine the version and binary path,
// then resolves config, plugins, and layouts directories.
func DetectZellij() ZellijInfo {
	info := ZellijInfo{}

	binaryPath, err := exec.LookPath("zellij")
	if err != nil {
		return info
	}
	info.BinaryPath = binaryPath
	info.Installed = true

	out, err := exec.Command(binaryPath, "--version").Output()
	if err == nil {
		// Output is typically "zellij 0.43.0\n"
		version := strings.TrimSpace(string(out))
		version = strings.TrimPrefix(version, "zellij ")
		info.Version = version
	}

	info.ConfigDir = resolveZellijConfigDir()
	info.PluginsDir = filepath.Join(info.ConfigDir, "plugins")
	info.LayoutsDir = filepath.Join(info.ConfigDir, "layouts")
	info.CacheDir = resolveZellijCacheDir()

	return info
}

// resolveZellijConfigDir returns the Zellij config directory.
// It checks ZELLIJ_CONFIG_DIR first, then falls back to ~/.config/zellij/.
func resolveZellijConfigDir() string {
	if dir := os.Getenv("ZELLIJ_CONFIG_DIR"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join("~", ".config", "zellij")
	}
	return filepath.Join(home, ".config", "zellij")
}

// resolveZellijCacheDir returns the Zellij cache directory.
func resolveZellijCacheDir() string {
	// macOS: ~/Library/Caches/org.Zellij-Contributors.Zellij
	// Linux: ~/.cache/zellij
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "~"
	}
	// Check macOS path first
	macDir := filepath.Join(home, "Library", "Caches", "org.Zellij-Contributors.Zellij")
	if info, err := os.Stat(macDir); err == nil && info.IsDir() {
		return macDir
	}
	// Linux fallback
	if cacheDir := os.Getenv("XDG_CACHE_HOME"); cacheDir != "" {
		return filepath.Join(cacheDir, "zellij")
	}
	return filepath.Join(home, ".cache", "zellij")
}

// EnsurePluginPermissions adds plugin permissions to Zellij's permissions.kdl
// cache. Background plugins loaded via load_plugins cannot show permission
// dialogs, so permissions must be pre-populated before the plugin loads.
// With the single-binary architecture, both controller (background) and sidebar
// (visible) share the same WASM URL. The sidebar would eventually show a dialog,
// but the controller may load first and enter a blocking state waiting for
// permissions that can never be granted interactively.
func EnsurePluginPermissions(cacheDir, pluginsDir string) error {
	permPath := filepath.Join(cacheDir, "permissions.kdl")
	pluginPath := filepath.Join(pluginsDir, "cc_deck.wasm")

	content, err := os.ReadFile(permPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	entry := fmt.Sprintf(`"%s" {
    MessageAndLaunchOtherPlugins
    ChangeApplicationState
    Reconfigure
    RunCommands
    ReadCliPipes
    ReadApplicationState
    WriteToStdin
}
`, pluginPath)

	if strings.Contains(string(content), pluginPath) {
		// Replace stale entry with current permissions
		lines := strings.Split(string(content), "\n")
		var filtered []string
		skip := false
		for _, line := range lines {
			if strings.Contains(line, pluginPath) {
				skip = true
				continue
			}
			if skip && strings.TrimSpace(line) == "}" {
				skip = false
				continue
			}
			if skip {
				continue
			}
			filtered = append(filtered, line)
		}
		content = []byte(strings.Join(filtered, "\n"))
	}

	if !strings.HasSuffix(string(content), "\n") && len(content) > 0 {
		content = append(content, '\n')
	}
	content = append(content, []byte(entry)...)
	return os.WriteFile(permPath, content, 0644)
}

// CheckCompatibility returns "compatible", "untested", or "incompatible"
// based on the Zellij version and the plugin SDK version.
//
// Rules:
//   - If major.minor < 0.40: "incompatible"
//   - If major.minor >= 0.40 and <= 0.43: "compatible"
//   - If major.minor > 0.43: "untested"
//   - If version cannot be parsed: "untested"
func CheckCompatibility(zellijVersion, sdkVersion string) string {
	major, minor, ok := parseVersion(zellijVersion)
	if !ok {
		return "untested"
	}

	sdkMajor, sdkMinor, sdkOk := parseVersion(sdkVersion)
	if !sdkOk {
		return "untested"
	}

	// Compare against minimum (0.40)
	if major == 0 && minor < 40 {
		return "incompatible"
	}

	// Compare against SDK version upper bound
	if major < sdkMajor || (major == sdkMajor && minor <= sdkMinor) {
		return "compatible"
	}

	return "untested"
}

// parseVersion extracts major and minor from a version string like "0.43.1".
func parseVersion(v string) (major, minor int, ok bool) {
	v = strings.TrimSpace(v)
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 2 {
		return 0, 0, false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, false
	}
	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, false
	}
	return major, minor, true
}

// PluginLocation returns the expected file:// location string for the plugin,
// using the given plugins directory path.
func PluginLocation(pluginsDir string) string {
	return fmt.Sprintf("file:%s", filepath.Join(pluginsDir, "cc_deck.wasm"))
}
