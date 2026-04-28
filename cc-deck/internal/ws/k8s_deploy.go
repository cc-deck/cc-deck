package ws

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// K8sDeployWorkspace manages a Kubernetes-based persistent development
// workspace backed by a StatefulSet with PVC storage.
type K8sDeployWorkspace struct {
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

	// Repo cloning
	Repos           []RepoEntry
	ExtraRemotes    map[string]string
	AutoDetectedURL string

	pipeOnce sync.Once
	pipeCh   PipeChannel
	dataOnce sync.Once
	dataCh   DataChannel
	dataErr  error
	gitOnce  sync.Once
	gitCh    GitChannel
	gitErr   error
}

const (
	k8sResourcePrefix   = "cc-deck-"
	defaultStorageSize  = "10Gi"
	defaultPodTimeout   = 5 * time.Minute
	k8sWorkspacePath    = "/workspace"
	k8sCredentialPath   = "/run/secrets/cc-deck"
)

func k8sResourceName(wsName string) string {
	return k8sResourcePrefix + wsName
}

func k8sPodName(wsName string) string {
	return k8sResourceName(wsName) + "-0"
}

// Type returns WorkspaceTypeK8sDeploy.
func (e *K8sDeployWorkspace) Type() WorkspaceType {
	return WorkspaceTypeK8sDeploy
}

// Name returns the workspace name.
func (e *K8sDeployWorkspace) Name() string {
	return e.name
}

