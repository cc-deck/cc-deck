package plugin

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// InstallOptions configures the install behavior.
type InstallOptions struct {
	Force         bool
	Layout        string // "minimal" or "full"
	InjectDefault bool
	Stdout        io.Writer
	Stderr        io.Writer
	Stdin         io.Reader // for confirmation prompt
}

// Install copies the embedded WASM binary and a layout file into the Zellij
// config directories. It optionally injects a plugin pane into the default
// layout when InjectDefault is set.
func Install(opts InstallOptions) error {
	// 1. Detect Zellij and warn if missing or incompatible
	zInfo := DetectZellij()
	pInfo := EmbeddedPlugin()

	if !zInfo.Installed {
		fmt.Fprintln(opts.Stderr, "Warning: Zellij not found on PATH. Install Zellij first.")
	} else {
		compat := CheckCompatibility(zInfo.Version, pInfo.SDKVersion)
		if compat == "incompatible" {
			fmt.Fprintf(opts.Stderr, "Warning: Zellij version %s may be incompatible (requires %s+).\n", zInfo.Version, pInfo.MinZellij)
		}
	}

	// 2. Check if already installed (prompt if not --force)
	pluginPath := filepath.Join(zInfo.PluginsDir, "cc_deck.wasm")
	if !opts.Force {
		if _, err := os.Stat(pluginPath); err == nil {
			fmt.Fprintf(opts.Stdout, "Plugin already installed at %s\n", pluginPath)
			fmt.Fprint(opts.Stdout, "Overwrite? [y/N] ")
			reader := bufio.NewReader(opts.Stdin)
			answer, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("reading confirmation: %w", err)
			}
			answer = strings.TrimSpace(strings.ToLower(answer))
			if answer != "y" && answer != "yes" {
				return fmt.Errorf("cancelled by user")
			}
		}
	}

	// 3. Create directories
	if err := os.MkdirAll(zInfo.PluginsDir, 0o755); err != nil {
		return wrapPermissionError(err, zInfo.PluginsDir, "write")
	}
	if err := os.MkdirAll(zInfo.LayoutsDir, 0o755); err != nil {
		return wrapPermissionError(err, zInfo.LayoutsDir, "write")
	}

	// 4. Atomic write WASM binary (temp file + rename)
	if err := atomicWrite(pluginPath, pInfo.Binary, 0o644); err != nil {
		return wrapPermissionError(err, pluginPath, "write")
	}

	// 5. Write layout file
	layoutPath := filepath.Join(zInfo.LayoutsDir, "cc-deck.kdl")
	var layoutContent string
	switch opts.Layout {
	case "full":
		layoutContent = FullLayout(zInfo.PluginsDir)
	default:
		layoutContent = MinimalLayout(zInfo.PluginsDir)
	}
	if err := atomicWrite(layoutPath, []byte(layoutContent), 0o644); err != nil {
		return wrapPermissionError(err, layoutPath, "write")
	}

	// 6. If --inject-default: call InjectDefault
	if opts.InjectDefault {
		if err := InjectDefault(zInfo, opts.Stderr); err != nil {
			return err
		}
	}

	// 7. Print summary
	printInstallSummary(opts, zInfo, pInfo, pluginPath, layoutPath)

	return nil
}

// InjectDefault locates default.kdl in the Zellij layouts directory, reads its
// content, checks for existing injection, and appends the cc-deck plugin block.
func InjectDefault(zInfo ZellijInfo, stderr io.Writer) error {
	defaultPath := filepath.Join(zInfo.LayoutsDir, "default.kdl")

	content, err := os.ReadFile(defaultPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(stderr, "Warning: No default layout found at "+defaultPath)
			fmt.Fprintln(stderr, "  Use the dedicated layout instead: zellij --layout cc-deck")
			return nil
		}
		fmt.Fprintf(stderr, "Warning: Could not read default layout: %v\n", err)
		fmt.Fprintln(stderr, "  Skipping injection. Use the dedicated layout instead: zellij --layout cc-deck")
		return nil
	}

	if HasInjection(string(content)) {
		// Already injected, nothing to do
		return nil
	}

	injected := InjectPlugin(string(content), zInfo.PluginsDir)
	if err := atomicWrite(defaultPath, []byte(injected), 0o644); err != nil {
		fmt.Fprintf(stderr, "Warning: Could not write default layout: %v\n", err)
		fmt.Fprintln(stderr, "  Skipping injection. Use the dedicated layout instead: zellij --layout cc-deck")
		return nil
	}

	return nil
}

// printInstallSummary writes the installation summary to stdout.
func printInstallSummary(opts InstallOptions, zInfo ZellijInfo, pInfo PluginInfo, pluginPath, layoutPath string) {
	sizeMB := float64(pInfo.BinarySize) / (1024 * 1024)
	fmt.Fprintln(opts.Stdout, "Plugin installed successfully.")
	fmt.Fprintln(opts.Stdout)
	fmt.Fprintf(opts.Stdout, "  Binary:  %s (%.1f MB)\n", pluginPath, sizeMB)
	fmt.Fprintf(opts.Stdout, "  Layout:  %s (%s)\n", layoutPath, opts.Layout)
	if opts.InjectDefault {
		defaultPath := filepath.Join(zInfo.LayoutsDir, "default.kdl")
		fmt.Fprintf(opts.Stdout, "  Default layout injected: %s\n", defaultPath)
	}
	fmt.Fprintln(opts.Stdout)
	fmt.Fprintln(opts.Stdout, "To start Zellij with the plugin:")
	fmt.Fprintln(opts.Stdout)
	fmt.Fprintln(opts.Stdout, "  zellij --layout cc-deck")
}

// wrapPermissionError adds actionable guidance when a file operation fails due
// to insufficient permissions.
func wrapPermissionError(err error, path string, access string) error {
	if os.IsPermission(err) {
		return fmt.Errorf("permission denied at %s: check directory permissions (need %s access): %w", path, access, err)
	}
	return fmt.Errorf("file operation failed at %s: %w", path, err)
}

// atomicWrite writes data to a temporary file and renames it to the target path.
func atomicWrite(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".cc-deck-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}
