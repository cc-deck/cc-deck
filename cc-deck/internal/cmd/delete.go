package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/cc-deck/cc-deck/internal/config"
	"github.com/cc-deck/cc-deck/internal/k8s"
	"github.com/cc-deck/cc-deck/internal/session"
)

// NewDeleteCmd creates the delete cobra command.
func NewDeleteCmd(globalFlags *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete <name>",
		Aliases: []string{"rm"},
		Short:   "Delete a Claude Code session and its resources",
		Hidden:  true,
		Long: `Delete a Claude Code session, removing all associated Kubernetes resources:

  - StatefulSet
  - Service (headless)
  - PersistentVolumeClaim
  - NetworkPolicy
  - ConfigMap (Zellij config)
  - Route/Ingress (if applicable)
  - EgressFirewall (OpenShift only, if applicable)

The session is also removed from the local config file.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDelete(cmd, args[0], globalFlags)
		},
	}

	return cmd
}

func runDelete(cmd *cobra.Command, sessionName string, gf *GlobalFlags) error {
	// Load config to find the session's namespace
	cfg, err := config.Load(gf.ConfigFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	sess := cfg.FindSession(sessionName)
	if sess == nil {
		return fmt.Errorf("session %q not found in config", sessionName)
	}

	// Use the session's namespace, but allow override from flag
	namespace := sess.Namespace
	if gf.Namespace != "" {
		namespace = gf.Namespace
	}

	client, err := k8s.NewClient(k8s.ClientOptions{
		Kubeconfig: gf.Kubeconfig,
		Namespace:  namespace,
	})
	if err != nil {
		return fmt.Errorf("creating kubernetes client: %w", err)
	}

	caps, err := k8s.DetectCapabilities(client.DiscoveryClient)
	if err != nil {
		// Non-fatal: proceed without OpenShift-specific cleanup
		caps = &k8s.ClusterCapabilities{}
	}

	return session.Delete(cmd.Context(), client.Clientset, client.RestConfig, caps, os.Stdout, sessionName, client.Namespace, session.DeleteOptions{
		ConfigPath: gf.ConfigFile,
		Verbose:    gf.Verbose,
	})
}