// Create provisions a new k8s-deploy workspace.
func (e *K8sDeployWorkspace) Create(ctx context.Context, opts CreateOpts) error {
	if err := ValidateWsName(e.name); err != nil {
		return err
	}

	// Check kubectl availability.
	if _, err := exec.LookPath("kubectl"); err != nil {
		return fmt.Errorf("kubectl not found in PATH: %w", ErrKubectlNotFound)
	}

	// Fail fast if ESO parameters are incomplete.
	if e.SecretStore != "" {
		if e.SecretStoreRef == "" {
			return fmt.Errorf("--secret-store-ref is required when --secret-store is specified")
		}
		if e.SecretPath == "" {
			return fmt.Errorf("--secret-path is required when --secret-store is specified")
		}
	}

	// Fail fast if a workspace with this name already exists.
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

	// Check ESO CRD availability (parameter validation done earlier).
	if e.SecretStore != "" && !client.HasAPIGroup("external-secrets.io/v1") {
		return fmt.Errorf("External Secrets Operator is not installed on this cluster (external-secrets.io API group not found)")
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

	// Clone repos into workspace if defined.
	if len(e.Repos) > 0 {
		creds := loadActiveGitCredentials()
		workspace := k8sWorkspacePath
		ns := e.Namespace
		podName := k8sPodName(e.name)
		kubeconfigArgs := e.kubeconfigArgs(&WorkspaceInstance{K8s: &K8sFields{Kubeconfig: e.Kubeconfig, Profile: e.Context}})
		k8sRunner := func(ctx2 context.Context, cmd string) (string, error) {
			return k8sExecOutput(ctx2, ns, podName, kubeconfigArgs, cmd)
		}
		fmt.Fprintf(os.Stderr, "Cloning %d repo(s) into %s...\n", len(e.Repos), workspace)
		cloneRepos(ctx, k8sRunner, e.Repos, workspace, creds, e.ExtraRemotes, e.AutoDetectedURL)
	}

	// Write workspace definition.
	if e.defs != nil {
		wsDef := &WorkspaceDefinition{
			Name: e.name,
			Type: WorkspaceTypeK8sDeploy,
			WorkspaceSpec: WorkspaceSpec{
				Image: image,
			},
		}
		if def, findErr := e.defs.FindByName(e.name); findErr == nil {
			_ = e.defs.Update(def)
		} else {
			_ = e.defs.Add(wsDef)
		}
	}

	// Write workspace instance to state store.
	resName := k8sResourceName(e.name)
	running := InfraStateRunning
	inst := &WorkspaceInstance{
		Name:         e.name,
		Type:         WorkspaceTypeK8sDeploy,
		InfraState:   &running,
		SessionState: SessionStateNone,
		CreatedAt:    time.Now().UTC(),
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
func (e *K8sDeployWorkspace) Attach(ctx context.Context) error {
	// Nested Zellij detection.
	if os.Getenv("ZELLIJ") != "" {
		fmt.Fprintf(os.Stderr, "Already inside Zellij. Detach first (Ctrl+o d), then run:\n")
		fmt.Fprintf(os.Stderr, "  cc-deck ws attach %s\n", e.name)
		return nil
	}

	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil {
		return err
	}

	// Auto-start if stopped.
	if inst.InfraState != nil && *inst.InfraState == InfraStateStopped {
		if startErr := e.Start(ctx); startErr != nil {
			return fmt.Errorf("auto-starting workspace: %w", startErr)
		}
		inst, err = e.store.FindInstanceByName(e.name)
		if err != nil {
			return err
		}
	}

	// Update LastAttached timestamp and session state.
	now := time.Now().UTC()
	inst.LastAttached = &now
	inst.SessionState = SessionStateExists
	_ = e.store.UpdateInstance(inst)

	ns := e.resolveNamespace(inst)
	podName := k8sPodName(e.name)
	kubeconfigArgs := e.kubeconfigArgs(inst)

	// Set terminal background color for remote sessions if configured.
	remoteBG := LoadRemoteBG(e.name, e.defs)
	if remoteBG != "" {
		SetRemoteBG(remoteBG)
	}

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
	err = cmd.Run()

	if remoteBG != "" {
		fmt.Fprint(os.Stdout, ResetBGEscape)
	}
	return err
}

// KillSession kills the Zellij session inside the K8s Pod without
// affecting the Pod or StatefulSet.
func (e *K8sDeployWorkspace) KillSession(ctx context.Context) error {
	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil {
		return err
	}
	ns := e.resolveNamespace(inst)
	podName := k8sPodName(e.name)
	kubeconfigArgs := e.kubeconfigArgs(inst)
	if !k8sHasZellijSession(ctx, ns, podName, kubeconfigArgs) {
		return nil
	}
	sessionName := zellijSessionPrefix + e.name
	cmd := []string{"zellij", "delete-session", "--force", sessionName}
	if err := k8sExec(ctx, ns, podName, kubeconfigArgs, cmd, false); err != nil {
		return fmt.Errorf("deleting session: %w", err)
	}
	inst.SessionState = SessionStateNone
	_ = e.store.UpdateInstance(inst)
	return nil
}

// Start scales the StatefulSet to 1 replica and waits for Pod readiness.
func (e *K8sDeployWorkspace) Start(ctx context.Context) error {
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

	running := InfraStateRunning
	inst.InfraState = &running
	return e.store.UpdateInstance(inst)
}

// Stop kills the session and scales the StatefulSet to 0, preserving the PVC.
func (e *K8sDeployWorkspace) Stop(ctx context.Context) error {
	_ = e.KillSession(ctx)

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

	stopped := InfraStateStopped
	inst.InfraState = &stopped
	inst.SessionState = SessionStateNone
	return e.store.UpdateInstance(inst)
}

// Delete removes all K8s resources and state records for the workspace.
func (e *K8sDeployWorkspace) Delete(ctx context.Context, force bool) error {
	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil {
		return err
	}

	if !force && inst.InfraState != nil && *inst.InfraState == InfraStateRunning {
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

// Status returns the current state and metadata for the workspace.
func (e *K8sDeployWorkspace) Status(ctx context.Context) (*WorkspaceStatus, error) {
	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil {
		return nil, err
	}

	var infraState InfraStateValue = InfraStateError

	// Reconcile with actual K8s state.
	client, clientErr := NewK8sClient(e.resolveKubeconfig(inst), e.resolveContext(inst))
	if clientErr == nil {
		ns := e.resolveNamespace(inst)
		resName := k8sResourceName(e.name)
		if reconciled, reconcileErr := client.ReconcileState(ctx, ns, resName); reconcileErr == nil {
			switch reconciled {
			case WorkspaceStateRunning:
				infraState = InfraStateRunning
			case WorkspaceStateStopped:
				infraState = InfraStateStopped
			default:
				infraState = InfraStateError
			}
		}
	}

	var sessState SessionStateValue = SessionStateNone
	if infraState == InfraStateRunning {
		ns := e.resolveNamespace(inst)
		podName := k8sPodName(e.name)
		kubeconfigArgs := e.kubeconfigArgs(inst)
		if k8sHasZellijSession(ctx, ns, podName, kubeconfigArgs) {
			sessState = SessionStateExists
		}
	}

	return &WorkspaceStatus{
		InfraState:   &infraState,
		SessionState: sessState,
		Since:        &inst.CreatedAt,
	}, nil
}

// Exec runs a command inside the K8s Pod.
func (e *K8sDeployWorkspace) Exec(ctx context.Context, cmd []string) error {
	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil {
		return err
	}
	if inst.InfraState == nil || *inst.InfraState != InfraStateRunning {
		return fmt.Errorf("workspace is not running (infra: %s); start it with: cc-deck ws start %s", InfraStateString(inst.InfraState), e.name)
	}

	return k8sExec(ctx, e.resolveNamespace(inst), k8sPodName(e.name), e.kubeconfigArgs(inst), cmd, false)
}

// ExecOutput runs a command inside the K8s Pod and returns stdout.
func (e *K8sDeployWorkspace) ExecOutput(ctx context.Context, cmd []string) (string, error) {
	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil {
		return "", err
	}
	return k8sExecOutput(ctx, e.resolveNamespace(inst), k8sPodName(e.name), e.kubeconfigArgs(inst), shellJoin(cmd))
}

// PipeChannel returns the pipe channel for this workspace.
func (e *K8sDeployWorkspace) PipeChannel(_ context.Context) (PipeChannel, error) {
	e.pipeOnce.Do(func() {
		e.pipeCh = &execPipeChannel{name: e.name, execFn: e.Exec, execOutputFn: e.ExecOutput}
	})
	return e.pipeCh, nil
}

// DataChannel returns the data channel for this workspace.
func (e *K8sDeployWorkspace) DataChannel(_ context.Context) (DataChannel, error) {
	e.dataOnce.Do(func() {
		inst, err := e.store.FindInstanceByName(e.name)
		if err != nil {
			e.dataErr = err
			return
		}
		e.dataCh = &k8sDataChannel{
			name:           e.name,
			ns:             e.resolveNamespace(inst),
			podName:        k8sPodName(e.name),
			kubeconfigArgs: append([]string(nil), e.kubeconfigArgs(inst)...),
		}
	})
	return e.dataCh, e.dataErr
}

// GitChannel returns the git channel for this workspace.
func (e *K8sDeployWorkspace) GitChannel(_ context.Context) (GitChannel, error) {
	e.gitOnce.Do(func() {
		inst, err := e.store.FindInstanceByName(e.name)
		if err != nil {
			e.gitErr = err
			return
		}
		e.gitCh = &k8sGitChannel{
			name:           e.name,
			ns:             e.resolveNamespace(inst),
			podName:        k8sPodName(e.name),
			kubeconfigArgs: append([]string(nil), e.kubeconfigArgs(inst)...),
			workspacePath:  k8sWorkspacePath,
		}
	})
	return e.gitCh, e.gitErr
}

// Push synchronizes local files into the K8s Pod via DataChannel.
func (e *K8sDeployWorkspace) Push(ctx context.Context, opts SyncOpts) error {
	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil {
		return err
	}
	if inst.InfraState == nil || *inst.InfraState != InfraStateRunning {
		return fmt.Errorf("workspace is not running (infra: %s); start it with: cc-deck ws start %s", InfraStateString(inst.InfraState), e.name)
	}

	ch, chErr := e.DataChannel(ctx)
	if chErr != nil {
		return chErr
	}
	return ch.Push(ctx, opts)
}

// Pull synchronizes files from the K8s Pod to local storage via DataChannel.
func (e *K8sDeployWorkspace) Pull(ctx context.Context, opts SyncOpts) error {
	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil {
		return err
	}
	if inst.InfraState == nil || *inst.InfraState != InfraStateRunning {
		return fmt.Errorf("workspace is not running (infra: %s); start it with: cc-deck ws start %s", InfraStateString(inst.InfraState), e.name)
	}

	ch, chErr := e.DataChannel(ctx)
	if chErr != nil {
		return chErr
	}
	return ch.Pull(ctx, opts)
}

// Harvest extracts git commits from the K8s Pod via GitChannel.
func (e *K8sDeployWorkspace) Harvest(ctx context.Context, opts HarvestOpts) error {
	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil {
		return err
	}
	if inst.InfraState == nil || *inst.InfraState != InfraStateRunning {
		return fmt.Errorf("workspace is not running (infra: %s); start it with: cc-deck ws start %s", InfraStateString(inst.InfraState), e.name)
	}

	ch, chErr := e.GitChannel(ctx)
	if chErr != nil {
		return chErr
	}
	return ch.Fetch(ctx, opts)
}

// applyResources creates all generated K8s resources on the cluster.
func (e *K8sDeployWorkspace) applyResources(ctx context.Context, client *K8sClient, resources *K8sResourceSet, cred *CredentialResult) error {
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
func (e *K8sDeployWorkspace) cleanupOnFailure(ctx context.Context, client *K8sClient, resources *K8sResourceSet, cred *CredentialResult) {
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
func (e *K8sDeployWorkspace) applyOpenShiftResources(ctx context.Context, client *K8sClient) error {
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
func (e *K8sDeployWorkspace) resolveDomains() ([]string, error) {
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
func (e *K8sDeployWorkspace) loadMCPSidecars() ([]MCPSidecarOpts, error) {
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

// resolveNamespace returns the namespace from the instance or the workspace config.
func (e *K8sDeployWorkspace) resolveNamespace(inst *WorkspaceInstance) string {
	if inst.K8s != nil && inst.K8s.Namespace != "" {
		return inst.K8s.Namespace
	}
	if e.Namespace != "" {
		return e.Namespace
	}
	return "default"
}

// resolveKubeconfig returns the kubeconfig path from the instance or the workspace config.
func (e *K8sDeployWorkspace) resolveKubeconfig(inst *WorkspaceInstance) string {
	if inst.K8s != nil && inst.K8s.Kubeconfig != "" {
		return inst.K8s.Kubeconfig
	}
	return e.Kubeconfig
}

// resolveContext returns the context name from the instance or the workspace config.
func (e *K8sDeployWorkspace) resolveContext(inst *WorkspaceInstance) string {
	if inst.K8s != nil && inst.K8s.Profile != "" {
		return inst.K8s.Profile
	}
	return e.Context
}

// kubeconfigArgs returns kubectl arguments for kubeconfig and context.
func (e *K8sDeployWorkspace) kubeconfigArgs(inst *WorkspaceInstance) []string {
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
func k8sStandardLabels(wsName string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "cc-deck",
		"app.kubernetes.io/instance":   wsName,
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

// k8sExecOutput runs a command inside the K8s Pod and returns stdout.
func k8sExecOutput(ctx context.Context, ns, podName string, kubeconfigArgs []string, shellCmd string) (string, error) {
	args := append(kubeconfigArgs, "exec", "-n", ns, podName, "--", "sh", "-c", shellCmd)
	c := exec.CommandContext(ctx, "kubectl", args...)
	out, err := c.Output()
	if err != nil {
		return "", fmt.Errorf("kubectl exec: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// ReconcileK8sDeployWorkspaces updates the state of all k8s-deploy instances
// by checking their actual K8s API state.
func ReconcileK8sDeployWorkspaces(store *FileStateStore) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	instances, err := store.ListInstances(nil)
	if err != nil {
		return err
	}

	for _, inst := range instances {
		if inst.Type != WorkspaceTypeK8sDeploy || inst.K8s == nil {
			continue
		}

		client, err := NewK8sClient(inst.K8s.Kubeconfig, inst.K8s.Profile)
		if err != nil {
			log.Printf("WARNING: reconcile %s: creating client: %v", inst.Name, err)
			continue
		}

		resName := k8sResourceName(inst.Name)
		reconciled, reconcileErr := client.ReconcileState(ctx, inst.K8s.Namespace, resName)
		if reconcileErr != nil {
			log.Printf("WARNING: reconcile %s: checking state: %v", inst.Name, reconcileErr)
			continue
		}

		var newInfra InfraStateValue
		switch reconciled {
		case WorkspaceStateRunning:
			newInfra = InfraStateRunning
		case WorkspaceStateStopped:
			newInfra = InfraStateStopped
		default:
			newInfra = InfraStateError
		}

		var newSess SessionStateValue = SessionStateNone
		if newInfra == InfraStateRunning {
			podName := k8sPodName(inst.Name)
			var kubeconfigArgs []string
			if inst.K8s.Kubeconfig != "" {
				kubeconfigArgs = append(kubeconfigArgs, "--kubeconfig", inst.K8s.Kubeconfig)
			}
			if inst.K8s.Profile != "" {
				kubeconfigArgs = append(kubeconfigArgs, "--context", inst.K8s.Profile)
			}
			if k8sHasZellijSession(ctx, inst.K8s.Namespace, podName, kubeconfigArgs) {
				newSess = SessionStateExists
			}
		}

		changed := inst.InfraState == nil || *inst.InfraState != newInfra || inst.SessionState != newSess
		if changed {
			inst.InfraState = &newInfra
			inst.SessionState = newSess
			if err := store.UpdateInstance(inst); err != nil {
				return err
			}
		}
	}

	return nil
}
