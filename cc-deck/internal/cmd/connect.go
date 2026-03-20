package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/cc-deck/cc-deck/internal/config"
	"github.com/cc-deck/cc-deck/internal/k8s"
	"github.com/cc-deck/cc-deck/internal/session"
)

// ConnectFlags holds the flags for the connect command.
type ConnectFlags struct {
	Method string
	Web    bool
	Port   int
}

// NewConnectCmd creates the connect cobra command.
func NewConnectCmd(globalFlags *GlobalFlags) *cobra.Command {
	flags := &ConnectFlags{}

	cmd := &cobra.Command{
		Use:    "connect <name>",
		Short:  "Connect to a running Claude Code session",
		Hidden: true,
		Long: `DEPRECATED: Use 'cc-deck env attach' instead. This command will be removed in a future release.

Connect to a running Claude Code session on Kubernetes.

Connection methods:
  exec          Interactive terminal via kubectl exec (default)
  web           Port-forward + open browser with Zellij web client
  port-forward  Port-forward only (no browser)

Auto-detection: If a Route (OpenShift) or Ingress exists, uses the web URL.
Otherwise defaults to exec.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConnect(cmd.Context(), args[0], flags, globalFlags)
		},
	}

	cmd.Flags().StringVar(&flags.Method, "method", "", "Connection method: exec, web, port-forward (default: auto-detect)")
	cmd.Flags().BoolVar(&flags.Web, "web", false, "Shorthand for --method web")
	cmd.Flags().IntVar(&flags.Port, "port", 8082, "Local port for port-forward")

	return cmd
}

func runConnect(ctx context.Context, sessionName string, flags *ConnectFlags, gf *GlobalFlags) error {
	// Load config
	cfg, err := config.Load(gf.ConfigFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Determine connection method
	method := flags.Method
	if flags.Web {
		method = session.MethodWeb
	}

	// Create K8s client
	client, err := k8s.NewClient(k8s.ClientOptions{
		Kubeconfig: gf.Kubeconfig,
		Namespace:  gf.Namespace,
	})
	if err != nil {
		return fmt.Errorf("creating kubernetes client: %w", err)
	}

	// Detect cluster capabilities for Route/Ingress discovery
	caps, err := k8s.DetectCapabilities(client.DiscoveryClient)
	if err != nil {
		// Non-fatal: just skip Route detection
		caps = &k8s.ClusterCapabilities{}
	}

	// Connect
	result, err := session.Connect(ctx, session.ConnectOptions{
		SessionName: sessionName,
		Method:      method,
		LocalPort:   flags.Port,
		Clientset:   client.Clientset,
		RestConfig:  client.RestConfig,
		Namespace:   client.Namespace,
		Caps:        caps,
	})
	if err != nil {
		return fmt.Errorf("connecting to session %q: %w", sessionName, err)
	}

	// Update session config with connection details
	session.UpdateSessionConnection(cfg, sessionName, result)
	if saveErr := cfg.Save(gf.ConfigFile); saveErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not save connection details: %v\n", saveErr)
	}

	return nil
}
