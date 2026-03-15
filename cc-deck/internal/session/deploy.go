package session

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/cc-deck/cc-deck/internal/config"
	"github.com/cc-deck/cc-deck/internal/k8s"
	filesync "github.com/cc-deck/cc-deck/internal/sync"
)

// ExitCodeResourceConflict is the exit code for duplicate session name.
const ExitCodeResourceConflict = 5

// ResourceConflictError indicates a session with the same name already exists.
type ResourceConflictError struct {
	Name string
}

func (e *ResourceConflictError) Error() string {
	return fmt.Sprintf("session %q already exists (StatefulSet exists in cluster)", e.Name)
}

// DeployOptions holds parameters for the deploy workflow.
type DeployOptions struct {
	Name            string
	Namespace       string
	ProfileName     string
	Profile         config.Profile
	Image           string
	ImageTag        string
	StorageSize     string
	SyncDir         string
	NoNetworkPolicy bool
	AllowEgress     []string
	Overlay         string
	Clientset       kubernetes.Interface
	RestConfig      *rest.Config
	Caps            *k8s.ClusterCapabilities
	Verbose         bool
}

// DeployResult holds the outcome of a deploy operation.
type DeployResult struct {
	PodName   string
	Namespace string
}

// Deploy runs the full deploy workflow: check for duplicates, validate inputs,
// build resources, apply to cluster, wait for Pod Running, and return result.
func Deploy(ctx context.Context, opts DeployOptions) (*DeployResult, error) {
	// T017: Check for duplicate session
	if err := checkDuplicateSession(ctx, opts.Clientset, opts.Namespace, opts.Name); err != nil {
		return nil, err
	}

	// T011: Validate profile
	if err := opts.Profile.Validate(); err != nil {
		return nil, fmt.Errorf("invalid profile: %w", err)
	}

	// Validate Secrets exist in namespace (T012)
	if err := ValidateProfileSecrets(ctx, opts.Clientset, opts.Namespace, opts.Profile); err != nil {
		return nil, err
	}

	// Merge AllowEgress from flag into profile
	mergedProfile := opts.Profile
	if len(opts.AllowEgress) > 0 {
		mergedProfile.AllowedEgress = append(mergedProfile.AllowedEgress, opts.AllowEgress...)
	}

	// Build resource parameters
	params := k8s.SessionParams{
		Name:        opts.Name,
		Namespace:   opts.Namespace,
		Profile:     mergedProfile,
		Image:       opts.Image,
		ImageTag:    opts.ImageTag,
		StorageSize: opts.StorageSize,
	}

	// Create applier
	applier, err := k8s.NewApplier(opts.RestConfig)
	if err != nil {
		return nil, fmt.Errorf("creating applier: %w", err)
	}

	// Build resources
	cm := k8s.BuildZellijConfigMap(params)
	sts := k8s.BuildStatefulSet(params)
	svc := k8s.BuildHeadlessService(params)

	// Apply resources, using kustomize overlay if specified
	if opts.Overlay != "" {
		if err := k8s.ApplyWithOverlay(ctx, applier, opts.Overlay, cm, sts, svc); err != nil {
			return nil, fmt.Errorf("applying resources with overlay: %w", err)
		}
	} else {
		if err := applier.ApplyConfigMap(ctx, cm); err != nil {
			return nil, fmt.Errorf("applying Zellij ConfigMap: %w", err)
		}
		if err := applier.ApplyStatefulSet(ctx, sts); err != nil {
			return nil, fmt.Errorf("applying StatefulSet: %w", err)
		}
		if err := applier.ApplyService(ctx, svc); err != nil {
			return nil, fmt.Errorf("applying Service: %w", err)
		}
	}

	// Deploy NetworkPolicy
	if err := DeployNetworkPolicy(ctx, DeployNetworkPolicyOptions{
		SessionName:     opts.Name,
		Namespace:       opts.Namespace,
		Profile:         mergedProfile,
		Caps:            opts.Caps,
		Applier:         applier,
		NoNetworkPolicy: opts.NoNetworkPolicy,
	}); err != nil {
		return nil, fmt.Errorf("deploying network policy: %w", err)
	}

	podName := k8s.ResourcePrefix(opts.Name) + "-0"

	// T014: Wait for Pod readiness
	if err := WaitForPodRunning(ctx, opts.Clientset, opts.Namespace, podName, 60*time.Second, opts.Verbose); err != nil {
		return nil, fmt.Errorf("waiting for Pod: %w", err)
	}

	// Initial file sync if specified
	if err := DeployInitialSync(ctx, DeployInitialSyncOptions{
		SessionName: opts.Name,
		Namespace:   opts.Namespace,
		SyncDir:     opts.SyncDir,
		Clientset:   opts.Clientset,
		RestConfig:  opts.RestConfig,
	}); err != nil {
		return nil, fmt.Errorf("initial file sync: %w", err)
	}

	return &DeployResult{
		PodName:   podName,
		Namespace: opts.Namespace,
	}, nil
}

