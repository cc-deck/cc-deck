package plugin

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// RemoveOptions configures the remove command.
type RemoveOptions struct {
	SkipBackup bool
	Stdout     io.Writer
	Stderr     io.Writer
}

// Remove deletes the cc-deck plugin binary, layout file, and hooks from
// settings.json. It creates a backup of settings.json before modification.
func Remove(opts RemoveOptions) error {
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}

	zInfo := DetectZellij()
	pInfo := EmbeddedPlugin()
	state := DetectInstallState(zInfo, pInfo)

	settingsPath := ClaudeSettingsPath()
	hasHooks := HasHooks(settingsPath)

	if !state.PluginInstalled && !state.LayoutInstalled && !state.DefaultInjected && !hasHooks {
		fmt.Fprintln(opts.Stdout, "Nothing to remove. cc-deck is not installed.")
		return nil
	}

	fmt.Fprintln(opts.Stdout, "cc-deck removed.")
	fmt.Fprintln(opts.Stdout)

	if state.PluginInstalled {
		if err := os.Remove(state.PluginPath); err != nil {
			return wrapPermissionError(err, state.PluginPath, "write")
		}
		fmt.Fprintf(opts.Stdout, "  Removed: %s\n", tildeHome(state.PluginPath))
	}

	cleanupPluginSymlinks(zInfo.PluginsDir, opts.Stdout)

	if state.LayoutInstalled {
		if err := os.Remove(state.LayoutPath); err != nil {
			return wrapPermissionError(err, state.LayoutPath, "write")
		}
		fmt.Fprintf(opts.Stdout, "  Removed: %s\n", tildeHome(state.LayoutPath))
	}

	if state.DefaultInjected {
		content, err := os.ReadFile(state.DefaultLayoutPath)
		if err != nil {
			return wrapPermissionError(err, state.DefaultLayoutPath, "read")
		}
		cleaned := RemoveInjection(string(content))
		if err := os.WriteFile(state.DefaultLayoutPath, []byte(cleaned), 0644); err != nil {
			return wrapPermissionError(err, state.DefaultLayoutPath, "write")
		}
		fmt.Fprintf(opts.Stdout, "  Reverted: %s (plugin pane removed)\n", tildeHome(state.DefaultLayoutPath))
	}

	// Remove hooks from settings.json (with backup)
	if hasHooks {
		backupPath, err := BackupFile(settingsPath, opts.SkipBackup)
		if err != nil {
			fmt.Fprintf(opts.Stderr, "Warning: Could not backup settings.json: %v\n", err)
		}
		if err := RemoveHooks(settingsPath); err != nil {
			return fmt.Errorf("removing hooks: %w", err)
		}
		fmt.Fprintf(opts.Stdout, "  Hooks:   removed from %s\n", tildeHome(settingsPath))
		if backupPath != "" {
			fmt.Fprintf(opts.Stdout, "  Backup:  %s\n", tildeHome(backupPath))
		}
	}

	if isZellijRunning() {
		fmt.Fprintln(opts.Stdout)
		fmt.Fprintln(opts.Stdout, "  Note: Zellij may be running. Restart sessions to fully unload the plugin.")
	}

	return nil
}

// RunRemove is the command runner function for the remove command.
func RunRemove(stdout, stderr io.Writer) error {
	return Remove(RemoveOptions{
		Stdout: stdout,
		Stderr: stderr,
	})
}

// cleanupPluginSymlinks removes any symlinks in the plugins directory that
// point to cc_deck.wasm.
func cleanupPluginSymlinks(pluginsDir string, stdout io.Writer) {
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		path := filepath.Join(pluginsDir, e.Name())
		fi, err := os.Lstat(path)
		if err != nil || fi.Mode()&os.ModeSymlink == 0 {
			continue
		}
		target, err := os.Readlink(path)
		if err != nil {
			continue
		}
		if target == "cc_deck.wasm" || filepath.Base(target) == "cc_deck.wasm" {
			os.Remove(path)
			fmt.Fprintf(stdout, "  Removed: %s (symlink)\n", tildeHome(path))
		}
	}
}

// isZellijRunning checks if any Zellij process is currently running.
func isZellijRunning() bool {
	return exec.Command("pgrep", "-x", "zellij").Run() == nil
}
