package env

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

// K8sDeployEnvironment manages a Kubernetes-based persistent development
// environment backed by a StatefulSet with PVC storage.
type K8sDeployEnvironment struct {
	name  string
	store *FileStateStore
	defs  *DefinitionStore

	// K8s connection
	Namespace  string // Target namespace
	Kubeconfig string // Path to kubeconfig (empty = default)
	Context    string // Kubeconfig context (empty = current)

	// Storage
	StorageSize  string // PVC size (default: "10Gi")
	StorageClass string // StorageClass name (empty = cluster default)

	// Credentials
	Credentials    map[string]string // Inline key=value pairs
	ExistingSecret string            // Reference to pre-existing Secret
	SecretStore    string            // ESO SecretStore type
	SecretStoreRef string            // ESO SecretStore name
	SecretPath     string            // ESO secret path

	// Network
	NoNetworkPolicy bool     // Skip NetworkPolicy creation
	AllowDomains    []string // Additional allowed domains
	AllowGroups     []string // Additional allowed domain groups

	// MCP
	BuildDir string // Build directory containing cc-deck-image.yaml

	// Lifecycle
	KeepVolumes bool          // Preserve PVCs on delete
	Timeout     time.Duration // Pod readiness timeout

	// Auth
	Auth AuthMode
}

const (
	k8sResourcePrefix   = "cc-deck-"
	defaultStorageSize  = "10Gi"
	defaultPodTimeout   = 5 * time.Minute
	k8sWorkspacePath    = "/workspace"
	k8sCredentialPath   = "/run/secrets/cc-deck"
)

func k8sResourceName(envName string) string {
	return k8sResourcePrefix + envName
}

func k8sPodName(envName string) string {
	return k8sResourceName(envName) + "-0"
}

// Type returns EnvironmentTypeK8sDeploy.
func (e *K8sDeployEnvironment) Type() EnvironmentType {
	return EnvironmentTypeK8sDeploy
}

// Name returns the environment name.
func (e *K8sDeployEnvironment) Name() string {
	return e.name
}