// checkDuplicateSession checks whether a StatefulSet for this session already
// exists. Returns a ResourceConflictError if so.
func checkDuplicateSession(ctx context.Context, clientset kubernetes.Interface, namespace, name string) error {
	stsName := k8s.ResourcePrefix(name)
	_, err := clientset.AppsV1().StatefulSets(namespace).Get(ctx, stsName, metav1.GetOptions{})
	if err == nil {
		return &ResourceConflictError{Name: name}
	}
	if !errors.IsNotFound(err) {
		return fmt.Errorf("checking for existing session: %w", err)
	}
	return nil
}

// WaitForPodRunning watches a Pod until it reaches the Running phase or the
// timeout expires. If the Pod stays Pending beyond the timeout, it fetches
// and returns recent events for the Pod to help diagnose scheduling failures.
func WaitForPodRunning(ctx context.Context, clientset kubernetes.Interface, namespace, podName string, timeout time.Duration, verbose bool) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Check if Pod already exists and is Running
	pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err == nil && pod.Status.Phase == corev1.PodRunning {
		return nil
	}

	// Watch for Pod phase changes
	watcher, err := clientset.CoreV1().Pods(namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("metadata.name", podName).String(),
	})
	if err != nil {
		return fmt.Errorf("watching Pod %q: %w", podName, err)
	}
	defer watcher.Stop()

	for {
		select {
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return fmt.Errorf("watch channel closed for Pod %q", podName)
			}

			if event.Type == watch.Error {
				return fmt.Errorf("watch error for Pod %q", podName)
			}

			pod, ok := event.Object.(*corev1.Pod)
			if !ok {
				continue
			}

			switch pod.Status.Phase {
			case corev1.PodRunning:
				return nil
			case corev1.PodFailed, corev1.PodSucceeded:
				return fmt.Errorf("Pod %q terminated with phase %s", podName, pod.Status.Phase)
			}

			if verbose {
				fmt.Printf("Pod %q phase: %s\n", podName, pod.Status.Phase)
			}

		case <-ctx.Done():
			// Timeout: fetch events for diagnostic info
			events, evtErr := getPodEvents(context.Background(), clientset, namespace, podName)
			if evtErr != nil || events == "" {
				return fmt.Errorf("timed out waiting for Pod %q to become Running", podName)
			}
			return fmt.Errorf("timed out waiting for Pod %q to become Running. Recent events:\n%s", podName, events)
		}
	}
}

// getPodEvents fetches recent events for a Pod and formats them as a string.
func getPodEvents(ctx context.Context, clientset kubernetes.Interface, namespace, podName string) (string, error) {
	eventList, err := clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Pod", podName),
	})
	if err != nil {
		return "", err
	}

	if len(eventList.Items) == 0 {
		return "", nil
	}

	var result string
	for _, e := range eventList.Items {
		result += fmt.Sprintf("  %s: %s - %s\n", e.Type, e.Reason, e.Message)
	}
	return result, nil
}

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
