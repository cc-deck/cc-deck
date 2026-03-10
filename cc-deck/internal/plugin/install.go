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
	Force      bool
	SkipBackup bool
	Layout     string // layout name (currently only "cc-deck")
	Stdout     io.Writer
	Stderr     io.Writer
	Stdin      io.Reader // for confirmation prompt
}

// Install copies the embedded WASM binary, writes a layout file, and registers
// hooks in ~/.claude/settings.json with a timestamped backup.
func Install(opts InstallOptions) error {
	// 1. Detect Zellij and warn if missing
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

	// 4. Atomic write WASM binary
	if err := atomicWrite(pluginPath, pInfo.Binary, 0o644); err != nil {
		return wrapPermissionError(err, pluginPath, "write")
	}

	// 5. Write all layout variants
	defaultVariant := LayoutVariant(opts.Layout)
	if defaultVariant == "" {
		defaultVariant = LayoutStandard
	}
	variants := []LayoutVariant{LayoutMinimal, LayoutStandard, LayoutClean}
	for _, v := range variants {
		filename := LayoutFilename(v)
		path := filepath.Join(zInfo.LayoutsDir, filename)
		content := GenerateLayout(zInfo.PluginsDir, v)
		if err := atomicWrite(path, []byte(content), 0o644); err != nil {
			return wrapPermissionError(err, path, "write")
		}
	}
	// Write the default as cc-deck.kdl (symlink or copy)
	defaultLayoutPath := filepath.Join(zInfo.LayoutsDir, "cc-deck.kdl")
	defaultContent := GenerateLayout(zInfo.PluginsDir, defaultVariant)
	if err := atomicWrite(defaultLayoutPath, []byte(defaultContent), 0o644); err != nil {
		return wrapPermissionError(err, defaultLayoutPath, "write")
	}
	layoutPath := defaultLayoutPath

	// 6. Register hooks in settings.json (with backup)
	settingsPath := ClaudeSettingsPath()
	backupPath, err := BackupFile(settingsPath, opts.SkipBackup)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "Warning: Could not backup settings.json: %v\n", err)
	}
	if err := RegisterHooks(settingsPath); err != nil {
		return fmt.Errorf("registering hooks: %w", err)
	}

	// 7. Print summary
	fmt.Fprintln(opts.Stdout, "cc-deck installed successfully.")
	fmt.Fprintln(opts.Stdout)
	sizeMB := float64(pInfo.BinarySize) / (1024 * 1024)
	fmt.Fprintf(opts.Stdout, "  Plugin:   %s (%.1f MB)\n", tildeHome(pluginPath), sizeMB)
	fmt.Fprintf(opts.Stdout, "  Layout:   %s (%s)\n", tildeHome(layoutPath), defaultVariant)
	hookCount := HookEventCount(settingsPath)
	fmt.Fprintf(opts.Stdout, "  Hooks:    registered (%d event types)\n", hookCount)
	if backupPath != "" {
		fmt.Fprintf(opts.Stdout, "  Backup:   %s\n", tildeHome(backupPath))
	}
	fmt.Fprintln(opts.Stdout)
	fmt.Fprintln(opts.Stdout, "Start Zellij with the cc-deck layout:")
	fmt.Fprintln(opts.Stdout)
	fmt.Fprintln(opts.Stdout, "  zellij --layout cc-deck")
	fmt.Fprintln(opts.Stdout)
	fmt.Fprintln(opts.Stdout, "Other layout variants:")
	fmt.Fprintln(opts.Stdout, "  zellij --layout cc-deck-minimal   (compact-bar only)")
	fmt.Fprintln(opts.Stdout, "  zellij --layout cc-deck-clean     (no bars)")
	fmt.Fprintln(opts.Stdout)
	fmt.Fprintln(opts.Stdout, "To make cc-deck the default, add to ~/.config/zellij/config.kdl:")
	fmt.Fprintln(opts.Stdout, "  default_layout \"cc-deck\"")
	fmt.Fprintln(opts.Stdout)

	return nil
}

// wrapPermissionError adds actionable guidance when a file operation fails.
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
