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
