package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/cc-deck/cc-deck/internal/setup"
	"github.com/cc-deck/cc-deck/internal/cmd"
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
		Short: "Manage Claude Code sessions with the Zellij sidebar plugin",
		Long: `cc-deck manages Claude Code + Zellij sessions through a sidebar plugin
that tracks session status, enables keyboard navigation, and provides
session snapshots. It also supports setting up container images and
SSH remotes for Claude Code environments.`,
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

	// Command groups in display order.
	rootCmd.AddGroup(
		&cobra.Group{ID: "daily", Title: "Daily:"},
		&cobra.Group{ID: "session", Title: "Session:"},
		&cobra.Group{ID: "environment", Title: "Environment:"},
		&cobra.Group{ID: "setup", Title: "Setup:"},
	)

	// Daily: promoted env subcommands.
	addToGroup(rootCmd, "daily",
		cmd.NewAttachCmd(gf),
		cmd.NewListCmd(gf),
		cmd.NewStatusCmd(gf),
		cmd.NewStartCmd(gf),
		cmd.NewStopCmd(gf),
		cmd.NewLogsCmd(gf),
	)

	// Session.
	addToGroup(rootCmd, "session",
		cmd.NewSnapshotCmd(gf),
	)

	// Environment.
	addToGroup(rootCmd, "environment",
		cmd.NewEnvCmd(gf),
	)

	// Setup.
	addToGroup(rootCmd, "setup",
		cmd.NewPluginCmd(gf),
		cmd.NewProfileCmd(gf),
		cmd.NewDomainsCmd(gf),
		cmd.NewSetupCmd(gf),
	)

	// Utility commands (ungrouped, appear under "Additional Commands").
	rootCmd.AddCommand(cmd.NewHookCmd())
	rootCmd.AddCommand(cmd.NewVersionCmd(gf))
	rootCmd.AddCommand(newCompletionCmd())

	return rootCmd
}

func newCompletionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "completion [bash|zsh|fish]",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for cc-deck.

To load completions:

Bash:
  $ source <(cc-deck completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ cc-deck completion bash > /etc/bash_completion.d/cc-deck
  # macOS:
  $ cc-deck completion bash > $(brew --prefix)/etc/bash_completion.d/cc-deck

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it. Execute the following once:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ cc-deck completion zsh > "${fpath[1]}/_cc-deck"

  # You will need to start a new shell for this setup to take effect.

Fish:
  $ cc-deck completion fish | source

  # To load completions for each session, execute once:
  $ cc-deck completion fish > ~/.config/fish/completions/cc-deck.fish
`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				return cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				return cmd.Root().GenFishCompletion(os.Stdout, true)
			default:
				return fmt.Errorf("unsupported shell: %s", args[0])
			}
		},
	}
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
	// Propagate build-time registry to the setup package
	if cmd.ImageRegistry != "" {
		setup.DefaultBaseImage = cmd.ImageRegistry + "/cc-deck-base:latest"
	}

	rootCmd := newRootCmd()
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
