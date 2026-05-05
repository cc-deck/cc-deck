package ws

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

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
	Image    string
	Command  string
	Policy   string
	Provider string
}

// sshTunnelState tracks an active SSH tunnel into a sandbox.
type sshTunnelState struct {
	sessionID string
	localPort int
	pid       int
	connected bool
}

func (t *sshTunnelState) isAlive() bool {
	if t == nil || t.pid == 0 {
		return false
	}
	proc, err := os.FindProcess(t.pid)
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
	client    *openshell.Client
	sandboxID string
	sshTunnel *sshTunnelState

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

// ensureClient lazily initializes the gRPC client from workspace definition.
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

func (w *OpenShellWorkspace) resolveSandboxConfig() SandboxConfig {
	cfg := SandboxConfig{
		Image:   defaultSandboxImage,
		Command: defaultSandboxCommand,
	}
	if w.defs == nil {
		return cfg
	}
	def, err := w.defs.FindByName(w.name)
	if err != nil || def == nil {
		return cfg
	}
	if def.SandboxImage != "" {
		cfg.Image = def.SandboxImage
	}
	if def.SandboxCommand != "" {
		cfg.Command = def.SandboxCommand
	}
	cfg.Policy = def.Policy
	cfg.Provider = def.Provider
	return cfg
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

	if w.sshTunnel != nil && w.sshTunnel.isAlive() {
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
	w.sshTunnel = nil
	log.Printf("DEBUG: openshell: cleared local state for %s", w.name)
}

// Create provisions a new OpenShell sandbox.
func (w *OpenShellWorkspace) Create(ctx context.Context, _ CreateOpts) error {
	if err := ValidateWsName(w.name); err != nil {
		return err
	}
	if err := w.ensureClient(); err != nil {
		return err
	}

	sbCfg := w.resolveSandboxConfig()
	sandboxID, err := w.client.CreateSandbox(ctx, sbCfg.Image, sbCfg.Command, sbCfg.Policy, sbCfg.Provider)
	if err != nil {
		return fmt.Errorf("creating sandbox: %w", err)
	}
	w.sandboxID = sandboxID

	if err := w.pollUntilRunning(ctx); err != nil {
		return err
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
// OpenShell sandboxes do not support suspend/resume, so Start always
// creates a fresh sandbox.
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
	w.sshTunnel = nil
	return nil
}

// Attach connects to the sandbox via SSH tunnel and attaches to Zellij.
func (w *OpenShellWorkspace) Attach(ctx context.Context) error {
	if w.sshTunnel != nil && w.sshTunnel.isAlive() {
		return fmt.Errorf("workspace %s is already attached (pid %d)", w.name, w.sshTunnel.pid)
	}
	if w.sshTunnel != nil {
		log.Printf("DEBUG: openshell: clearing stale tunnel for %s (pid %d dead)", w.name, w.sshTunnel.pid)
		w.sshTunnel = nil
	}

	w.loadSandboxID()
	if w.sandboxID == "" {
		return fmt.Errorf("workspace %s has no sandbox; create it first", w.name)
	}
	if err := w.ensureClient(); err != nil {
		return err
	}

	session, err := w.client.CreateSshSession(ctx, w.sandboxID)
	if err != nil {
		return fmt.Errorf("establishing SSH tunnel: %w", err)
	}

	w.sshTunnel = &sshTunnelState{
		sessionID: session.SessionID,
		localPort: session.Port,
		connected: true,
	}

	sshTarget := fmt.Sprintf("%s@%s", session.User, session.Host)
	// Host key verification disabled: sandbox SSH endpoints are ephemeral
	// and their keys rotate on every sandbox creation.
	sshArgs := []string{
		"-p", fmt.Sprintf("%d", session.Port),
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		sshTarget,
		"zellij", "attach", "--create",
	}
	sshCmd := exec.CommandContext(ctx, "ssh", sshArgs...)
	sshCmd.Stdin = os.Stdin
	sshCmd.Stdout = os.Stdout
	sshCmd.Stderr = os.Stderr
	if err = sshCmd.Start(); err != nil {
		w.sshTunnel = nil
		return fmt.Errorf("starting SSH session: %w", err)
	}
	w.sshTunnel.pid = sshCmd.Process.Pid
	err = sshCmd.Wait()

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
