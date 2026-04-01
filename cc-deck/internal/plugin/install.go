package plugin

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// InstallOptions configures the install behavior.
type InstallOptions struct {
	Force         bool
	SkipBackup    bool
	Layout        string // layout name (currently only "cc-deck")
	InstallZellij bool   // download and install matching Zellij binary
	Stdout        io.Writer
	Stderr        io.Writer
	Stdin         io.Reader // for confirmation prompt
}

// Install copies the embedded WASM binary, writes a layout file, and registers
// hooks in ~/.claude/settings.json with a timestamped backup.
func Install(opts InstallOptions) error {
	// 0. Install Zellij if requested (before detection)
	if opts.InstallZellij {
		if err := installZellijBinary(opts); err != nil {
			return fmt.Errorf("installing Zellij: %w", err)
		}
	}

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
	controllerPath := filepath.Join(zInfo.PluginsDir, "cc_deck_controller.wasm")
	sidebarPath := filepath.Join(zInfo.PluginsDir, "cc_deck_sidebar.wasm")
	legacyPath := filepath.Join(zInfo.PluginsDir, "cc_deck.wasm")
	if !opts.Force {
		if _, err := os.Stat(controllerPath); err == nil {
			fmt.Fprintf(opts.Stdout, "Plugin already installed at %s\n", controllerPath)
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

	// 4. Atomic write both WASM binaries
	if err := atomicWrite(controllerPath, pInfo.ControllerBinary, 0o644); err != nil {
		return wrapPermissionError(err, controllerPath, "write")
	}
	if err := atomicWrite(sidebarPath, pInfo.SidebarBinary, 0o644); err != nil {
		// Rollback controller to avoid version mismatch
		_ = os.Remove(controllerPath)
		return wrapPermissionError(err, sidebarPath, "write")
	}

	// Remove legacy single binary if present (migration from old architecture)
	_ = os.Remove(legacyPath)

	// 4b. Pre-populate controller permissions (background plugins can't show dialogs)
	if err := EnsureControllerPermissions(zInfo.CacheDir, zInfo.PluginsDir); err != nil {
		fmt.Fprintf(opts.Stderr, "Warning: Could not set controller permissions: %v\n", err)
	}

	// 4c. Ensure controller is registered in config.kdl load_plugins
	configPath := filepath.Join(zInfo.ConfigDir, "config.kdl")
	if err := ensureControllerInConfig(configPath, zInfo.PluginsDir); err != nil {
		fmt.Fprintf(opts.Stderr, "Warning: Could not update config.kdl: %v\n", err)
	}

	// 5. Write all layout variants (with diff check for existing files)
	defaultVariant := LayoutVariant(opts.Layout)
	if defaultVariant == "" {
		defaultVariant = LayoutStandard
	}
	variants := []LayoutVariant{LayoutMinimal, LayoutStandard, LayoutClean}
	for _, v := range variants {
		filename := LayoutFilename(v)
		path := filepath.Join(zInfo.LayoutsDir, filename)
		content := GenerateLayout(zInfo.PluginsDir, v)
		if err := writeLayoutWithDiff(path, []byte(content), opts); err != nil {
			return err
		}
	}
	// Write the default as cc-deck.kdl
	defaultLayoutPath := filepath.Join(zInfo.LayoutsDir, "cc-deck.kdl")
	defaultContent := GenerateLayout(zInfo.PluginsDir, defaultVariant)
	if err := writeLayoutWithDiff(defaultLayoutPath, []byte(defaultContent), opts); err != nil {
		return err
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
	fmt.Fprintf(opts.Stdout, "  Controller: %s\n", tildeHome(controllerPath))
	fmt.Fprintf(opts.Stdout, "  Sidebar:    %s (%.1f MB total)\n", tildeHome(sidebarPath), sizeMB)
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

// writeLayoutWithDiff writes a layout file, checking for differences with existing content.
// If the file exists and differs, shows a colored diff and prompts for action.
func writeLayoutWithDiff(path string, newContent []byte, opts InstallOptions) error {
	existing, err := os.ReadFile(path)
	if err != nil {
		// File doesn't exist, write directly
		return atomicWrite(path, newContent, 0o644)
	}

	// File exists, check if identical
	if string(existing) == string(newContent) {
		return nil // No changes needed
	}

	// Force mode: overwrite without prompting
	if opts.Force {
		return atomicWrite(path, newContent, 0o644)
	}

	// Show diff and prompt
	fmt.Fprintf(opts.Stdout, "\n\x1b[33mLayout %s has local changes:\x1b[0m\n", tildeHome(path))
	showColorDiff(string(existing), string(newContent), opts.Stdout)
	fmt.Fprintln(opts.Stdout)
	fmt.Fprint(opts.Stdout, "  [o]verwrite  [s]kip  [b]ackup+overwrite  ? ")

	reader := bufio.NewReader(opts.Stdin)
	answer, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}
	answer = strings.TrimSpace(strings.ToLower(answer))

	switch answer {
	case "o", "overwrite":
		return atomicWrite(path, newContent, 0o644)
	case "b", "backup":
		backupPath := path + ".bak"
		if err := os.WriteFile(backupPath, existing, 0o644); err != nil {
			return fmt.Errorf("creating backup: %w", err)
		}
		fmt.Fprintf(opts.Stdout, "  Backed up to %s\n", tildeHome(backupPath))
		return atomicWrite(path, newContent, 0o644)
	default:
		fmt.Fprintf(opts.Stdout, "  Skipped %s\n", tildeHome(path))
		return nil
	}
}

// showColorDiff prints a simple colored unified diff between old and new content.
func showColorDiff(old, new string, w io.Writer) {
	oldLines := strings.Split(old, "\n")
	newLines := strings.Split(new, "\n")

	// Simple line-by-line comparison (not a full diff algorithm, but sufficient for small layouts)
	maxLines := len(oldLines)
	if len(newLines) > maxLines {
		maxLines = len(newLines)
	}

	for i := 0; i < maxLines; i++ {
		oldLine := ""
		newLine := ""
		if i < len(oldLines) {
			oldLine = oldLines[i]
		}
		if i < len(newLines) {
			newLine = newLines[i]
		}

		if oldLine == newLine {
			continue // Skip identical lines
		}
		if i < len(oldLines) && oldLine != "" {
			fmt.Fprintf(w, "  \x1b[31m- %s\x1b[0m\n", oldLine)
		}
		if i < len(newLines) && newLine != "" {
			fmt.Fprintf(w, "  \x1b[32m+ %s\x1b[0m\n", newLine)
		}
	}
}

// wrapPermissionError adds actionable guidance when a file operation fails.
func wrapPermissionError(err error, path string, access string) error {
	if os.IsPermission(err) {
		return fmt.Errorf("permission denied at %s: check directory permissions (need %s access): %w", path, access, err)
	}
	return fmt.Errorf("file operation failed at %s: %w", path, err)
}

// installZellijBinary downloads and installs the Zellij binary matching the plugin SDK version.
func installZellijBinary(opts InstallOptions) error {
	pInfo := EmbeddedPlugin()
	// Use the SDK version as the target Zellij version (e.g., "0.43")
	// Append .0 for the release tag if only major.minor
	version := pInfo.SDKVersion
	parts := strings.SplitN(version, ".", 3)
	if len(parts) == 2 {
		version = version + ".0"
	}

	// Map Go arch to Zellij release arch
	arch := runtime.GOARCH
	switch arch {
	case "amd64":
		arch = "x86_64"
	case "arm64":
		arch = "aarch64"
	}

	goos := runtime.GOOS
	switch goos {
	case "darwin":
		goos = "apple-darwin"
	case "linux":
		goos = "unknown-linux-musl"
	default:
		return fmt.Errorf("unsupported OS: %s", goos)
	}

	url := fmt.Sprintf("https://github.com/zellij-org/zellij/releases/download/v%s/zellij-%s-%s.tar.gz", version, arch, goos)
	fmt.Fprintf(opts.Stdout, "Downloading Zellij v%s from %s\n", version, url)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("downloading Zellij: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d from %s", resp.StatusCode, url)
	}

	// Write to temp file
	tmpFile, err := os.CreateTemp("", "zellij-*.tar.gz")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		return err
	}
	tmpFile.Close()

	// Extract with tar
	installDir := "/usr/local/bin"
	if _, err := os.Stat(installDir); os.IsNotExist(err) {
		if err := os.MkdirAll(installDir, 0o755); err != nil {
			return fmt.Errorf("creating install directory: %w", err)
		}
	}

	tarCmd := exec.Command("tar", "xzf", tmpPath, "-C", installDir, "zellij")
	tarCmd.Stdout = opts.Stdout
	tarCmd.Stderr = opts.Stderr
	if err := tarCmd.Run(); err != nil {
		return fmt.Errorf("extracting Zellij: %w", err)
	}

	fmt.Fprintf(opts.Stdout, "Zellij v%s installed to %s/zellij\n", version, installDir)
	return nil
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
