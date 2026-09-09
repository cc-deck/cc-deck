package ws

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cc-deck/cc-deck/internal/agent"
	"github.com/cc-deck/cc-deck/internal/build"
	"github.com/cc-deck/cc-deck/internal/config"
	"github.com/cc-deck/cc-deck/internal/credential"
	"github.com/cc-deck/cc-deck/internal/oci"
	"github.com/cc-deck/cc-deck/internal/openshell"
	"github.com/cc-deck/cc-deck/internal/profile"
	v1 "github.com/rhuss/openshell-sdk-go/openshell/v1"
	"github.com/rhuss/openshell-sdk-go/openshell/v1/types"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

const (
	defaultSandboxImage   = "cc-deck/openshell-sandbox:latest"
	defaultSandboxCommand = "zellij"
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
	active bool
	cancel context.CancelFunc
}

func (a *attachState) isAlive() bool {
	return a != nil && a.active
}

// OpenShellWorkspace manages a workspace backed by an OpenShell sandbox.
type OpenShellWorkspace struct {
	name        string
	store       *FileStateStore
	defs        *DefinitionStore
	client      v1.ClientInterface
	gatewayAddr string
	sandboxID   string
	attach      *attachState

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

// ensureClient lazily initializes the SDK client from workspace definition.
func (w *OpenShellWorkspace) ensureClient() error {
	w.clientOnce.Do(func() {
		gwCfg := w.resolveGatewayConfig()
		w.gatewayAddr = gwCfg.Address
		w.client, w.clientErr = openshell.NewSDKClient(gwCfg)
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
			log.Printf("WARNING: could not extract policy from image %s: %v", cfg.Image, extractErr)
			return cfg, nil
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

// resolveSandboxCommand returns the configured sandbox command without
// triggering OCI image extraction (unlike resolveSandboxConfig).
func (w *OpenShellWorkspace) resolveSandboxCommand() string {
	if w.defs != nil {
		if def, err := w.defs.FindByName(w.name); err == nil && def != nil && def.SandboxCommand != "" {
			return def.SandboxCommand
		}
	}
	return defaultSandboxCommand
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

	sb, err := w.client.Sandboxes().Get(ctx, w.sandboxID)
	if err != nil {
		if v1.IsNotFound(err) {
			w.clearLocalState()
			return &WorkspaceStatus{SessionState: SessionStateNone, Message: "sandbox deleted"}, nil
		}
		return nil, fmt.Errorf("querying sandbox status: %w", err)
	}

	status := &WorkspaceStatus{SessionState: SessionStateNone}

	switch sb.Status.Phase {
	case types.SandboxReady:
		running := InfraStateRunning
		status.InfraState = &running
	case types.SandboxUnknown:
		stopped := InfraStateStopped
		status.InfraState = &stopped
	case types.SandboxError:
		errState := InfraStateError
		status.InfraState = &errState
	case types.SandboxDeleting:
		w.clearLocalState()
		return &WorkspaceStatus{SessionState: SessionStateNone, Message: "sandbox deleted"}, nil
	case types.SandboxProvisioning:
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

// resolveAgentName returns the agent name from a workspace definition,
// defaulting to "claude" when unset.
func resolveAgentName(def *WorkspaceDefinition) string {
	return "claude"
}

// resolveAuthField returns the workspace definition's Auth field, or empty
// string if no definition is available.
func (w *OpenShellWorkspace) resolveAuthField() string {
	if w.defs == nil {
		return ""
	}
	def, err := w.defs.FindByName(w.name)
	if err != nil || def == nil {
		return ""
	}
	return def.Auth
}

// selectCredentialMode picks the credential spec to use from the available
// modes. If authField names a specific mode, that mode is selected. Otherwise
// the first available mode (highest priority) is used.
func selectCredentialMode(available []credential.AvailableMode, authField string) (agent.CredentialSpec, bool, error) {
	if len(available) == 0 {
		if authField != "" && authField != "none" && authField != "auto" {
			return agent.CredentialSpec{}, false, fmt.Errorf("auth mode %q was requested but no matching credentials were detected", authField)
		}
		return agent.CredentialSpec{}, false, nil
	}
	if authField == "none" {
		return agent.CredentialSpec{}, false, nil
	}
	if authField != "" && authField != "auto" {
		for _, m := range available {
			if m.Spec.Name == authField {
				return m.Spec, true, nil
			}
		}
		return agent.CredentialSpec{}, false, fmt.Errorf("auth mode %q was requested but no matching credentials were detected", authField)
	}
	return available[0].Spec, true, nil
}

// mapToOpenShellProvider maps a resolved credential spec to an OpenShell
// provider name, type, and credentials map. Returns empty providerType when
// no provider should be created (e.g., bedrock has no OpenShell provider).
func mapToOpenShellProvider(wsName string, spec agent.CredentialSpec, resolved credential.ResolvedCredentials) (name, providerType string, creds map[string]string) {
	name = fmt.Sprintf("cc-deck-%s-%s", wsName, spec.Name)

	switch spec.Name {
	case "api":
		providerType = "claude"
		creds = make(map[string]string)
		for k, v := range resolved.EnvVars {
			creds[k] = v
		}
	case "vertex":
		providerType = "google-cloud"
		creds = make(map[string]string)
		if v, ok := resolved.EnvVars["ANTHROPIC_VERTEX_PROJECT_ID"]; ok {
			creds["project_id"] = v
		}
		if v, ok := resolved.EnvVars["CLOUD_ML_REGION"]; ok {
			creds["region"] = v
		} else {
			creds["region"] = "global"
		}
	}

	return name, providerType, creds
}

// openShellTarget adapts an OpenShell sandbox for use as a profile.Target.
type openShellTarget struct {
	ws   *OpenShellWorkspace
	ctx  context.Context
	home string
}

func (t *openShellTarget) Upload(path string, content []byte, mode os.FileMode) error {
	// Write content to a temp file locally, then upload via the SDK.
	tmp, err := os.CreateTemp("", "cc-deck-provision-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()

	adapter := &openshell.OpenShellClientAdapter{Client: t.ws.client}
	if err := adapter.FileUpload(t.ctx, t.ws.sandboxID, tmp.Name(), path); err != nil {
		return err
	}
	// Set permissions.
	chmodCmd := fmt.Sprintf("chmod %04o %q", mode, path)
	return adapter.ExecRun(t.ctx, t.ws.sandboxID, []string{"bash", "-c", chmodCmd})
}

func (t *openShellTarget) Run(cmd string) (string, error) {
	return t.ws.ExecOutput(t.ctx, []string{"bash", "-c", cmd})
}

func (t *openShellTarget) Home() string { return t.home }

func (t *openShellTarget) Agents() []string {
	var agents []string
	for _, a := range agent.All() {
		if _, err := t.ws.ExecOutput(t.ctx, []string{"bash", "-c", fmt.Sprintf("command -v %s", a.Binary())}); err == nil {
			agents = append(agents, a.Name())
		}
	}
	return agents
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

	// Resolve credentials via agent-declared specs.
	var resolved credential.ResolvedCredentials
	var credProviders []string

	agentName := "claude"
	if w.defs != nil {
		if def, defErr := w.defs.FindByName(w.name); defErr == nil && def != nil {
			agentName = resolveAgentName(def)
		}
	}
	agentObj := agent.Get(agentName)
	if agentObj != nil {
		specs := agentObj.CredentialSpecs()
		available := credential.Detect(specs)
		selectedSpec, found, selectErr := selectCredentialMode(available, w.resolveAuthField())
		if selectErr != nil {
			return selectErr
		}
		if found {
			resolved = credential.Resolve(selectedSpec)
			providerName, providerType, providerCreds := mapToOpenShellProvider(w.name, selectedSpec, resolved)
			if providerType != "" {
				provider := &v1.Provider{
					Name: providerName,
					Type: providerType,
					Spec: types.ProviderSpec{
						Credentials: providerCreds,
					},
				}
				if _, err := w.client.Providers().Ensure(ctx, provider); err != nil {
					return fmt.Errorf("creating credential provider %s: %w", providerName, err)
				}
				credProviders = append(credProviders, providerName)
				log.Printf("DEBUG: openshell: created provider %s (type=%s)", providerName, providerType)
			}
		}
	}

	// Create per-profile credential providers (FR-026).
	if cfg, loadErr := config.Load(""); loadErr == nil && cfg != nil {
		seen := make(map[string]bool)
		for pName, p := range cfg.Profiles {
			harness := p.HarnessName()
			a := agent.Get(harness)
			if a == nil {
				continue
			}
			tr, ok := profile.Lookup(harness)
			if !ok {
				continue
			}
			backend := p.EffectiveBackend()
			provType := tr.ProviderType(backend)
			if provType == "" {
				continue
			}
			// Deduplicate: skip if this profile's provider type matches
			// the already-created single-spec provider.
			provName := fmt.Sprintf("cc-deck-%s-%s", w.name, pName)
			if seen[provName] {
				continue
			}
			seen[provName] = true

			creds, credErr := credential.ResolveProfile(pName, p)
			if credErr != nil {
				log.Printf("profile %s: credential resolution skipped: %v", pName, credErr)
				continue
			}
			provCreds := make(map[string]string)
			if creds != nil {
				for k, v := range creds.EnvVars {
					provCreds[k] = v
				}
			}
			provider := &v1.Provider{
				Name: provName,
				Type: provType,
				Spec: types.ProviderSpec{
					Credentials: provCreds,
				},
			}
			if _, err := w.client.Providers().Ensure(ctx, provider); err != nil {
				log.Printf("WARNING: failed to create profile provider %s: %v", provName, err)
				continue
			}
			credProviders = append(credProviders, provName)
			log.Printf("DEBUG: openshell: created profile provider %s (type=%s, profile=%s)", provName, provType, pName)
		}
	}

	// Merge credential providers with any providers from the definition.
	allProviders := append(sbCfg.Providers, credProviders...)

	sbSpec := &v1.SandboxSpec{
		Template: &v1.SandboxTemplate{
			Image: sbCfg.Image,
		},
		Providers: allProviders,
	}

	if sbCfg.Policy != "" {
		sdkPolicy, policyErr := loadSDKPolicy(sbCfg.Policy)
		if policyErr != nil {
			return fmt.Errorf("loading sandbox policy: %w", policyErr)
		}
		sbSpec.Policy = sdkPolicy
	}
	created, err := w.client.Sandboxes().Create(ctx, "", sbSpec, nil)
	if err != nil {
		return fmt.Errorf("creating sandbox: %w", err)
	}
	w.sandboxID = created.Name

	if _, err := w.client.Sandboxes().WaitReady(ctx, w.sandboxID); err != nil {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if delErr := w.client.Sandboxes().Delete(cleanupCtx, w.sandboxID); delErr != nil && !v1.IsNotFound(delErr) {
			log.Printf("WARNING: failed to clean up sandbox %s after readiness failure: %v", w.sandboxID, delErr)
		}
		w.sandboxID = ""
		return fmt.Errorf("waiting for sandbox to become ready: %w", err)
	}

	// Inject credentials into the sandbox via the shared credential package.
	if len(resolved.EnvVars) > 0 || resolved.FileCredential != nil || len(resolved.UnsetVars) > 0 {
		adapter := &openshell.OpenShellClientAdapter{Client: w.client}
		if err := credential.InjectOpenShell(ctx, adapter, w.sandboxID, resolved); err != nil {
			log.Printf("WARNING: failed to inject credentials: %v", err)
		}
	}

	// Provision profile wrappers and config dirs in the sandbox.
	if cfg, loadErr := config.Load(""); loadErr == nil && cfg != nil && len(cfg.Profiles) > 0 {
		target := &openShellTarget{ws: w, ctx: ctx, home: "/sandbox"}
		syncResult, provErr := profile.Provision(cfg, target)
		if provErr != nil {
			log.Printf("WARNING: profile provisioning failed: %v", provErr)
		} else {
			for _, skip := range syncResult.Skipped {
				log.Printf("profile %s skipped: %s", skip.Profile, skip.Reason)
			}
			for _, warn := range syncResult.Warnings {
				log.Printf("WARNING: %s", warn)
			}
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
			SandboxID:   w.sandboxID,
			GatewayAddr: w.gatewayAddr,
			Providers:   credProviders,
		},
	}
	return w.store.AddInstance(&inst)
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
	if err := w.client.Sandboxes().Delete(ctx, w.sandboxID); err != nil && !v1.IsNotFound(err) {
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
			if err := w.client.Sandboxes().Delete(ctx, w.sandboxID); err != nil && !v1.IsNotFound(err) {
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
	if w.attach != nil && w.attach.isAlive() && w.attach.cancel != nil {
		w.attach.cancel()
	}
	w.loadSandboxID()
	if w.sandboxID == "" {
		return nil
	}
	if err := w.ensureClient(); err != nil {
		return err
	}
	_, err := w.client.Exec().Run(ctx, w.sandboxID, []string{"zellij", "kill-all-sessions"})
	if err != nil {
		log.Printf("DEBUG: openshell: kill-session best-effort failed: %v", err)
	}
	w.attach = nil
	return nil
}

// Attach connects to the sandbox interactively, running the configured
// sandbox command (default: zellij) inside the sandbox via the SDK.
func (w *OpenShellWorkspace) Attach(ctx context.Context) error {
	if w.attach != nil && w.attach.isAlive() {
		return fmt.Errorf("workspace %s is already attached", w.name)
	}
	if w.attach != nil {
		log.Printf("DEBUG: openshell: clearing stale attach for %s", w.name)
		w.attach = nil
	}

	w.loadSandboxID()
	if w.sandboxID == "" {
		return fmt.Errorf("workspace %s has no sandbox; create it first", w.name)
	}
	if err := w.ensureClient(); err != nil {
		return err
	}

	cmdStr := w.resolveSandboxCommand()
	var command []string
	if strings.ContainsAny(cmdStr, `"'\`) {
		command = []string{"bash", "-lc", cmdStr}
	} else {
		command = strings.Fields(cmdStr)
	}

	fd := int(os.Stdin.Fd())
	cols, rows := uint32(80), uint32(24)
	if term.IsTerminal(fd) {
		if w, h, err := term.GetSize(fd); err == nil {
			cols, rows = uint32(w), uint32(h)
		}
	}

	attachCtx, attachCancel := context.WithCancel(ctx)
	defer attachCancel()
	w.attach = &attachState{active: true, cancel: attachCancel}
	defer func() { w.attach = nil }()

	session, err := w.client.Exec().Interactive(attachCtx, w.sandboxID, command, cols, rows)
	if err != nil {
		return fmt.Errorf("starting interactive session: %w", err)
	}
	defer session.Close()

	now := time.Now()
	if inst, loadErr := w.store.FindInstanceByName(w.name); loadErr == nil && inst != nil {
		inst.LastAttached = &now
		_ = w.store.UpdateInstance(inst)
	}

	if term.IsTerminal(fd) {
		oldState, rawErr := term.MakeRaw(fd)
		if rawErr != nil {
			return fmt.Errorf("setting terminal to raw mode: %w", rawErr)
		}
		defer term.Restore(fd, oldState)
	}

	stopResize := watchTerminalResize(session)
	defer stopResize()

	errCh := make(chan error, 2)
	go func() {
		_, copyErr := io.Copy(session, os.Stdin)
		errCh <- copyErr
	}()
	go func() {
		_, copyErr := io.Copy(os.Stdout, session)
		errCh <- copyErr
	}()

	select {
	case <-attachCtx.Done():
	case ioErr := <-errCh:
		if ioErr != nil && ioErr != io.EOF {
			log.Printf("DEBUG: openshell: I/O error: %v", ioErr)
		}
	}

	exitCode, exitErr := session.ExitCode()
	if exitErr != nil {
		log.Printf("DEBUG: openshell: exit code unavailable: %v", exitErr)
	} else if exitCode != 0 {
		return fmt.Errorf("session exited with code %d", exitCode)
	}

	return nil
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
	stream, err := w.client.Exec().Stream(ctx, w.sandboxID, cmd)
	if err != nil {
		return err
	}
	if stream == nil {
		return nil
	}
	defer stream.Close()
	var streamErr error
	for {
		chunk, chunkErr := stream.Next()
		if chunkErr != nil {
			streamErr = chunkErr
			break
		}
		if chunk == nil {
			break
		}
		if chunk.Stream == "stderr" {
			os.Stderr.Write(chunk.Data)
		} else {
			os.Stdout.Write(chunk.Data)
		}
	}
	exitCode, exitErr := stream.ExitCode()
	if exitErr != nil {
		return exitErr
	}
	if exitCode != 0 {
		return fmt.Errorf("command exited with code %d", exitCode)
	}
	if streamErr != nil && !errors.Is(streamErr, io.EOF) {
		return streamErr
	}
	return nil
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
	result, err := w.client.Exec().Run(ctx, w.sandboxID, cmd)
	if err != nil {
		return "", err
	}
	if result.ExitCode != 0 {
		return string(result.Stdout), fmt.Errorf("command exited with code %d: %s", result.ExitCode, string(result.Stderr))
	}
	return string(result.Stdout), nil
}

// loadSDKPolicy reads a policy YAML file and converts it to the SDK's SandboxPolicy type.
func loadSDKPolicy(path string) (*v1.SandboxPolicy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading policy file: %w", err)
	}

	var pf build.PolicyFile
	if err := yaml.Unmarshal(data, &pf); err != nil {
		return nil, fmt.Errorf("parsing policy YAML: %w", err)
	}

	policy := &v1.SandboxPolicy{
		Version: uint32(pf.Version),
	}

	if pf.FilesystemPolicy != nil {
		policy.Filesystem = &types.FilesystemPolicy{
			IncludeWorkdir: pf.FilesystemPolicy.IncludeWorkdir,
			ReadOnly:       pf.FilesystemPolicy.ReadOnly,
			ReadWrite:      pf.FilesystemPolicy.ReadWrite,
		}
	}
	if pf.Landlock != nil {
		policy.Landlock = &types.LandlockPolicy{
			Compatibility: pf.Landlock.Compatibility,
		}
	}
	if pf.Process != nil {
		policy.Process = &types.ProcessPolicy{
			RunAsUser:  pf.Process.RunAsUser,
			RunAsGroup: pf.Process.RunAsGroup,
		}
	}
	if len(pf.NetworkPolicies) > 0 {
		policy.NetworkPolicies = make(map[string]types.NetworkPolicyRule, len(pf.NetworkPolicies))
		for key, np := range pf.NetworkPolicies {
			rule := types.NetworkPolicyRule{Name: np.Name}
			for _, ep := range np.Endpoints {
				port := uint32(0)
				if ep.Port > 0 && ep.Port <= 65535 {
					port = uint32(ep.Port)
				}
				sdkEP := types.PolicyNetworkEndpoint{
					Host:        ep.Host,
					Port:        port,
					Protocol:    ep.Protocol,
					Enforcement: ep.Enforcement,
					Access:      ep.Access,
				}
				for _, r := range ep.Rules {
					if r.Allow != nil {
						sdkEP.Rules = append(sdkEP.Rules, types.L7Rule{
							Allow: &types.L7Allow{
								Method: r.Allow.Method,
								Path:   r.Allow.Path,
							},
						})
					}
				}
				rule.Endpoints = append(rule.Endpoints, sdkEP)
			}
			for _, b := range np.Binaries {
				rule.Binaries = append(rule.Binaries, types.PolicyNetworkBinary{
					Path: b.Path,
				})
			}
			policy.NetworkPolicies[key] = rule
		}
	}

	return policy, nil
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
