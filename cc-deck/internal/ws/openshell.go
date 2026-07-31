package ws

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cc-deck/cc-deck/internal/agent"
	"github.com/cc-deck/cc-deck/internal/build"
	"github.com/cc-deck/cc-deck/internal/credential"
	"github.com/cc-deck/cc-deck/internal/oci"
	"github.com/cc-deck/cc-deck/internal/openshell"
	v1 "github.com/rhuss/openshell-sdk-go/openshell/v1"
	"github.com/rhuss/openshell-sdk-go/openshell/v1/types"
	"golang.org/x/term"
)

const (
	defaultSandboxImage   = "cc-deck/openshell-sandbox:latest"
	defaultSandboxCommand = "zellij"
)

// SandboxConfig holds sandbox provisioning parameters.
type SandboxConfig struct {
	Image     string
	Command   string
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

// profileManifestPath is the well-known location of the profile manifest
// inside OpenShell sandbox images.
const profileManifestPath = "/etc/openshell/profiles.yaml"

// extractProfileManifest extracts and parses the profile manifest from an
// OCI image. Returns nil (without error) if the manifest is not present.
func extractProfileManifest(image string) (*build.ProfileManifest, error) {
	data, err := oci.ExtractFileFromImage(image, profileManifestPath)
	if err != nil {
		log.Printf("INFO: no profile manifest in image %s: %v", image, err)
		return nil, nil
	}
	pm, parseErr := build.ParseProfileManifest(data)
	if parseErr != nil {
		return nil, fmt.Errorf("parsing profile manifest from image %s: %w", image, parseErr)
	}
	return pm, nil
}

// importMCPProfile creates and imports an ephemeral profile containing all MCP
// endpoints and agent binaries for a workspace, then creates a provider
// referencing that profile. Returns the provider name, or empty string if no
// MCP entries have endpoints.
func importMCPProfile(ctx context.Context, client v1.ClientInterface, wsName string, mcpEntries []build.MCPManifestEntry, agentBinaries []string) (string, error) {
	var endpoints []types.NetworkEndpoint
	for _, mcp := range mcpEntries {
		if mcp.Endpoint == "" {
			continue
		}
		host, port, err := parseHostPort(mcp.Endpoint)
		if err != nil {
			log.Printf("WARNING: MCP %q endpoint %q: %v, skipping", mcp.Name, mcp.Endpoint, err)
			continue
		}
		endpoints = append(endpoints, types.NetworkEndpoint{
			Host:     host,
			Port:     uint32(port),
			Protocol: "rest",
		})
	}

	if len(endpoints) == 0 {
		return "", nil
	}

	var binaries []types.NetworkBinary
	for _, b := range agentBinaries {
		binaries = append(binaries, types.NetworkBinary{Path: b})
	}

	profileID := fmt.Sprintf("cc-deck-%s-mcp", openshell.SanitizeWorkspaceName(wsName))
	profile := types.ProviderProfile{
		ID:          profileID,
		DisplayName: fmt.Sprintf("MCP endpoints for %s", wsName),
		Category:    types.ProfileCategoryOther,
		Endpoints:   endpoints,
		Binaries:    binaries,
	}

	_, err := client.Providers().Profiles().Import(ctx, []types.ProfileImportItem{
		{Profile: profile},
	})
	if err != nil {
		log.Printf("WARNING: failed to import MCP profile %s: %v", profileID, err)
		return "", nil
	}

	providerName := profileID
	provider := &v1.Provider{
		Name: providerName,
		Type: profileID,
	}
	if _, err := client.Providers().Ensure(ctx, provider); err != nil {
		return "", fmt.Errorf("creating MCP provider %s: %w", providerName, err)
	}
	log.Printf("DEBUG: openshell: imported MCP profile %s with %d endpoints", profileID, len(endpoints))
	return providerName, nil
}

// importCustomDomainsProfile creates and imports an ephemeral profile
// containing user-defined domain endpoints. Returns the provider name,
// or empty string if no domains are given.
func importCustomDomainsProfile(ctx context.Context, client v1.ClientInterface, wsName string, domains []string) (string, error) {
	if len(domains) == 0 {
		return "", nil
	}

	var endpoints []types.NetworkEndpoint
	for _, d := range domains {
		endpoints = append(endpoints, types.NetworkEndpoint{
			Host:     d,
			Port:     443,
			Protocol: "https",
		})
	}

	profileID := fmt.Sprintf("cc-deck-%s-custom", openshell.SanitizeWorkspaceName(wsName))
	profile := types.ProviderProfile{
		ID:          profileID,
		DisplayName: fmt.Sprintf("Custom domains for %s", wsName),
		Category:    types.ProfileCategoryOther,
		Endpoints:   endpoints,
	}

	_, err := client.Providers().Profiles().Import(ctx, []types.ProfileImportItem{
		{Profile: profile},
	})
	if err != nil {
		log.Printf("WARNING: failed to import custom domains profile %s: %v", profileID, err)
		return "", nil
	}

	providerName := profileID
	provider := &v1.Provider{
		Name: providerName,
		Type: profileID,
	}
	if _, err := client.Providers().Ensure(ctx, provider); err != nil {
		return "", fmt.Errorf("creating custom domains provider %s: %w", providerName, err)
	}
	log.Printf("DEBUG: openshell: imported custom domains profile %s with %d domains", profileID, len(domains))
	return providerName, nil
}

// parseHostPort splits a "host:port" string into its components. Uses
// net.SplitHostPort for correct IPv6 handling.
func parseHostPort(endpoint string) (string, int, error) {
	host, portStr, err := net.SplitHostPort(endpoint)
	if err != nil {
		return "", 0, fmt.Errorf("parsing endpoint %q: %w", endpoint, err)
	}
	if host == "" {
		return "", 0, fmt.Errorf("empty host in %q", endpoint)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, fmt.Errorf("invalid port in %q: %w", endpoint, err)
	}
	if port <= 0 || port > 65535 {
		return "", 0, fmt.Errorf("port %d out of range in %q", port, endpoint)
	}
	return host, port, nil
}

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
	if def.Provider != "" {
		cfg.Providers = []string{def.Provider}
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

// resolveAgentNames always returns ["claude"]. Multi-agent support is
// not yet implemented; this stub centralizes the default.
func resolveAgentNames() []string {
	return []string{"claude"}
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
// provider name, type, and credentials map. The providerType is resolved from
// the profile mapping table. Returns empty providerType when no provider
// should be created (e.g., bedrock has no OpenShell provider).
func mapToOpenShellProvider(wsName string, agentName string, spec agent.CredentialSpec, resolved credential.ResolvedCredentials) (name, providerType string, creds map[string]string) {
	name = fmt.Sprintf("cc-deck-%s-%s", wsName, spec.Name)
	providerType = openshell.LookupCredentialProfile(spec.Name, agentName)

	switch spec.Name {
	case "api":
		creds = make(map[string]string)
		for k, v := range resolved.EnvVars {
			creds[k] = v
		}
	case "vertex":
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

// createProfileProviders creates OpenShell providers for each profile ID in the
// manifest. It verifies profiles against the gateway, creates one provider per
// verified profile, and returns the list of created provider names.
func createProfileProviders(ctx context.Context, client v1.ClientInterface, wsName string, profileIDs []string) ([]string, error) {
	if len(profileIDs) == 0 {
		return nil, nil
	}

	verified, missing, verifyErr := openshell.VerifyProfiles(ctx, client, profileIDs)
	if verifyErr != nil {
		return nil, fmt.Errorf("verifying profiles: %w", verifyErr)
	}
	if len(missing) > 0 {
		log.Printf("WARNING: %d profiles not found on gateway: %v", len(missing), missing)
	}

	var providers []string
	for _, profileID := range verified {
		providerName := fmt.Sprintf("cc-deck-%s-%s", openshell.SanitizeWorkspaceName(wsName), profileID)
		provider := &v1.Provider{
			Name: providerName,
			Type: profileID,
		}
		if _, err := client.Providers().Ensure(ctx, provider); err != nil {
			return nil, fmt.Errorf("creating profile provider %s (type=%s): %w", providerName, profileID, err)
		}
		providers = append(providers, providerName)
		log.Printf("DEBUG: openshell: created profile provider %s (type=%s)", providerName, profileID)
	}
	return providers, nil
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

	// Resolve credentials via agent-declared specs.
	var resolved credential.ResolvedCredentials
	var credProviders []string

	agentNames := resolveAgentNames()

	authField := w.resolveAuthField()
	for _, agentName := range agentNames {
		agentObj := agent.Get(agentName)
		if agentObj == nil {
			continue
		}
		specs := agentObj.CredentialSpecs()
		available := credential.Detect(specs)
		selectedSpec, found, selectErr := selectCredentialMode(available, authField)
		if selectErr != nil {
			return selectErr
		}
		if !found {
			continue
		}
		resolved = credential.Resolve(selectedSpec)
		providerName, providerType, providerCreds := mapToOpenShellProvider(w.name, agentName, selectedSpec, resolved)
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
		break
	}

	// Extract profile manifest from OCI image and create providers.
	var profileProviders []string
	if sbCfg.Image != "" {
		pm, pmErr := extractProfileManifest(sbCfg.Image)
		if pmErr != nil {
			return pmErr
		}
		if pm != nil && len(pm.Profiles) > 0 {
			pp, ppErr := createProfileProviders(ctx, w.client, w.name, pm.Profiles)
			if ppErr != nil {
				return ppErr
			}
			profileProviders = pp
			log.Printf("INFO: openshell: using profile-based path with %d profiles", len(pm.Profiles))

			if len(pm.MCP) > 0 {
				mcpProvider, mcpErr := importMCPProfile(ctx, w.client, w.name, pm.MCP, pm.AgentBinaries)
				if mcpErr != nil {
					return mcpErr
				}
				if mcpProvider != "" {
					profileProviders = append(profileProviders, mcpProvider)
				}
			}

			if len(pm.CustomDomains) > 0 {
				domainProvider, domErr := importCustomDomainsProfile(ctx, w.client, w.name, pm.CustomDomains)
				if domErr != nil {
					return domErr
				}
				if domainProvider != "" {
					profileProviders = append(profileProviders, domainProvider)
				}
			}
		}
	}

	// Merge all providers: definition providers + profile providers + credential providers.
	allProviders := append(sbCfg.Providers, profileProviders...)
	allProviders = append(allProviders, credProviders...)

	sbSpec := &v1.SandboxSpec{
		Template: &v1.SandboxTemplate{
			Image: sbCfg.Image,
		},
		Providers: allProviders,
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
