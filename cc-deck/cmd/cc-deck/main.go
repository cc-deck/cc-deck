package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/rhuss/cc-mux/cc-deck/internal/cmd"
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
		Short: "Deploy and manage Claude Code sessions on Kubernetes/OpenShift",
		Long: `cc-deck deploys and manages Claude Code + Zellij sessions as StatefulSets
on Kubernetes and OpenShift clusters. It supports multiple credential profiles
(Anthropic API, Vertex AI) and provides commands for deploying, connecting,
and managing remote Claude Code sessions.`,
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

	rootCmd.AddCommand(cmd.NewDeployCmd(gf))
	rootCmd.AddCommand(cmd.NewConnectCmd(gf))
	rootCmd.AddCommand(cmd.NewProfileCmd(gf))
	rootCmd.AddCommand(cmd.NewListCmd(gf))
	rootCmd.AddCommand(cmd.NewDeleteCmd(gf))
	rootCmd.AddCommand(cmd.NewLogsCmd(gf))
	rootCmd.AddCommand(cmd.NewVersionCmd(gf))
	rootCmd.AddCommand(cmd.NewPluginCmd(gf))
	rootCmd.AddCommand(cmd.NewSessionCmd(gf))
	rootCmd.AddCommand(cmd.NewHookCmd())
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
	rootCmd := newRootCmd()
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
