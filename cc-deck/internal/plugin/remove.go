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
	Stdout io.Writer
	Stderr io.Writer
}

// Remove deletes the cc-deck plugin binary, layout file, and any injection
// from the default layout. It prints a summary of actions taken.
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

	if !state.PluginInstalled && !state.LayoutInstalled && !state.DefaultInjected {
		fmt.Fprintln(opts.Stdout, "Nothing to remove. Plugin is not installed.")
		return nil
	}

	fmt.Fprintln(opts.Stdout, "Plugin removed.")
	fmt.Fprintln(opts.Stdout)

	if state.PluginInstalled {
		if err := os.Remove(state.PluginPath); err != nil {
			return wrapPermissionError(err, state.PluginPath, "write")
		}
		fmt.Fprintf(opts.Stdout, "  Removed: %s\n", tildeHome(state.PluginPath))
	}

	// Clean up any symlinks pointing to the plugin binary (e.g. cc-deck.wasm -> cc_deck.wasm)
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

	if isZellijRunning() {
		fmt.Fprintf(opts.Stdout, "  Warning: Zellij may be running. Restart Zellij sessions to fully unload the plugin.\n")
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
// point to cc_deck.wasm (e.g. cc-deck.wasm -> cc_deck.wasm from manual setup).
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
