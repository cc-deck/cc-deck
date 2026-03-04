package cmd

import (
	"fmt"
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
	force         bool
	layout        string
	injectDefault bool
}

func newPluginInstallCmd(_ *GlobalFlags) *cobra.Command {
	f := &pluginInstallFlags{}

	installCmd := &cobra.Command{
		Use:   "install",
		Short: "Install the Zellij plugin and layout",
		Long: `Install the embedded cc-deck WASM plugin into the Zellij plugins directory
and write a layout file to the Zellij layouts directory.

Optionally inject the plugin pane into an existing default layout with
the --inject-default flag.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runPluginInstall(f)
		},
	}

	installCmd.Flags().BoolVarP(&f.force, "force", "f", false, "Overwrite without prompting")
	installCmd.Flags().StringVarP(&f.layout, "layout", "l", "minimal", `Layout template: "minimal" or "full"`)
	installCmd.Flags().BoolVar(&f.injectDefault, "inject-default", false, "Inject plugin pane into default layout")

	return installCmd
}

func newPluginStatusCmd(gf *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show plugin installation status",
		Long:  "Display the current installation status of the cc-deck Zellij plugin, layout, and Zellij itself.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return plugin.RunStatus(os.Stdout, os.Stderr, gf.Output)
		},
	}
}

func newPluginRemoveCmd(_ *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "remove",
		Short: "Remove the Zellij plugin and layout",
		Long:  "Remove the cc-deck WASM plugin, layout file, and any injection from the default layout.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return plugin.RunRemove(os.Stdout, os.Stderr)
		},
	}
}

func runPluginInstall(f *pluginInstallFlags) error {
	if f.layout != "minimal" && f.layout != "full" {
		return fmt.Errorf("invalid layout %q: must be \"minimal\" or \"full\"", f.layout)
	}

	err := plugin.Install(plugin.InstallOptions{
		Force:         f.force,
		Layout:        f.layout,
		InjectDefault: f.injectDefault,
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
