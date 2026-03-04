package plugin

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// StatusReport holds the complete status of the cc-deck plugin installation.
type StatusReport struct {
	Plugin  PluginStatus `json:"plugin" yaml:"plugin"`
	Zellij  ZellijStatus `json:"zellij" yaml:"zellij"`
	Layouts LayoutStatus `json:"layouts" yaml:"layouts"`
}

// PluginStatus describes the installed plugin state.
type PluginStatus struct {
	Installed bool   `json:"installed" yaml:"installed"`
	Path      string `json:"path,omitempty" yaml:"path,omitempty"`
	Size      int64  `json:"size,omitempty" yaml:"size,omitempty"`
	Version   string `json:"version,omitempty" yaml:"version,omitempty"`
}

// ZellijStatus describes the Zellij installation state.
type ZellijStatus struct {
	Installed     bool   `json:"installed" yaml:"installed"`
	Version       string `json:"version,omitempty" yaml:"version,omitempty"`
	Compatibility string `json:"compatibility,omitempty" yaml:"compatibility,omitempty"`
	ConfigDir     string `json:"configDir,omitempty" yaml:"configDir,omitempty"`
}

// LayoutStatus describes the layout installation state.
type LayoutStatus struct {
	CcDeckLayout    string `json:"ccDeckLayout,omitempty" yaml:"ccDeckLayout,omitempty"`
	DefaultInjected bool   `json:"defaultInjected" yaml:"defaultInjected"`
}

// Status gathers all installation information and returns a StatusReport.
func Status() StatusReport {
	zInfo := DetectZellij()
	pInfo := EmbeddedPlugin()
	state := DetectInstallState(zInfo, pInfo)

	report := StatusReport{}

	report.Plugin.Installed = state.PluginInstalled
	if state.PluginInstalled {
		report.Plugin.Path = state.PluginPath
		report.Plugin.Size = state.PluginSize
		report.Plugin.Version = pInfo.Version
	} else {
		// Show expected path even when not installed
		report.Plugin.Path = expectedPluginPath(zInfo)
	}

	report.Zellij.Installed = zInfo.Installed
	if zInfo.Installed {
		report.Zellij.Version = zInfo.Version
		report.Zellij.Compatibility = state.Compatibility
		report.Zellij.ConfigDir = zInfo.ConfigDir
	}

	report.Layouts.CcDeckLayout = state.LayoutType
	report.Layouts.DefaultInjected = state.DefaultInjected

	return report
}

// expectedPluginPath returns the path where the plugin would be installed.
func expectedPluginPath(zInfo ZellijInfo) string {
	if zInfo.Installed {
		return fmt.Sprintf("%s/cc_deck.wasm", zInfo.PluginsDir)
	}
	return ""
}

// FormatText writes the status report in human-readable text format.
func (r StatusReport) FormatText(w io.Writer) error {
	var b strings.Builder

	b.WriteString("Plugin Status\n")
	b.WriteString(fmt.Sprintf("  Installed:      %s\n", yesNo(r.Plugin.Installed)))
	if r.Plugin.Installed {
		b.WriteString(fmt.Sprintf("  Path:           %s\n", tildeHome(r.Plugin.Path)))
		b.WriteString(fmt.Sprintf("  Size:           %s\n", humanSize(r.Plugin.Size)))
		b.WriteString(fmt.Sprintf("  Version:        %s\n", r.Plugin.Version))
	} else if r.Plugin.Path != "" {
		b.WriteString(fmt.Sprintf("  Expected path:  %s\n", tildeHome(r.Plugin.Path)))
	}

	b.WriteString("\nZellij\n")
	if r.Zellij.Installed {
		b.WriteString(fmt.Sprintf("  Installed:      yes (%s)\n", r.Zellij.Version))
		b.WriteString(fmt.Sprintf("  Compatibility:  %s\n", r.Zellij.Compatibility))
		b.WriteString(fmt.Sprintf("  Config dir:     %s\n", tildeHome(r.Zellij.ConfigDir)))
	} else {
		b.WriteString("  Installed:      no\n")
	}

	b.WriteString("\nLayouts\n")
	if r.Layouts.CcDeckLayout != "" {
		b.WriteString(fmt.Sprintf("  cc-deck.kdl:    installed (%s)\n", r.Layouts.CcDeckLayout))
	} else {
		b.WriteString("  cc-deck.kdl:    not installed\n")
	}
	injectedStr := "not injected"
	if r.Layouts.DefaultInjected {
		injectedStr = "injected"
	}
	b.WriteString(fmt.Sprintf("  Default layout: %s\n", injectedStr))

	_, err := io.WriteString(w, b.String())
	return err
}

// FormatJSON writes the status report as JSON.
func (r StatusReport) FormatJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// FormatYAML writes the status report as YAML.
func (r StatusReport) FormatYAML(w io.Writer) error {
	enc := yaml.NewEncoder(w)
	defer enc.Close()
	return enc.Encode(r)
}

// RunStatus is the command runner function for the status command.
// It gathers status and writes output in the requested format to w.
// If Zellij is not found, a warning is printed to stderr.
func RunStatus(w io.Writer, stderr io.Writer, outputFormat string) error {
	report := Status()

	if !report.Zellij.Installed {
		fmt.Fprintln(stderr, "Warning: Zellij not found on PATH. Install Zellij first.")
	}

	switch strings.ToLower(outputFormat) {
	case "json":
		return report.FormatJSON(w)
	case "yaml":
		return report.FormatYAML(w)
	default:
		return report.FormatText(w)
	}
}

// yesNo returns "yes" or "no" for a boolean value.
func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// humanSize formats a byte count into a human-readable string.
func humanSize(bytes int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
	)
	switch {
	case bytes >= mb:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(mb))
	case bytes >= kb:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(kb))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// tildeHome replaces the user's home directory prefix with ~ for display.
func tildeHome(path string) string {
	home, err := homeDir()
	if err != nil || home == "" {
		return path
	}
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}

// homeDir returns the current user's home directory.
func homeDir() (string, error) {
	return homeDirFunc()
}

var homeDirFunc = os.UserHomeDir
