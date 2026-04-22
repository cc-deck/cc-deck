package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/cc-deck/cc-deck/internal/build"
	"github.com/cc-deck/cc-deck/internal/cmd"
	"github.com/cc-deck/cc-deck/internal/ws"
	"github.com/cc-deck/cc-deck/internal/xdg"
)

const (
	appName       = "cc-deck"
	configDirName = "cc-deck"
	configFile    = "config.yaml"
)

func newRootCmd() *cobra.Command {
	gf := &cmd.GlobalFlags{}

	rootCmd := &cobra.Command{
		Use:   appName,
		Short: "Manage Claude Code workspaces with the Zellij sidebar plugin",
		Long: `cc-deck manages Claude Code + Zellij sessions through a sidebar plugin
that tracks session status, enables keyboard navigation, and provides
session snapshots. It also supports building container images and
provisioning SSH remotes for Claude Code workspaces.`,
		SilenceUsage: true,
	}

	rootCmd.PersistentFlags().StringVar(&gf.Kubeconfig, "kubeconfig", "", "Path to kubeconfig (default: $KUBECONFIG or ~/.kube/config)")
	rootCmd.PersistentFlags().StringVarP(&gf.Namespace, "namespace", "n", "", "Target namespace (default: current context namespace)")
	rootCmd.PersistentFlags().StringVarP(&gf.Profile, "profile", "p", "", "Credential profile to use (default: config default)")
	rootCmd.PersistentFlags().StringVar(&gf.ConfigFile, "config", "", fmt.Sprintf("Config file (default: $XDG_CONFIG_HOME/%s/%s)", configDirName, configFile))
	rootCmd.PersistentFlags().BoolVarP(&gf.Verbose, "verbose", "v", false, "Verbose output")
	rootCmd.PersistentFlags().StringVarP(&gf.Output, "output", "o", "text", "Output format (text, json, yaml)")

	cobra.OnInitialize(func() {
		initConfig(gf)
	})

	rootCmd.AddGroup(
		&cobra.Group{ID: "workspace", Title: "Workspace:"},
		&cobra.Group{ID: "session", Title: "Session:"},
		&cobra.Group{ID: "build", Title: "Build:"},
		&cobra.Group{ID: "config", Title: "Config:"},
	)

	addToGroup(rootCmd, "workspace",
		cmd.NewAttachCmd(gf),
		cmd.NewListCmd(gf),
		cmd.NewExecCmd(gf),
		cmd.NewWsCmd(gf),
	)

	addToGroup(rootCmd, "session",
		cmd.NewSnapshotCmd(gf),
	)

	addToGroup(rootCmd, "build",
		cmd.NewBuildCmd(gf),
	)

	addToGroup(rootCmd, "config",
		cmd.NewConfigCmd(gf),
	)

	rootCmd.AddCommand(cmd.NewHookCmd())
	rootCmd.AddCommand(cmd.NewVersionCmd(gf))

	return rootCmd
}

func addToGroup(parent *cobra.Command, groupID string, cmds ...*cobra.Command) {
	for _, c := range cmds {
		c.GroupID = groupID
		parent.AddCommand(c)
	}
}

func initConfig(gf *cmd.GlobalFlags) {
	if gf.ConfigFile != "" {
		viper.SetConfigFile(gf.ConfigFile)
	} else {
		configDir := filepath.Join(xdg.ConfigHome, configDirName)
		viper.AddConfigPath(configDir)
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if gf.Verbose {
			if _, ok := err.(viper.ConfigFileNotFoundError); ok {
				fmt.Fprintf(os.Stderr, "Config file not found, using defaults\n")
			} else {
				fmt.Fprintf(os.Stderr, "Error reading config: %v\n", err)
			}
		}
	}
}

func main() {
	// Propagate build-time registry to the build package
	if cmd.ImageRegistry != "" {
		build.DefaultBaseImage = cmd.ImageRegistry + "/cc-deck-base:latest"
	}

	rootCmd := newRootCmd()
	if err := rootCmd.Execute(); err != nil {
		var chErr *ws.ChannelError
		if errors.As(err, &chErr) {
			verbose, _ := rootCmd.PersistentFlags().GetBool("verbose")
			if verbose && chErr.Err != nil {
				fmt.Fprintf(os.Stderr, "Channel: %s, Op: %s, Workspace: %s\n", chErr.Channel, chErr.Op, chErr.Workspace)
				fmt.Fprintf(os.Stderr, "Cause: %v\n", chErr.Err)
			}
		}
		os.Exit(1)
	}
}