// Create provisions a new k8s-deploy environment.
func (e *K8sDeployEnvironment) Create(ctx context.Context, opts CreateOpts) error {
	if err := ValidateEnvName(e.name); err != nil {
		return err
	}

	// Check kubectl availability.
	if _, err := exec.LookPath("kubectl"); err != nil {
		return fmt.Errorf("kubectl not found in PATH: %w", ErrKubectlNotFound)
	}

	// Fail fast if an environment with this name already exists.
	if _, err := e.store.FindInstanceByName(e.name); err == nil {
		return fmt.Errorf("instance %q: %w", e.name, ErrNameConflict)
	}

	// Build K8s client.
	client, err := NewK8sClient(e.Kubeconfig, e.Context)
	if err != nil {
		return fmt.Errorf("creating K8s client: %w", err)
	}

	// Validate namespace exists.
	if e.Namespace == "" {
		e.Namespace = "default"
	}
	if err := client.ValidateNamespace(ctx, e.Namespace); err != nil {
		return fmt.Errorf("namespace %q: %w", e.Namespace, err)
	}

	// Resolve image.
	image := opts.Image
	if image == "" {
		log.Printf("WARNING: no image specified, using default %s", defaultImage)
		image = defaultImage
	}

	// Resolve storage size.
	storageSize := e.StorageSize
	if storageSize == "" {
		storageSize = defaultStorageSize
	}

	// Resolve credentials.
	creds := e.Credentials
	if creds == nil {
		creds = make(map[string]string)
	}

	// Auth mode detection and credential injection.
	authMode := e.Auth
	if authMode == "" || authMode == AuthModeAuto {
		authMode = DetectAuthMode()
	}
	if authMode != AuthModeNone {
		DetectAuthCredentials(authMode, creds)
	}

	// Resolve allowed domains for NetworkPolicy.
	var resolvedDomains []string
	if !e.NoNetworkPolicy {
		resolvedDomains, err = e.resolveDomains()
		if err != nil {
			return fmt.Errorf("resolving domains: %w", err)
		}
	}

	// Load MCP sidecars from build manifest if specified.
	var mcpSidecars []MCPSidecarOpts
	if e.BuildDir != "" {
		mcpSidecars, err = e.loadMCPSidecars()
		if err != nil {
			return fmt.Errorf("loading MCP sidecars: %w", err)
		}
	}

	// Build resource generation options.
	labels := k8sStandardLabels(e.name)
	resourceOpts := K8sResourceOpts{
		Name:            e.name,
		Namespace:       e.Namespace,
		Image:           image,
		StorageSize:     storageSize,
		StorageClass:    e.StorageClass,
		Credentials:     creds,
		ExistingSecret:  e.ExistingSecret,
		Domains:         resolvedDomains,
		NoNetworkPolicy: e.NoNetworkPolicy,
		MCPSidecars:     mcpSidecars,
		Labels:          labels,
	}

	// Generate K8s resources.
	resources, err := GenerateResources(resourceOpts)
	if err != nil {
		return fmt.Errorf("generating K8s resources: %w", err)
	}

	// Handle credential creation.
	credResult, err := HandleCredentials(ctx, client, e.name, e.Namespace, labels, creds, e.ExistingSecret, e.SecretStore, e.SecretStoreRef, e.SecretPath)
	if err != nil {
		return fmt.Errorf("handling credentials: %w", err)
	}

	// Apply resources to cluster with cleanup on failure.
	if err := e.applyResources(ctx, client, resources, credResult); err != nil {
		e.cleanupOnFailure(ctx, client, resources, credResult)
		return fmt.Errorf("applying resources: %w", err)
	}

	// Wait for Pod readiness.
	timeout := e.Timeout
	if timeout == 0 {
		timeout = defaultPodTimeout
	}
	if err := client.WaitForPodReady(ctx, e.Namespace, k8sPodName(e.name), timeout); err != nil {
		e.cleanupOnFailure(ctx, client, resources, credResult)
		return fmt.Errorf("waiting for Pod readiness: %w", err)
	}

	// Detect and apply OpenShift-specific resources.
	if err := e.applyOpenShiftResources(ctx, client); err != nil {
		log.Printf("WARNING: OpenShift resource creation: %v", err)
	}

	// Write environment definition.
	if e.defs != nil {
		envDef := &EnvironmentDefinition{
			Name:  e.name,
			Type:  EnvironmentTypeK8sDeploy,
			Image: image,
		}
		if def, findErr := e.defs.FindByName(e.name); findErr == nil {
			_ = e.defs.Update(def)
		} else {
			_ = e.defs.Add(envDef)
		}
	}

	// Write environment instance to state store.
	resName := k8sResourceName(e.name)
	inst := &EnvironmentInstance{
		Name:      e.name,
		Type:      EnvironmentTypeK8sDeploy,
		State:     EnvironmentStateRunning,
		CreatedAt: time.Now().UTC(),
		K8s: &K8sFields{
			Namespace:   e.Namespace,
			StatefulSet: resName,
			Profile:     e.Context,
			Kubeconfig:  e.Kubeconfig,
		},
	}

	return e.store.AddInstance(inst)
}

