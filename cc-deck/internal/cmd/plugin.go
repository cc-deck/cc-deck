package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/rhuss/cc-mux/cc-deck/internal/plugin"
)

// NewPluginCmd creates the plugin cobra command with subcommands.
func NewPluginCmd(gf *GlobalFlags) *cobra.Command {
	pluginCmd := &cobra.Command{
		Use:   "plugin",
		Short: "Manage the Zellij plugin",
		Long:  "Install, inspect, and remove the cc-deck Zellij plugin.",
	}

	pluginCmd.AddCommand(
		newPluginInstallCmd(gf),
		newPluginStatusCmd(gf),
		newPluginRemoveCmd(gf),
	)

	return pluginCmd
}

type pluginInstallFlags struct {
	force      bool
	skipBackup bool
	layout     string
}

func newPluginInstallCmd(_ *GlobalFlags) *cobra.Command {
	f := &pluginInstallFlags{}

	installCmd := &cobra.Command{
		Use:   "install",
		Short: "Install the Zellij plugin, layout, and hooks",
		Long: `Install the embedded cc-deck WASM plugin into the Zellij plugins directory,
write sidebar layouts to the Zellij layouts directory, and register Claude Code
hooks in ~/.claude/settings.json.

Layout variants:
  minimal   Sidebar + compact-bar at bottom (default)
  standard  Sidebar + tab-bar at top + status-bar at bottom (beginner-friendly)
  clean     Sidebar only, no bars (maximum terminal space)

All variants are written as layout files. The --layout flag sets the default
(cc-deck.kdl). Use "zellij --layout cc-deck-standard" to try other variants.

A timestamped backup of settings.json is created before modification
unless --skip-backup is specified.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runPluginInstall(f)
		},
	}

	installCmd.Flags().BoolVarP(&f.force, "force", "f", false, "Overwrite without prompting")
	installCmd.Flags().BoolVar(&f.skipBackup, "skip-backup", false, "Skip creating backup of settings.json")
	installCmd.Flags().StringVar(&f.layout, "layout", "standard", "Default layout variant (standard, minimal, clean)")

	return installCmd
}

func newPluginStatusCmd(gf *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show plugin installation status",
		Long:  "Display the current installation status of the cc-deck Zellij plugin, layout, hooks, and Zellij itself.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return plugin.RunStatus(os.Stdout, os.Stderr, gf.Output)
		},
	}
}

type pluginRemoveFlags struct {
	skipBackup bool
}

func newPluginRemoveCmd(_ *GlobalFlags) *cobra.Command {
	f := &pluginRemoveFlags{}

	removeCmd := &cobra.Command{
		Use:     "remove",
		Aliases: []string{"uninstall"},
		Short:   "Remove the Zellij plugin, layout, and hooks",
		Long: `Remove the cc-deck WASM plugin, layout file, and Claude Code hooks from
settings.json. A timestamped backup of settings.json is created before
modification unless --skip-backup is specified.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runPluginRemove(f)
		},
	}

	removeCmd.Flags().BoolVar(&f.skipBackup, "skip-backup", false, "Skip creating backup of settings.json")

	return removeCmd
}

func runPluginInstall(f *pluginInstallFlags) error {
	err := plugin.Install(plugin.InstallOptions{
		Force:      f.force,
		SkipBackup: f.skipBackup,
		Layout:     f.layout,
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
		Stdin:      os.Stdin,
	})
	if err != nil {
		if err.Error() == "cancelled by user" {
			os.Exit(2)
		}
		return err
	}
	return nil
}

func runPluginRemove(f *pluginRemoveFlags) error {
	return plugin.Remove(plugin.RemoveOptions{
		SkipBackup: f.skipBackup,
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
	})
}
