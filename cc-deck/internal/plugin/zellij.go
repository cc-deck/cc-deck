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
		// Output is typically "zellij 0.44.3\n"
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

// RequiredPermissions is every Zellij permission the plugin asks for, in
// either role. It must match REQUIRED_PERMISSIONS in the plugin's lib.rs;
// a test compares the two.
var RequiredPermissions = []string{
	"ReadApplicationState",
	"ChangeApplicationState",
	"RunCommands",
	"ReadCliPipes",
	"MessageAndLaunchOtherPlugins",
	"Reconfigure",
	"WriteToStdin",
}

// EnsurePluginPermissions seeds Zellij's permissions.kdl cache with the
// plugin's grant and reports whether the file had to be written.
//
// The controller is a background plugin from `load_plugins`. Zellij cannot
// show it a permission dialog (zellij-org/zellij#4982), and a grant given to
// the sidebar's dialog does not reach a controller that is already waiting
// (zellij-org/zellij#4990 is still open). A cached grant is the only thing
// that lets the controller start on the first run, so the cache is seeded
// before Zellij ever loads the plugin and checked again before every launch.
//
// The write is idempotent: an entry that already carries exactly
// RequiredPermissions is left alone, so a preflight on a healthy machine
// touches nothing.
func EnsurePluginPermissions(cacheDir, pluginsDir string) (bool, error) {
	permPath := filepath.Join(cacheDir, "permissions.kdl")
	pluginPath := filepath.Join(pluginsDir, "cc_deck.wasm")

	content, err := os.ReadFile(permPath)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}

	kept, existing := splitPermissionEntry(string(content), pluginPath)
	if existing != nil && sameStringSet(existing, RequiredPermissions) {
		return false, nil
	}

	var b strings.Builder
	b.WriteString(kept)
	if kept != "" && !strings.HasSuffix(kept, "\n") {
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "%q {\n", pluginPath)
	for _, p := range RequiredPermissions {
		fmt.Fprintf(&b, "    %s\n", p)
	}
	b.WriteString("}\n")

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return false, err
	}
	if err := os.WriteFile(permPath, []byte(b.String()), 0644); err != nil {
		return false, err
	}
	return true, nil
}

// PreflightPluginPermissions runs EnsurePluginPermissions against the
// detected Zellij directories. It is meant for the moment just before a
// Zellij session is created, when a wiped cache would otherwise strand the
// controller until the next restart.
func PreflightPluginPermissions() (bool, error) {
	configDir := resolveZellijConfigDir()
	return EnsurePluginPermissions(resolveZellijCacheDir(), filepath.Join(configDir, "plugins"))
}

// splitPermissionEntry removes the block for pluginPath from a
// permissions.kdl body. It returns the remaining content and the permission
// names found in the removed block, or nil when there was no block.
func splitPermissionEntry(content, pluginPath string) (string, []string) {
	if !strings.Contains(content, pluginPath) {
		return content, nil
	}
	var kept []string
	var found []string
	inBlock := false
	for _, line := range strings.Split(content, "\n") {
		switch {
		case !inBlock && strings.Contains(line, pluginPath):
			inBlock = true
			found = []string{}
		case inBlock && strings.TrimSpace(line) == "}":
			inBlock = false
		case inBlock:
			if name := strings.TrimSpace(line); name != "" {
				found = append(found, name)
			}
		default:
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n"), found
}

func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	set := make(map[string]bool, len(a))
	for _, s := range a {
		set[s] = true
	}
	for _, s := range b {
		if !set[s] {
			return false
		}
	}
	return true
}

// MinZellijVersion is the oldest Zellij release cc-deck supports.
//
// 0.45.0 is the first release that loads a `load_plugins` background plugin
// exactly once (zellij-org/zellij#5178). Older releases could create two
// controller instances on startup, and cc-deck no longer carries the leader
// election that used to paper over that.
const MinZellijVersion = "0.45"

// MaxTestedZellijVersion is the newest Zellij minor the plugin has been
// verified against. Newer releases are reported as "untested", not refused.
const MaxTestedZellijVersion = "0.45"

// CheckCompatibility returns "compatible", "untested", or "incompatible"
// based on the Zellij version and the newest tested Zellij version.
//
// Rules:
//   - If major.minor < MinZellijVersion: "incompatible"
//   - If major.minor >= MinZellijVersion and <= maxTested: "compatible"
//   - If major.minor > maxTested: "untested"
//   - If version cannot be parsed: "untested"
func CheckCompatibility(zellijVersion, maxTested string) string {
	major, minor, ok := parseVersion(zellijVersion)
	if !ok {
		return "untested"
	}

	sdkMajor, sdkMinor, sdkOk := parseVersion(maxTested)
	if !sdkOk {
		return "untested"
	}

	minMajor, minMinor, _ := parseVersion(MinZellijVersion)
	if major < minMajor || (major == minMajor && minor < minMinor) {
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
