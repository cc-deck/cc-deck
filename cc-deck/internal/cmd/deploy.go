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

// DeployFlags holds the flags for the deploy command.
type DeployFlags struct {
	Profile         string
	Storage         string
	Image           string
	SyncDir         string
	AllowEgress     []string
	NoNetworkPolicy bool
	Overlay         string
}

// NewDeployCmd creates the deploy cobra command.
func NewDeployCmd(globalFlags *GlobalFlags) *cobra.Command {
	flags := &DeployFlags{}

	cmd := &cobra.Command{
		Use:    "deploy <name>",
		Short:  "Deploy a new Claude Code session to Kubernetes",
		Hidden: true,
		Long: `Deploy a new Claude Code session as a StatefulSet on Kubernetes.

This creates a StatefulSet (replicas=1), headless Service, ConfigMap for
Zellij configuration, PVC for persistent storage, and a NetworkPolicy
for egress control. The session Pod runs Zellij with the Claude Code
web server enabled.

The session name must be unique within the namespace.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDeploy(cmd.Context(), args[0], flags, globalFlags)
		},
	}

	cmd.Flags().StringVar(&flags.Profile, "profile", "", "Credential profile to use (overrides global --profile)")
	cmd.Flags().StringVar(&flags.Storage, "storage", "", "PVC storage size (e.g., 10Gi)")
	cmd.Flags().StringVar(&flags.Image, "image", "", "Container image (e.g., ghcr.io/org/claude-code:latest)")
	cmd.Flags().StringVar(&flags.SyncDir, "sync-dir", "", "Local directory to sync to the Pod on deploy")
	cmd.Flags().StringSliceVar(&flags.AllowEgress, "allow-egress", nil, "Additional egress hosts to allow (can be repeated)")
	cmd.Flags().BoolVar(&flags.NoNetworkPolicy, "no-network-policy", false, "Skip creating NetworkPolicy")
	cmd.Flags().StringVar(&flags.Overlay, "overlay", "", "Path to a kustomize overlay directory to merge with generated resources")

	return cmd
}

func runDeploy(ctx context.Context, sessionName string, flags *DeployFlags, gf *GlobalFlags) error {
	// Load config
	cfg, err := config.Load(gf.ConfigFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Resolve profile name (deploy flag > global flag > config default)
	profileName := flags.Profile
	if profileName == "" {
		profileName = cfg.ResolveProfile(gf.Profile)
	}
	if profileName == "" {
		return fmt.Errorf("no profile specified; use --profile flag or set a default profile")
	}

	profile, err := cfg.GetProfile(profileName)
	if err != nil {
		return fmt.Errorf("loading profile: %w", err)
	}

	// Create K8s client
	client, err := k8s.NewClient(k8s.ClientOptions{
		Kubeconfig: gf.Kubeconfig,
		Namespace:  gf.Namespace,
	})
	if err != nil {
		return fmt.Errorf("creating kubernetes client: %w", err)
	}

	// Detect cluster capabilities
	caps, err := k8s.DetectCapabilities(client.DiscoveryClient)
	if err != nil {
		// Non-fatal: proceed without OpenShift features
		caps = &k8s.ClusterCapabilities{}
	}

	// Resolve image and tag from flags or config defaults
	image, imageTag := resolveImage(flags.Image, cfg.Defaults)

	// Resolve storage size
	storage := flags.Storage
	if storage == "" && cfg.Defaults.StorageSize != "" {
		storage = cfg.Defaults.StorageSize
	}

	// Run deploy workflow
	result, err := session.Deploy(ctx, session.DeployOptions{
		Name:            sessionName,
		Namespace:       client.Namespace,
		ProfileName:     profileName,
		Profile:         profile,
		Image:           image,
		ImageTag:        imageTag,
		StorageSize:     storage,
		SyncDir:         flags.SyncDir,
		NoNetworkPolicy: flags.NoNetworkPolicy,
		AllowEgress:     flags.AllowEgress,
		Overlay:         flags.Overlay,
		Clientset:       client.Clientset,
		RestConfig:      client.RestConfig,
		Caps:            caps,
		Verbose:         gf.Verbose,
	})
	if err != nil {
		if _, ok := err.(*session.ResourceConflictError); ok {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(session.ExitCodeResourceConflict)
		}
		return err
	}

	// T015: Track session in local config
	cfg.AddSession(config.Session{
		Name:      sessionName,
		Namespace: result.Namespace,
		Profile:   profileName,
		PodName:   result.PodName,
		Status:    "running",
		SyncDir:   flags.SyncDir,
	})

	if err := cfg.Save(gf.ConfigFile); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not save session to config: %v\n", err)
	}

	fmt.Printf("Session %q deployed successfully\n", sessionName)
	fmt.Printf("  Pod:       %s\n", result.PodName)
	fmt.Printf("  Namespace: %s\n", result.Namespace)

	return nil
}

// resolveImage determines the container image and tag from flags or config defaults.
func resolveImage(flagImage string, defaults config.Defaults) (string, string) {
	if flagImage != "" {
		// Parse image:tag from the flag value
		image, tag := parseImageRef(flagImage)
		return image, tag
	}

	image := defaults.Image
	if image == "" {
		image = "ghcr.io/anthropics/claude-code"
	}

	tag := defaults.ImageTag
	if tag == "" {
		tag = "latest"
	}

	return image, tag
}

// parseImageRef splits "image:tag" into image and tag parts.
func parseImageRef(ref string) (string, string) {
	// Handle digest references (image@sha256:...)
	if atIdx := indexOf(ref, '@'); atIdx >= 0 {
		return ref[:atIdx], ref[atIdx:]
	}

	// Handle tag references (image:tag)
	// But not port-only colons (localhost:5000/image)
	lastColon := lastIndexOf(ref, ':')
	if lastColon < 0 {
		return ref, "latest"
	}

	// Check if the colon is part of a port (before any slash after the colon)
	afterColon := ref[lastColon+1:]
	if slashIdx := indexOf(afterColon, '/'); slashIdx >= 0 {
		// Colon is in the registry part, not a tag separator
		return ref, "latest"
	}

	return ref[:lastColon], ref[lastColon+1:]
}

func indexOf(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

func lastIndexOf(s string, c byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == c {
			return i
		}
	}
	return -1
}