// Attach opens an interactive session inside the K8s Pod.
func (e *K8sDeployEnvironment) Attach(ctx context.Context) error {
	// Nested Zellij detection.
	if os.Getenv("ZELLIJ") != "" {
		fmt.Fprintf(os.Stderr, "Already inside Zellij. Detach first (Ctrl+o d), then run:\n")
		fmt.Fprintf(os.Stderr, "  cc-deck env attach %s\n", e.name)
		return nil
	}

	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil {
		return err
	}

	// Auto-start if stopped.
	if inst.State == EnvironmentStateStopped {
		if startErr := e.Start(ctx); startErr != nil {
			return fmt.Errorf("auto-starting environment: %w", startErr)
		}
		inst, err = e.store.FindInstanceByName(e.name)
		if err != nil {
			return err
		}
	}

	// Update LastAttached timestamp.
	now := time.Now().UTC()
	inst.LastAttached = &now
	_ = e.store.UpdateInstance(inst)

	ns := e.resolveNamespace(inst)
	podName := k8sPodName(e.name)
	kubeconfigArgs := e.kubeconfigArgs(inst)

	// Check for existing Zellij session, attach or create.
	args := append(kubeconfigArgs, "exec", "-it", "-n", ns, podName, "--", "zellij")
	if k8sHasZellijSession(ctx, ns, podName, kubeconfigArgs) {
		args = append(args, "attach")
	} else {
		args = append(args, "-n", "cc-deck")
	}

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Start scales the StatefulSet to 1 replica and waits for Pod readiness.
func (e *K8sDeployEnvironment) Start(ctx context.Context) error {
	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil {
		return err
	}

	client, err := NewK8sClient(e.resolveKubeconfig(inst), e.resolveContext(inst))
	if err != nil {
		return fmt.Errorf("creating K8s client: %w", err)
	}

	ns := e.resolveNamespace(inst)
	resName := k8sResourceName(e.name)

	if err := client.ScaleStatefulSet(ctx, ns, resName, 1); err != nil {
		return fmt.Errorf("scaling StatefulSet: %w", err)
	}

	timeout := e.Timeout
	if timeout == 0 {
		timeout = defaultPodTimeout
	}
	if err := client.WaitForPodReady(ctx, ns, k8sPodName(e.name), timeout); err != nil {
		return fmt.Errorf("waiting for Pod readiness: %w", err)
	}

	inst.State = EnvironmentStateRunning
	return e.store.UpdateInstance(inst)
}

// Stop scales the StatefulSet to 0 replicas, preserving the PVC.
func (e *K8sDeployEnvironment) Stop(ctx context.Context) error {
	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil {
		return err
	}

	client, err := NewK8sClient(e.resolveKubeconfig(inst), e.resolveContext(inst))
	if err != nil {
		return fmt.Errorf("creating K8s client: %w", err)
	}

	ns := e.resolveNamespace(inst)
	resName := k8sResourceName(e.name)

	if err := client.ScaleStatefulSet(ctx, ns, resName, 0); err != nil {
		return fmt.Errorf("scaling StatefulSet to 0: %w", err)
	}

	inst.State = EnvironmentStateStopped
	return e.store.UpdateInstance(inst)
}

// Delete removes all K8s resources and state records for the environment.
func (e *K8sDeployEnvironment) Delete(ctx context.Context, force bool) error {
	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil {
		return err
	}

	if !force && inst.State == EnvironmentStateRunning {
		return ErrRunning
	}

	client, err := NewK8sClient(e.resolveKubeconfig(inst), e.resolveContext(inst))
	if err != nil {
		return fmt.Errorf("creating K8s client: %w", err)
	}

	ns := e.resolveNamespace(inst)
	resName := k8sResourceName(e.name)

	// Best-effort cleanup of all K8s resources.
	if err := client.DeleteStatefulSet(ctx, ns, resName); err != nil {
		log.Printf("WARNING: deleting StatefulSet: %v", err)
	}
	if err := client.DeleteService(ctx, ns, resName); err != nil {
		log.Printf("WARNING: deleting Service: %v", err)
	}
	if err := client.DeleteConfigMap(ctx, ns, resName); err != nil {
		log.Printf("WARNING: deleting ConfigMap: %v", err)
	}
	if err := client.DeleteNetworkPolicy(ctx, ns, resName); err != nil {
		log.Printf("WARNING: deleting NetworkPolicy: %v", err)
	}

	// Clean up credentials (inline Secrets are cc-deck-managed).
	credSecretName := resName + "-creds"
	if e.ExistingSecret == "" {
		if err := client.DeleteSecret(ctx, ns, credSecretName); err != nil {
			log.Printf("WARNING: deleting credential Secret: %v", err)
		}
	}

	// Clean up ESO ExternalSecret if created.
	esoName := resName + "-eso"
	if err := client.DeleteExternalSecret(ctx, ns, esoName); err != nil {
		log.Printf("WARNING: deleting ExternalSecret: %v", err)
	}

	// Clean up OpenShift resources.
	if err := client.DeleteRoute(ctx, ns, resName); err != nil {
		log.Printf("WARNING: deleting Route: %v", err)
	}
	if err := client.DeleteEgressFirewall(ctx, ns, resName); err != nil {
		log.Printf("WARNING: deleting EgressFirewall: %v", err)
	}

	// Delete PVC unless --keep-volumes.
	if !e.KeepVolumes {
		pvcName := "data-" + resName + "-0"
		if err := client.DeletePVC(ctx, ns, pvcName); err != nil {
			log.Printf("WARNING: deleting PVC: %v", err)
		}
	}

	// Remove instance from state store.
	if err := e.store.RemoveInstance(e.name); err != nil {
		log.Printf("WARNING: removing instance from state: %v", err)
	}

	// Remove definition.
	if e.defs != nil {
		if err := e.defs.Remove(e.name); err != nil {
			log.Printf("WARNING: removing definition: %v", err)
		}
	}

	return nil
}

// Status returns the current state and metadata for the environment.
func (e *K8sDeployEnvironment) Status(ctx context.Context) (*EnvironmentStatus, error) {
	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil {
		return nil, err
	}

	state := inst.State

	// Reconcile with actual K8s state.
	client, clientErr := NewK8sClient(e.resolveKubeconfig(inst), e.resolveContext(inst))
	if clientErr == nil {
		ns := e.resolveNamespace(inst)
		resName := k8sResourceName(e.name)
		if reconciled, reconcileErr := client.ReconcileState(ctx, ns, resName); reconcileErr == nil {
			state = reconciled
			if inst.State != state {
				inst.State = state
				_ = e.store.UpdateInstance(inst)
			}
		}
	}

	return &EnvironmentStatus{
		State: state,
		Since: inst.CreatedAt,
	}, nil
}

// Exec runs a command inside the K8s Pod.
func (e *K8sDeployEnvironment) Exec(ctx context.Context, cmd []string) error {
	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil {
		return err
	}
	if inst.State != EnvironmentStateRunning {
		return fmt.Errorf("environment is not running (state: %s); start it with: cc-deck env start %s", inst.State, e.name)
	}

	return k8sExec(ctx, e.resolveNamespace(inst), k8sPodName(e.name), e.kubeconfigArgs(inst), cmd, false)
}

// Push synchronizes local files into the K8s Pod via tar-over-exec.
func (e *K8sDeployEnvironment) Push(ctx context.Context, opts SyncOpts) error {
	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil {
		return err
	}
	if inst.State != EnvironmentStateRunning {
		return fmt.Errorf("environment is not running (state: %s); start it with: cc-deck env start %s", inst.State, e.name)
	}

	return k8sPush(ctx, e.resolveNamespace(inst), k8sPodName(e.name), e.kubeconfigArgs(inst), opts)
}

// Pull synchronizes files from the K8s Pod to local storage via tar-over-exec.
func (e *K8sDeployEnvironment) Pull(ctx context.Context, opts SyncOpts) error {
	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil {
		return err
	}
	if inst.State != EnvironmentStateRunning {
		return fmt.Errorf("environment is not running (state: %s); start it with: cc-deck env start %s", inst.State, e.name)
	}

	return k8sPull(ctx, e.resolveNamespace(inst), k8sPodName(e.name), e.kubeconfigArgs(inst), opts)
}

// Harvest extracts git commits from the K8s Pod via ext::kubectl exec.
func (e *K8sDeployEnvironment) Harvest(ctx context.Context, opts HarvestOpts) error {
	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil {
		return err
	}
	if inst.State != EnvironmentStateRunning {
		return fmt.Errorf("environment is not running (state: %s); start it with: cc-deck env start %s", inst.State, e.name)
	}

	return k8sHarvest(ctx, e.resolveNamespace(inst), k8sPodName(e.name), e.kubeconfigArgs(inst), opts)
}

// applyResources creates all generated K8s resources on the cluster.
func (e *K8sDeployEnvironment) applyResources(ctx context.Context, client *K8sClient, resources *K8sResourceSet, cred *CredentialResult) error {
	ns := e.Namespace

	// Create Secret first (StatefulSet references it).
	if cred != nil && cred.Secret != nil {
		if err := client.CreateSecret(ctx, ns, cred.Secret); err != nil {
			return fmt.Errorf("creating credential Secret: %w", err)
		}
	}

	// Create ExternalSecret if applicable.
	if cred != nil && cred.ExternalSecret != nil {
		if err := client.CreateExternalSecret(ctx, ns, cred.ExternalSecret); err != nil {
			return fmt.Errorf("creating ExternalSecret: %w", err)
		}
	}

	// Create headless Service (required before StatefulSet).
	if resources.Service != nil {
		if err := client.CreateService(ctx, ns, resources.Service); err != nil {
			return fmt.Errorf("creating Service: %w", err)
		}
	}

	// Create ConfigMap.
	if resources.ConfigMap != nil {
		if err := client.CreateConfigMap(ctx, ns, resources.ConfigMap); err != nil {
			return fmt.Errorf("creating ConfigMap: %w", err)
		}
	}

	// Create NetworkPolicy.
	if resources.NetworkPolicy != nil {
		if err := client.CreateNetworkPolicy(ctx, ns, resources.NetworkPolicy); err != nil {
			return fmt.Errorf("creating NetworkPolicy: %w", err)
		}
	}

	// Create StatefulSet (last, depends on Service and Secret).
	if resources.StatefulSet != nil {
		if err := client.CreateStatefulSet(ctx, ns, resources.StatefulSet); err != nil {
			return fmt.Errorf("creating StatefulSet: %w", err)
		}
	}

	return nil
}

// cleanupOnFailure removes any partially created resources.
func (e *K8sDeployEnvironment) cleanupOnFailure(ctx context.Context, client *K8sClient, resources *K8sResourceSet, cred *CredentialResult) {
	ns := e.Namespace
	resName := k8sResourceName(e.name)

	if resources.StatefulSet != nil {
		if err := client.DeleteStatefulSet(ctx, ns, resName); err != nil {
			log.Printf("WARNING: cleanup: deleting StatefulSet: %v", err)
		}
	}
	if resources.Service != nil {
		if err := client.DeleteService(ctx, ns, resName); err != nil {
			log.Printf("WARNING: cleanup: deleting Service: %v", err)
		}
	}
	if resources.ConfigMap != nil {
		if err := client.DeleteConfigMap(ctx, ns, resName); err != nil {
			log.Printf("WARNING: cleanup: deleting ConfigMap: %v", err)
		}
	}
	if resources.NetworkPolicy != nil {
		if err := client.DeleteNetworkPolicy(ctx, ns, resName); err != nil {
			log.Printf("WARNING: cleanup: deleting NetworkPolicy: %v", err)
		}
	}
	if cred != nil && cred.Secret != nil {
		if err := client.DeleteSecret(ctx, ns, resName+"-creds"); err != nil {
			log.Printf("WARNING: cleanup: deleting credential Secret: %v", err)
		}
	}
	if cred != nil && cred.ExternalSecret != nil {
		if err := client.DeleteExternalSecret(ctx, ns, resName+"-eso"); err != nil {
			log.Printf("WARNING: cleanup: deleting ExternalSecret: %v", err)
		}
	}
}

// applyOpenShiftResources detects OpenShift and creates platform-specific resources.
func (e *K8sDeployEnvironment) applyOpenShiftResources(ctx context.Context, client *K8sClient) error {
	caps, err := DetectOpenShift(client)
	if err != nil || (!caps.HasRoutes && !caps.HasEgressFirewall) {
		return nil
	}

	resName := k8sResourceName(e.name)
	labels := k8sStandardLabels(e.name)

	if caps.HasRoutes {
		route := GenerateRoute(e.name, e.Namespace, resName, labels)
		if err := client.CreateRoute(ctx, e.Namespace, route); err != nil {
			return fmt.Errorf("creating Route: %w", err)
		}
	}

	if caps.HasEgressFirewall && !e.NoNetworkPolicy {
		domains, _ := e.resolveDomains()
		fw := GenerateEgressFirewall(e.name, e.Namespace, labels, domains)
		if err := client.CreateEgressFirewall(ctx, e.Namespace, fw); err != nil {
			return fmt.Errorf("creating EgressFirewall: %w", err)
		}
	}

	return nil
}

// resolveDomains expands domain groups and literals into a flat list.
func (e *K8sDeployEnvironment) resolveDomains() ([]string, error) {
	allNames := append(e.AllowDomains, e.AllowGroups...)
	if len(allNames) == 0 {
		return nil, nil
	}

	resolver, err := newDomainResolver()
	if err != nil {
		return nil, err
	}

	return resolver.ExpandAll(allNames)
}

// loadMCPSidecars reads the build manifest and extracts MCP sidecar definitions.
func (e *K8sDeployEnvironment) loadMCPSidecars() ([]MCPSidecarOpts, error) {
	manifestPath := e.BuildDir + "/cc-deck-image.yaml"
	manifest, err := loadBuildManifest(manifestPath)
	if err != nil {
		return nil, err
	}

	var sidecars []MCPSidecarOpts
	for _, mcp := range manifest.MCP {
		if mcp.Image == "" {
			continue
		}
		sidecars = append(sidecars, MCPSidecarOpts{
			Name:    mcp.Name,
			Image:   mcp.Image,
			Port:    mcp.Port,
			EnvVars: mcp.Auth.EnvVars,
		})
	}
	return sidecars, nil
}

// resolveNamespace returns the namespace from the instance or the environment config.
func (e *K8sDeployEnvironment) resolveNamespace(inst *EnvironmentInstance) string {
	if inst.K8s != nil && inst.K8s.Namespace != "" {
		return inst.K8s.Namespace
	}
	if e.Namespace != "" {
		return e.Namespace
	}
	return "default"
}

// resolveKubeconfig returns the kubeconfig path from the instance or the environment config.
func (e *K8sDeployEnvironment) resolveKubeconfig(inst *EnvironmentInstance) string {
	if inst.K8s != nil && inst.K8s.Kubeconfig != "" {
		return inst.K8s.Kubeconfig
	}
	return e.Kubeconfig
}

// resolveContext returns the context name from the instance or the environment config.
func (e *K8sDeployEnvironment) resolveContext(inst *EnvironmentInstance) string {
	if inst.K8s != nil && inst.K8s.Profile != "" {
		return inst.K8s.Profile
	}
	return e.Context
}

// kubeconfigArgs returns kubectl arguments for kubeconfig and context.
func (e *K8sDeployEnvironment) kubeconfigArgs(inst *EnvironmentInstance) []string {
	var args []string
	kc := e.resolveKubeconfig(inst)
	if kc != "" {
		args = append(args, "--kubeconfig", kc)
	}
	ctx := e.resolveContext(inst)
	if ctx != "" {
		args = append(args, "--context", ctx)
	}
	return args
}

// k8sStandardLabels returns the standard label set for all K8s resources.
func k8sStandardLabels(envName string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "cc-deck",
		"app.kubernetes.io/instance":   envName,
		"app.kubernetes.io/managed-by": "cc-deck",
		"app.kubernetes.io/component":  "workspace",
	}
}

