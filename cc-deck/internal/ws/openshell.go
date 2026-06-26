package ws

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/cc-deck/cc-deck/internal/build"
	"github.com/cc-deck/cc-deck/internal/oci"
	"github.com/cc-deck/cc-deck/internal/openshell"
)

const (
	defaultSandboxImage   = "cc-deck/openshell-sandbox:latest"
	defaultSandboxCommand = "zellij"
	createPollInterval    = 2 * time.Second
	createTimeout         = 60 * time.Second
)

// SandboxConfig holds sandbox provisioning parameters.
type SandboxConfig struct {
	Image     string
	Command   string
	Policy    string
	Providers []string
}

// attachState tracks an active interactive attach session.
type attachState struct {
	pid int
}

func (a *attachState) isAlive() bool {
	if a == nil || a.pid == 0 {
		return false
	}
	proc, err := os.FindProcess(a.pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

// OpenShellWorkspace manages a workspace backed by an OpenShell sandbox.
type OpenShellWorkspace struct {
	name      string
	store     *FileStateStore
	defs      *DefinitionStore
	client    openshell.Client
	sandboxID string
	attach    *attachState

	Repos           []RepoEntry
	ExtraRemotes    map[string]string
	AutoDetectedURL string

	clientOnce sync.Once
	clientErr  error

	pipeOnce sync.Once
	pipeCh   PipeChannel
	dataOnce sync.Once
	dataCh   DataChannel
	gitOnce  sync.Once
	gitCh    GitChannel
}

func (w *OpenShellWorkspace) Type() WorkspaceType { return WorkspaceTypeOpenShell }
func (w *OpenShellWorkspace) Name() string        { return w.name }

// ensureClient lazily initializes the CLI client from workspace definition.
func (w *OpenShellWorkspace) ensureClient() error {
	w.clientOnce.Do(func() {
		gwCfg := w.resolveGatewayConfig()
		w.client, w.clientErr = openshell.NewClient(gwCfg)
		if w.clientErr != nil {
			w.clientErr = fmt.Errorf("connecting to OpenShell gateway: %w", w.clientErr)
		}
	})
	return w.clientErr
}

func (w *OpenShellWorkspace) resolveGatewayConfig() openshell.GatewayConfig {
	var defGw *openshell.GatewayConfig
	if w.defs != nil {
		if def, err := w.defs.FindByName(w.name); err == nil && def.Gateway != "" {
			cfg := openshell.GatewayConfig{Address: def.Gateway}
			if def.GatewayTLS != nil {
				cfg.TLS = *def.GatewayTLS
			}
			cfg.TLSCertPath = def.TLSCertPath
			cfg.TLSKeyPath = def.TLSKeyPath
			cfg.TLSCAPath = def.TLSCAPath
			defGw = &cfg
		}
	}
	return openshell.ResolveGatewayConfig(defGw)
}

// policyFilePath is the well-known location of the policy file inside
// OpenShell sandbox images.
const policyFilePath = "/etc/openshell/policy.yaml"

func (w *OpenShellWorkspace) resolveSandboxConfig() (SandboxConfig, error) {
	cfg := SandboxConfig{
		Image:   defaultSandboxImage,
		Command: defaultSandboxCommand,
	}
	if w.defs == nil {
		return cfg, nil
	}
	def, err := w.defs.FindByName(w.name)
	if err != nil || def == nil {
		return cfg, nil
	}
	if def.SandboxImage != "" {
		cfg.Image = def.SandboxImage
	}
	if def.SandboxCommand != "" {
		cfg.Command = def.SandboxCommand
	}
	cfg.Policy = def.Policy
	if def.Provider != "" {
		cfg.Providers = []string{def.Provider}
	}

	// If no explicit policy is set and we have an image reference, attempt
	// to extract the policy file directly from the OCI image.
	if cfg.Policy == "" && cfg.Image != "" {
		policyBytes, extractErr := oci.ExtractFileFromImage(cfg.Image, policyFilePath)
		if extractErr != nil {
			return cfg, fmt.Errorf("extracting policy from image %s: %w\n\nTo provide the policy file manually, use the --policy flag", cfg.Image, extractErr)
		}

		tmpFile, tmpErr := os.CreateTemp("", "cc-deck-policy-*.yaml")
		if tmpErr != nil {
			return cfg, fmt.Errorf("creating temp file for extracted policy: %w", tmpErr)
		}
		if _, writeErr := tmpFile.Write(policyBytes); writeErr != nil {
			tmpFile.Close()
			os.Remove(tmpFile.Name())
			return cfg, fmt.Errorf("writing extracted policy to temp file: %w", writeErr)
		}
		tmpFile.Close()
		cfg.Policy = tmpFile.Name()
		log.Printf("INFO: oci: extracted policy to %s", tmpFile.Name())
	}

	return cfg, nil
}

// loadSandboxID restores the sandbox ID from the state store.
func (w *OpenShellWorkspace) loadSandboxID() {
	if w.sandboxID != "" {
		return
	}
	if w.store == nil {
		return
	}
	inst, err := w.store.FindInstanceByName(w.name)
	if err == nil && inst != nil && inst.OpenShell != nil {
		w.sandboxID = inst.OpenShell.SandboxID
	}
}

// Status returns the current workspace state by querying the gateway.
func (w *OpenShellWorkspace) Status(ctx context.Context) (*WorkspaceStatus, error) {
	w.loadSandboxID()
	if w.sandboxID == "" {
		return &WorkspaceStatus{SessionState: SessionStateNone}, nil
	}
	if err := w.ensureClient(); err != nil {
		return nil, err
	}

	info, err := w.client.GetSandbox(ctx, w.sandboxID)
	if err != nil {
		return nil, fmt.Errorf("querying sandbox status: %w", err)
	}

	status := &WorkspaceStatus{SessionState: SessionStateNone}

	switch info.State {
	case openshell.SandboxStateRunning:
		running := InfraStateRunning
		status.InfraState = &running
	case openshell.SandboxStateSuspended:
		stopped := InfraStateStopped
		status.InfraState = &stopped
	case openshell.SandboxStateError:
		errState := InfraStateError
		status.InfraState = &errState
	case openshell.SandboxStateDeleted:
		w.clearLocalState()
		return &WorkspaceStatus{SessionState: SessionStateNone, Message: "sandbox deleted"}, nil
	case openshell.SandboxStateCreating:
		running := InfraStateRunning
		status.InfraState = &running
		status.Message = "sandbox is starting"
	}

	if w.attach != nil && w.attach.isAlive() {
		status.SessionState = SessionStateExists
	}

	return status, nil
}

func (w *OpenShellWorkspace) clearLocalState() {
	if w.store == nil {
		return
	}
	_ = w.store.RemoveInstance(w.name)
	w.sandboxID = ""
	w.attach = nil
	log.Printf("DEBUG: openshell: cleared local state for %s", w.name)
}

// loadManifestCredentials finds and loads credential entries from the project's
// build.yaml manifest. Returns nil if no manifest or no credentials section.
func (w *OpenShellWorkspace) loadManifestCredentials() []openshell.CredentialInput {
	if w.defs == nil {
		return nil
	}
	def, err := w.defs.FindByName(w.name)
	if err != nil || def == nil || def.ProjectDir == "" {
		return nil
	}

	// Walk up from ProjectDir to find .cc-deck/setup/build.yaml.
	dir := def.ProjectDir
	for {
		manifestPath := filepath.Join(dir, ".cc-deck", "setup", "build.yaml")
		if _, statErr := os.Stat(manifestPath); statErr == nil {
			m, loadErr := build.LoadManifest(manifestPath)
			if loadErr != nil {
				log.Printf("WARNING: failed to load manifest %s: %v", manifestPath, loadErr)
				return nil
			}
			if len(m.Credentials) == 0 {
				return nil
			}
			var inputs []openshell.CredentialInput
			for _, c := range m.Credentials {
				inputs = append(inputs, openshell.CredentialInput{
					Type:    c.Type,
					EnvVars: c.EnvVars,
					File:    c.File,
				})
			}
			return inputs
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return nil
}

// Create provisions a new OpenShell sandbox.
func (w *OpenShellWorkspace) Create(ctx context.Context, _ CreateOpts) error {
	if err := ValidateWsName(w.name); err != nil {
		return err
	}
	if err := w.ensureClient(); err != nil {
		return err
	}

	sbCfg, cfgErr := w.resolveSandboxConfig()
	if cfgErr != nil {
		return cfgErr
	}

	// Clean up any temp policy file extracted from the OCI image after
	// sandbox creation completes (success or failure).
	if sbCfg.Policy != "" {
		if _, statErr := os.Stat(sbCfg.Policy); statErr == nil {
			if matched, _ := filepath.Match("cc-deck-policy-*.yaml", filepath.Base(sbCfg.Policy)); matched {
				defer os.Remove(sbCfg.Policy)
			}
		}
	}

	// Resolve credentials from manifest and create providers.
	credInputs := w.loadManifestCredentials()
	providerConfigs := openshell.ResolveCredentials(credInputs, w.name)

	var credProviders []string
	for _, pc := range providerConfigs {
		if pc.SkipProvider {
			continue
		}
		if err := w.client.EnsureProvider(ctx, pc.Name, pc.Type, pc.FromExisting, pc.Credentials); err != nil {
			return fmt.Errorf("creating credential provider %s: %w", pc.Name, err)
		}
		credProviders = append(credProviders, pc.Name)
		log.Printf("DEBUG: openshell: created provider %s (type=%s)", pc.Name, pc.Type)
	}

	// Merge credential providers with any providers from the definition.
	allProviders := append(sbCfg.Providers, credProviders...)

	sandboxID, err := w.client.CreateSandbox(ctx, sbCfg.Image, sbCfg.Command, sbCfg.Policy, allProviders)
	if err != nil {
		return fmt.Errorf("creating sandbox: %w", err)
	}
	w.sandboxID = sandboxID

	if err := w.pollUntilRunning(ctx); err != nil {
		return err
	}

	// Handle post-start credential injection.
	for _, pc := range providerConfigs {
		if pc.FilePath != "" && pc.Type != "google-cloud" {
			remotePath := "/sandbox/.config/gcloud/credentials.json"
			if err := openshell.UploadFileCredential(ctx, w.client, w.sandboxID, pc.FilePath, remotePath, pc.FileVar); err != nil {
				log.Printf("WARNING: failed to upload file credential for %s: %v", pc.Type, err)
			}
		}
		if len(pc.EnvVarsToInject) > 0 {
			if err := openshell.InjectEnvVars(ctx, w.client, w.sandboxID, pc.EnvVarsToInject); err != nil {
				log.Printf("WARNING: failed to inject env vars for %s: %v", pc.Type, err)
			}
			log.Printf("DEBUG: openshell: injected %d env vars for %s", len(pc.EnvVarsToInject), pc.Type)
		}
	}

	if len(w.Repos) > 0 {
		creds := loadActiveGitCredentials()
		workspace := "/sandbox"
		runner := func(ctx2 context.Context, cmd string) (string, error) {
			return w.ExecOutput(ctx2, []string{"bash", "-c", cmd})
		}
		fmt.Fprintf(os.Stderr, "Cloning %d repo(s) into %s...\n", len(w.Repos), workspace)
		cloneRepos(ctx, runner, w.Repos, workspace, creds, w.ExtraRemotes, w.AutoDetectedURL, true)
	}

	now := time.Now()
	running := InfraStateRunning
	inst := WorkspaceInstance{
		Name:         w.name,
		Type:         WorkspaceTypeOpenShell,
		InfraState:   &running,
		SessionState: SessionStateNone,
		CreatedAt:    now,
		OpenShell: &OpenShellFields{
			SandboxID:   sandboxID,
			GatewayAddr: w.client.Address(),
		},
	}
	return w.store.AddInstance(&inst)
}

func (w *OpenShellWorkspace) pollUntilRunning(ctx context.Context) error {
	deadline := time.After(createTimeout)
	ticker := time.NewTicker(createPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			return fmt.Errorf("timeout waiting for sandbox %s to reach running state", w.sandboxID)
		case <-ticker.C:
			info, err := w.client.GetSandbox(ctx, w.sandboxID)
			if err != nil {
				log.Printf("DEBUG: openshell: poll error for %s: %v", w.sandboxID, err)
				continue
			}
			switch info.State {
			case openshell.SandboxStateRunning:
				return nil
			case openshell.SandboxStateError:
				return fmt.Errorf("sandbox %s entered error state", w.sandboxID)
			case openshell.SandboxStateDeleted:
				return fmt.Errorf("sandbox %s was deleted during creation", w.sandboxID)
			}
		}
	}
}

// Start provisions a new sandbox for this workspace (InfraManager).
func (w *OpenShellWorkspace) Start(ctx context.Context) error {
	return w.Create(ctx, CreateOpts{})
}

// Stop destroys the sandbox but preserves workspace definition (InfraManager).
func (w *OpenShellWorkspace) Stop(ctx context.Context) error {
	_ = w.KillSession(ctx)
	w.loadSandboxID()
	if w.sandboxID == "" {
		return nil
	}
	if err := w.ensureClient(); err != nil {
		return err
	}
	if err := w.client.DeleteSandbox(ctx, w.sandboxID); err != nil {
		return err
	}
	w.clearLocalState()
	return nil
}

// Delete removes the workspace and all resources.
func (w *OpenShellWorkspace) Delete(ctx context.Context, force bool) error {
	_ = w.KillSession(ctx)
	w.loadSandboxID()

	if w.sandboxID != "" {
		if err := w.ensureClient(); err != nil {
			if !force {
				return err
			}
			log.Printf("WARNING: gateway unreachable during delete, sandbox %s may be orphaned", w.sandboxID)
		} else {
			if err := w.client.DeleteSandbox(ctx, w.sandboxID); err != nil {
				if !force {
					return err
				}
				log.Printf("WARNING: failed to delete sandbox %s: %v (continuing with force)", w.sandboxID, err)
			}
		}
	}

	w.clearLocalState()
	return nil
}

// KillSession kills the Zellij session inside the sandbox without destroying it.
func (w *OpenShellWorkspace) KillSession(ctx context.Context) error {
	w.loadSandboxID()
	if w.sandboxID == "" {
		return nil
	}
	if err := w.ensureClient(); err != nil {
		return err
	}
	_, err := w.client.ExecSandbox(ctx, w.sandboxID, []string{"zellij", "kill-all-sessions"})
	if err != nil {
		log.Printf("DEBUG: openshell: kill-session best-effort failed: %v", err)
	}
	w.attach = nil
	return nil
}

// Attach connects to the sandbox interactively and attaches to Zellij.
func (w *OpenShellWorkspace) Attach(ctx context.Context) error {
	if w.attach != nil && w.attach.isAlive() {
		return fmt.Errorf("workspace %s is already attached (pid %d)", w.name, w.attach.pid)
	}
	if w.attach != nil {
		log.Printf("DEBUG: openshell: clearing stale attach for %s (pid %d dead)", w.name, w.attach.pid)
		w.attach = nil
	}

	w.loadSandboxID()
	if w.sandboxID == "" {
		return fmt.Errorf("workspace %s has no sandbox; create it first", w.name)
	}
	if err := w.ensureClient(); err != nil {
		return err
	}

	err := w.client.AttachExec(ctx, w.sandboxID, nil)

	now := time.Now()
	if inst, loadErr := w.store.FindInstanceByName(w.name); loadErr == nil && inst != nil {
		inst.LastAttached = &now
		_ = w.store.UpdateInstance(inst)
	}
	return err
}

// Exec runs a command inside the sandbox.
func (w *OpenShellWorkspace) Exec(ctx context.Context, cmd []string) error {
	w.loadSandboxID()
	if w.sandboxID == "" {
		return fmt.Errorf("workspace %s has no sandbox", w.name)
	}
	if err := w.ensureClient(); err != nil {
		return err
	}
	return w.client.ExecSandboxStream(ctx, w.sandboxID, cmd)
}

// ExecOutput runs a command inside the sandbox and returns stdout.
func (w *OpenShellWorkspace) ExecOutput(ctx context.Context, cmd []string) (string, error) {
	w.loadSandboxID()
	if w.sandboxID == "" {
		return "", fmt.Errorf("workspace %s has no sandbox", w.name)
	}
	if err := w.ensureClient(); err != nil {
		return "", err
	}
	result, err := w.client.ExecSandbox(ctx, w.sandboxID, cmd)
	if err != nil {
		return "", err
	}
	if result.ExitCode != 0 {
		return result.Stdout, fmt.Errorf("command exited with code %d: %s", result.ExitCode, result.Stderr)
	}
	return result.Stdout, nil
}

// Push synchronizes local files into the sandbox.
func (w *OpenShellWorkspace) Push(ctx context.Context, opts SyncOpts) error {
	ch, err := w.DataChannel(ctx)
	if err != nil {
		return err
	}
	return ch.Push(ctx, opts)
}

// Pull synchronizes files from the sandbox to local storage.
func (w *OpenShellWorkspace) Pull(ctx context.Context, opts SyncOpts) error {
	ch, err := w.DataChannel(ctx)
	if err != nil {
		return err
	}
	return ch.Pull(ctx, opts)
}

// Harvest extracts git commits from the sandbox.
func (w *OpenShellWorkspace) Harvest(ctx context.Context, opts HarvestOpts) error {
	ch, err := w.GitChannel(ctx)
	if err != nil {
		return err
	}
	return ch.Fetch(ctx, opts)
}

// PipeChannel returns the pipe channel (lazy init).
func (w *OpenShellWorkspace) PipeChannel(_ context.Context) (PipeChannel, error) {
	w.pipeOnce.Do(func() {
		w.pipeCh = &execPipeChannel{name: w.name, execFn: w.Exec, execOutputFn: w.ExecOutput}
	})
	return w.pipeCh, nil
}

// DataChannel returns the data channel (lazy init).
func (w *OpenShellWorkspace) DataChannel(_ context.Context) (DataChannel, error) {
	w.dataOnce.Do(func() {
		w.dataCh = &openShellDataChannel{ws: w}
	})
	return w.dataCh, nil
}

// GitChannel returns the git channel (lazy init).
func (w *OpenShellWorkspace) GitChannel(_ context.Context) (GitChannel, error) {
	w.gitOnce.Do(func() {
		w.gitCh = &openShellGitChannel{ws: w}
	})
	return w.gitCh, nil
}
