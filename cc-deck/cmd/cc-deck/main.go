package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	appName       = "cc-deck"
	configDirName = "cc-deck"
	configFile    = "config.yaml"
)

var (
	cfgFile    string
	kubeconfig string
	namespace  string
	profile    string
	verbose    bool
	output     string
)

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   appName,
		Short: "Deploy and manage Claude Code sessions on Kubernetes/OpenShift",
		Long: `cc-deck deploys and manages Claude Code + Zellij sessions as StatefulSets
on Kubernetes and OpenShift clusters. It supports multiple credential profiles
(Anthropic API, Vertex AI) and provides commands for deploying, connecting,
and managing remote Claude Code sessions.`,
		SilenceUsage: true,
	}

	rootCmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig (default: $KUBECONFIG or ~/.kube/config)")
	rootCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "", "Target namespace (default: current context namespace)")
	rootCmd.PersistentFlags().StringVarP(&profile, "profile", "p", "", "Credential profile to use (default: config default)")
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", fmt.Sprintf("Config file (default: $XDG_CONFIG_HOME/%s/%s)", configDirName, configFile))
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	rootCmd.PersistentFlags().StringVarP(&output, "output", "o", "text", "Output format (text, json, yaml)")

	cobra.OnInitialize(initConfig)

	return rootCmd
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		configDir := filepath.Join(xdg.ConfigHome, configDirName)
		viper.AddConfigPath(configDir)
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if verbose {
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
