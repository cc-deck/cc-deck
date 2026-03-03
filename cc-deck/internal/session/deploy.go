package session

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/rhuss/cc-mux/cc-deck/internal/config"
	"github.com/rhuss/cc-mux/cc-deck/internal/k8s"
	filesync "github.com/rhuss/cc-mux/cc-deck/internal/sync"
)

// DeployNetworkPolicyOptions holds parameters for deploying network policies.
type DeployNetworkPolicyOptions struct {
	SessionName     string
	Namespace       string
	Profile         config.Profile
	Caps            *k8s.ClusterCapabilities
	Applier         *k8s.Applier
	NoNetworkPolicy bool
}

// DeployNetworkPolicy creates and applies the NetworkPolicy (and EgressFirewall
// on OpenShift with OVN-Kubernetes) for a session. Skipped if NoNetworkPolicy is true.
func DeployNetworkPolicy(ctx context.Context, opts DeployNetworkPolicyOptions) error {
	if opts.NoNetworkPolicy {
		return nil
	}

	// Build and apply the standard NetworkPolicy
	npParams := k8s.NetworkPolicyParams{
		SessionName:   opts.SessionName,
		Namespace:     opts.Namespace,
		Backend:       opts.Profile.Backend,
		AllowedEgress: opts.Profile.AllowedEgress,
	}

	np, err := k8s.BuildNetworkPolicy(npParams)
	if err != nil {
		return fmt.Errorf("building NetworkPolicy: %w", err)
	}

	if err := opts.Applier.ApplyNetworkPolicy(ctx, np); err != nil {
		return fmt.Errorf("applying NetworkPolicy: %w", err)
	}

	// On OpenShift with OVN-Kubernetes, also create an EgressFirewall for FQDN filtering
	if opts.Caps != nil && opts.Caps.IsOpenShift && opts.Caps.HasOVNKubernetes {
		efParams := k8s.EgressFirewallParams{
			SessionName:   opts.SessionName,
			Namespace:     opts.Namespace,
			Backend:       opts.Profile.Backend,
			AllowedEgress: opts.Profile.AllowedEgress,
		}

		ef := k8s.BuildEgressFirewall(efParams)

		egressFirewallGVR := schema.GroupVersionResource{
			Group:    "k8s.ovn.org",
			Version:  "v1",
			Resource: "egressfirewalls",
		}

		if err := opts.Applier.ApplyUnstructured(ctx, ef, egressFirewallGVR); err != nil {
			return fmt.Errorf("applying EgressFirewall: %w", err)
		}
	}

	return nil
}

// DeployInitialSyncOptions holds parameters for initial file sync during deploy.
type DeployInitialSyncOptions struct {
	SessionName string
	Namespace   string
	SyncDir     string
	Excludes    []string
	Clientset   kubernetes.Interface
	RestConfig  *rest.Config
}

// DeployInitialSync performs a push sync from the local directory to the Pod
// as part of the deploy workflow. Skipped if SyncDir is empty.
func DeployInitialSync(ctx context.Context, opts DeployInitialSyncOptions) error {
	if opts.SyncDir == "" {
		return nil
	}

	podName := k8s.ResourcePrefix(opts.SessionName) + "-0"

	syncOpts := filesync.SyncOptions{
		PodName:       podName,
		Namespace:     opts.Namespace,
		ContainerName: "claude",
		LocalDir:      opts.SyncDir,
		RemoteDir:     "/workspace",
		Excludes:      opts.Excludes,
		Clientset:     opts.Clientset,
		RestConfig:    opts.RestConfig,
	}

	if err := filesync.Push(ctx, syncOpts); err != nil {
		return fmt.Errorf("initial sync to Pod %q: %w", podName, err)
	}

	return nil
}