// k8sHasZellijSession checks whether a Zellij session is running in the Pod.
func k8sHasZellijSession(ctx context.Context, ns, podName string, kubeconfigArgs []string) bool {
	args := append(kubeconfigArgs, "exec", "-n", ns, podName, "--", "zellij", "list-sessions", "-n")
	cmd := exec.CommandContext(ctx, "kubectl", args...)
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.Contains(line, "(EXITED") {
			return true
		}
	}
	return false
}

// k8sExec runs a command inside the K8s Pod.
func k8sExec(ctx context.Context, ns, podName string, kubeconfigArgs, cmd []string, interactive bool) error {
	args := append(kubeconfigArgs, "exec")
	if interactive {
		args = append(args, "-it")
	}
	args = append(args, "-n", ns, podName, "--")
	args = append(args, cmd...)

	c := exec.CommandContext(ctx, "kubectl", args...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

// ReconcileK8sDeployEnvs updates the state of all k8s-deploy instances
// by checking their actual K8s API state.
func ReconcileK8sDeployEnvs(store *FileStateStore) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	instances, err := store.ListInstances(nil)
	if err != nil {
		return err
	}

	for _, inst := range instances {
		if inst.Type != EnvironmentTypeK8sDeploy || inst.K8s == nil {
			continue
		}

		client, err := NewK8sClient(inst.K8s.Kubeconfig, inst.K8s.Profile)
		if err != nil {
			log.Printf("WARNING: reconcile %s: creating client: %v", inst.Name, err)
			continue
		}

		resName := k8sResourceName(inst.Name)
		newState, err := client.ReconcileState(ctx, inst.K8s.Namespace, resName)
		if err != nil {
			log.Printf("WARNING: reconcile %s: checking state: %v", inst.Name, err)
			continue
		}

		if inst.State != newState {
			inst.State = newState
			if err := store.UpdateInstance(inst); err != nil {
				return err
			}
		}
	}

	return nil
}
