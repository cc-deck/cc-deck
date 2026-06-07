package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cc-deck/cc-deck/internal/agent"
	"github.com/cc-deck/cc-deck/internal/plugin"
)

// NewPluginCmd creates the plugin cobra command with subcommands.
func NewPluginCmd(gf *GlobalFlags) *cobra.Command {
	pluginCmd := &cobra.Command{
		Use:   "plugin",
		Short: "Manage the Zellij plugin and agent hooks",
		Long:  "Install, inspect, and remove the cc-deck Zellij plugin and AI agent hooks.",
	}

	pluginCmd.AddCommand(
		newPluginInstallCmd(gf),
		newPluginStatusCmd(gf),
		newPluginRemoveCmd(gf),
	)

	return pluginCmd
}

type pluginInstallFlags struct {
	force         bool
	skipBackup    bool
	layout        string
	installZellij bool
	zellijVersion string
	agents        string
}

func newPluginInstallCmd(_ *GlobalFlags) *cobra.Command {
	f := &pluginInstallFlags{}

	installCmd := &cobra.Command{
		Use:   "install",
		Short: "Install the Zellij plugin, layout, and agent hooks",
		Long: `Install the embedded cc-deck WASM plugin into the Zellij plugins directory,
write sidebar layouts to the Zellij layouts directory, and register hooks
for all detected AI agents.

Layout variants:
  minimal   Sidebar + compact-bar at bottom (default)
  standard  Sidebar + tab-bar at top + status-bar at bottom (beginner-friendly)
  clean     Sidebar only, no bars (maximum terminal space)

All variants are written as layout files. The --layout flag sets the default
(cc-deck.kdl). Use "zellij --layout cc-deck-standard" to try other variants.

Use --agents to install hooks for specific agents only (comma-separated).`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runPluginInstall(f)
		},
	}

	installCmd.Flags().BoolVarP(&f.force, "force", "f", false, "Overwrite without prompting")
	installCmd.Flags().BoolVar(&f.skipBackup, "skip-backup", false, "Skip creating backup of settings.json")
	installCmd.Flags().StringVar(&f.layout, "layout", "standard", "Default layout variant (standard, minimal, clean)")
	installCmd.Flags().BoolVar(&f.installZellij, "install-zellij", false, "Download and install Zellij binary")
	installCmd.Flags().StringVar(&f.zellijVersion, "zellij-version", "", "Zellij version to install (default: latest release)")
	installCmd.Flags().StringVar(&f.agents, "agents", "", "Comma-separated list of agent names to install hooks for (default: all detected)")

	return installCmd
}

func newPluginStatusCmd(gf *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show plugin and agent hook status",
		Long:  "Display the current installation status of the cc-deck Zellij plugin, layout, and agent hooks.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := plugin.RunStatus(os.Stdout, os.Stderr, gf.Output); err != nil {
				return err
			}
			printAgentStatus(os.Stdout)
			return nil
		},
	}
}

type pluginRemoveFlags struct {
	skipBackup bool
	agents     string
}

func newPluginRemoveCmd(_ *GlobalFlags) *cobra.Command {
	f := &pluginRemoveFlags{}

	removeCmd := &cobra.Command{
		Use:     "remove",
		Aliases: []string{"uninstall"},
		Short:   "Remove the Zellij plugin, layout, and agent hooks",
		Long: `Remove the cc-deck WASM plugin, layout file, and agent hooks.

Use --agents to uninstall hooks for specific agents only (comma-separated).
Without --agents, removes all hooks and the plugin.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if f.agents != "" {
				return runAgentUninstall(f)
			}
			return runPluginRemove(f)
		},
	}

	removeCmd.Flags().BoolVar(&f.skipBackup, "skip-backup", false, "Skip creating backup of settings.json")
	removeCmd.Flags().StringVar(&f.agents, "agents", "", "Comma-separated list of agent names to remove hooks for")

	return removeCmd
}

func runPluginInstall(f *pluginInstallFlags) error {
	err := plugin.Install(plugin.InstallOptions{
		Force:         f.force,
		SkipBackup:    f.skipBackup,
		Layout:        f.layout,
		InstallZellij: f.installZellij,
		ZellijVersion: f.zellijVersion,
		AgentFilter:   parseAgentFilter(f.agents),
		Stdout:        os.Stdout,
		Stderr:        os.Stderr,
		Stdin:         os.Stdin,
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

func runAgentUninstall(f *pluginRemoveFlags) error {
	filter := parseAgentFilter(f.agents)
	for _, a := range agent.All() {
		if len(filter) > 0 && !filter[a.Name()] {
			continue
		}
		if !a.HooksInstalled() {
			fmt.Fprintf(os.Stdout, "%s: no hooks installed\n", a.DisplayName())
			continue
		}
		if err := a.UninstallHooks(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Could not remove hooks for %s: %v\n", a.DisplayName(), err)
			continue
		}
		fmt.Fprintf(os.Stdout, "%s: hooks removed\n", a.DisplayName())
	}
	return nil
}

func printAgentStatus(w *os.File) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Agents")
	for _, a := range agent.All() {
		installed := "not detected"
		if a.IsInstalled() {
			installed = "detected"
		}
		hooked := "no"
		if a.HooksInstalled() {
			hooked = "yes"
		}
		configPath := a.DetectConfig()
		if configPath == "" {
			configPath = "N/A"
		}
		fmt.Fprintf(w, "  %-14s %s, hooks: %s, config: %s\n",
			a.DisplayName()+":", installed, hooked, configPath)
	}
}

func parseAgentFilter(agents string) map[string]bool {
	if agents == "" {
		return nil
	}
	filter := make(map[string]bool)
	for _, name := range strings.Split(agents, ",") {
		name = strings.TrimSpace(name)
		if name != "" {
			filter[name] = true
		}
	}
	return filter
}
