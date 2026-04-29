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

	"github.com/cc-deck/cc-deck/internal/ssh"
)

const (
	sshSessionPrefix   = "cc-deck-"
	defaultSSHWorkspace = "~/workspace"
)

// SSHWorkspace manages a remote development workspace over SSH.
type SSHWorkspace struct {
	name  string
	store *FileStateStore
	defs  *DefinitionStore

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

// Type returns WorkspaceTypeSSH.
func (e *SSHWorkspace) Type() WorkspaceType {
	return WorkspaceTypeSSH
}

// Name returns the workspace name.
func (e *SSHWorkspace) Name() string {
	return e.name
}

// sshSessionName returns the Zellij session name for this workspace.
func (e *SSHWorkspace) sshSessionName() string {
	return sshSessionPrefix + e.name
}

// newSSHClient creates an SSH client from the workspace definition.
func (e *SSHWorkspace) newSSHClient(def *WorkspaceDefinition) *ssh.Client {
	return ssh.NewClient(def.Host, def.Port, def.IdentityFile, def.JumpHost, def.SSHConfig)
}

// workspacePath returns the configured workspace path (before remote resolution).
func workspacePath(def *WorkspaceDefinition) string {
	if def.Workspace != "" {
		return def.Workspace
	}
	return defaultSSHWorkspace
}

// resolveWorkspaceRemote resolves the workspace path to an absolute path on the
// remote host. This handles tilde expansion safely by asking the remote shell
// to evaluate the path, avoiding shell injection from unquoted variables.
func resolveWorkspaceRemote(ctx context.Context, runner ssh.Runner, ws string) (string, error) {
	// Use eval with printf to safely expand ~ and $HOME on the remote.
	out, err := runner.Run(ctx, fmt.Sprintf("eval printf '%%s' %q", ws))
	if err != nil {
		return "", fmt.Errorf("resolving workspace path %q on remote: %w", ws, err)
	}
	resolved := strings.TrimSpace(out)
	if resolved == "" {
		return "", fmt.Errorf("workspace path %q resolved to empty string on remote", ws)
	}
	return resolved, nil
}

// Create provisions a new SSH workspace.
func (e *SSHWorkspace) Create(ctx context.Context, _ CreateOpts) error {
	if err := ValidateWsName(e.name); err != nil {
		return err
	}

	if _, err := e.store.FindInstanceByName(e.name); err == nil {
		return fmt.Errorf("instance %q: %w", e.name, ErrNameConflict)
	}

	if _, err := exec.LookPath("ssh"); err != nil {
		return ErrSSHNotFound
	}

	def, err := e.loadDefinition()
	if err != nil {
		return err
	}

	if def.Host == "" {
		return fmt.Errorf("SSH host is required for workspace %q", e.name)
	}

	client := e.newSSHClient(def)

	// Verify SSH connectivity and check that the host is provisioned.
	fmt.Fprintf(os.Stderr, "Checking host %s...\n", def.Host)
	if err := client.Check(ctx); err != nil {
		return fmt.Errorf("SSH connectivity failed: %w", err)
	}
	if err := ssh.Probe(ctx, client); err != nil {
		return err
	}

	workspace, err := resolveWorkspaceRemote(ctx, client, workspacePath(def))
	if err != nil {
		return err
	}
	// Create the workspace directory on the remote.
	if _, mkErr := client.Run(ctx, fmt.Sprintf("mkdir -p %q", workspace)); mkErr != nil {
		return fmt.Errorf("creating workspace directory: %w", mkErr)
	}
	// Clone repos into workspace if defined.
	if len(e.Repos) > 0 {
		creds := loadActiveGitCredentials()
		if creds != nil && creds.Type == "ssh" {
			client.AgentForwarding = true
		}
		sshRunner := func(ctx2 context.Context, cmd string) (string, error) {
			return client.Run(ctx2, cmd)
		}
		fmt.Fprintf(os.Stderr, "Cloning %d repo(s) into %s...\n", len(e.Repos), workspace)
		cloneRepos(ctx, sshRunner, e.Repos, workspace, creds, e.ExtraRemotes, e.AutoDetectedURL)

		// FR-012: Set workspace to auto-detected repo's directory so Zellij
		// opens in the right place.
		if e.AutoDetectedURL != "" {
			for _, r := range e.Repos {
				if NormalizeURL(r.URL) == e.AutoDetectedURL {
					workspace = workspace + "/" + TargetDir(r)
					break
				}
			}
		}
	}

	inst := &WorkspaceInstance{
		Name:         e.name,
		Type:         WorkspaceTypeSSH,
		SessionState: SessionStateNone,
		CreatedAt:    time.Now().UTC(),
		SSH: &SSHFields{
			Host:         def.Host,
			Port:         def.Port,
			IdentityFile: def.IdentityFile,
			JumpHost:     def.JumpHost,
			SSHConfig:    def.SSHConfig,
			Workspace:    workspace,
		},
	}

	return e.store.AddInstance(inst)
}

// Attach opens an interactive SSH session to the remote Zellij session.
func (e *SSHWorkspace) Attach(ctx context.Context) error {
	if os.Getenv("ZELLIJ") != "" {
		fmt.Fprintf(os.Stderr, "Already inside Zellij. Detach first (Ctrl+o d), then run:\n")
		fmt.Fprintf(os.Stderr, "  cc-deck ws attach %s\n", e.name)
		return nil
	}

	def, err := e.loadDefinition()
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	if inst, findErr := e.store.FindInstanceByName(e.name); findErr == nil {
		inst.LastAttached = &now
		inst.SessionState = SessionStateExists
		_ = e.store.UpdateInstance(inst)
	}

	client := e.newSSHClient(def)
	client.AgentForwarding = true

	// Write credentials to remote before attaching (unless auth=none).
	if def.Auth != "none" {
		creds, credErr := ssh.BuildCredentialSet(def.Auth, def.Credentials, def.Env)
		if credErr != nil {
			return fmt.Errorf("building credentials: %w", credErr)
		}
		if len(creds) > 0 {
			if writeErr := ssh.WriteCredentialFile(ctx, client, creds); writeErr != nil {
				log.Printf("WARNING: could not write credentials to remote: %v", writeErr)
			}
		}
	}

	sessionName := e.sshSessionName()

	// Check if a Zellij session already exists on the remote.
	hasSession := e.remoteHasSession(client, sessionName)

	if !hasSession {
		// Create the session in the background. Try with cc-deck layout first,
		// fall back to default layout if the cc-deck layout is not installed.
		workspace, wsErr := resolveWorkspaceRemote(ctx, client, workspacePath(def))
		if wsErr != nil {
			return wsErr
		}
		createCmd := fmt.Sprintf("mkdir -p %q && cd %q && zellij --layout cc-deck attach --create-background %q",
			workspace, workspace, sessionName)
		if _, err := client.Run(ctx, createCmd); err != nil {
			// Layout not found, retry without layout specification.
			createCmd = fmt.Sprintf("cd %q && zellij attach --create-background %q",
				workspace, sessionName)
			if _, retryErr := client.Run(ctx, createCmd); retryErr != nil {
				return fmt.Errorf("creating remote Zellij session: %w", retryErr)
			}
			log.Printf("NOTE: cc-deck layout not found on remote, using default layout")
		}
	}

	// Set terminal background color for remote sessions if configured.
	remoteBG := LoadRemoteBG(e.name, e.defs)
	if remoteBG != "" {
		client.OnAttach = func() { SetRemoteBG(remoteBG) }
		client.OnDetachEscape = ResetBGEscape
	}

	// Replace current process with SSH to attach to the remote Zellij session.
	attachCmd := fmt.Sprintf("zellij attach %s", sessionName)
	return client.RunInteractive(attachCmd)
}

// Delete removes the SSH workspace.
func (e *SSHWorkspace) Delete(ctx context.Context, force bool) error {
	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil {
		return err
	}

	if !force && inst.SessionState == SessionStateExists {
		return ErrRunning
	}

	_ = e.KillSession(ctx)

	if err := e.store.RemoveInstance(e.name); err != nil {
		log.Printf("WARNING: removing instance from state: %v", err)
	}

	// Remove definition (best-effort, matching container/compose behavior).
	if e.defs != nil {
		if err := e.defs.Remove(e.name); err != nil {
			log.Printf("WARNING: removing definition: %v", err)
		}
	}

	return nil
}

// SyncRepos clones missing repos on the remote workspace.
func (e *SSHWorkspace) SyncRepos(ctx context.Context, repos []RepoEntry) error {
	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil {
		return err
	}
	if inst.SSH == nil {
		return fmt.Errorf("workspace %q has no SSH configuration", e.name)
	}

	client := ssh.NewClient(inst.SSH.Host, inst.SSH.Port, inst.SSH.IdentityFile, inst.SSH.JumpHost, inst.SSH.SSHConfig)
	client.AgentForwarding = true

	workspace := inst.SSH.Workspace
	if workspace == "" {
		workspace = "~/workspace"
	}

	sshRunner := func(ctx2 context.Context, cmd string) (string, error) {
		return client.Run(ctx2, cmd)
	}

	fmt.Fprintf(os.Stderr, "Syncing %d repo(s) to %s...\n", len(repos), workspace)
	cloneRepos(ctx, sshRunner, repos, workspace, nil, nil, "")
	return nil
}

// KillSession kills the remote Zellij session without affecting the remote host.
func (e *SSHWorkspace) KillSession(ctx context.Context) error {
	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil || inst.SSH == nil {
		return nil
	}
	client := ssh.NewClient(inst.SSH.Host, inst.SSH.Port, inst.SSH.IdentityFile, inst.SSH.JumpHost, inst.SSH.SSHConfig)
	sessionName := e.sshSessionName()
	if !e.remoteHasSession(client, sessionName) {
		return nil
	}
	killCmd := fmt.Sprintf("zellij kill-session %q", sessionName)
	_, _ = client.Run(ctx, killCmd)
	deleteCmd := fmt.Sprintf("zellij delete-session --force %q", sessionName)
	if _, err := client.Run(ctx, deleteCmd); err != nil {
		return fmt.Errorf("deleting remote session: %w", err)
	}
	inst.SessionState = SessionStateNone
	_ = e.store.UpdateInstance(inst)
	return nil
}

// Status queries the remote for current workspace status.
func (e *SSHWorkspace) Status(ctx context.Context) (*WorkspaceStatus, error) {
	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil {
		return nil, err
	}

	status := &WorkspaceStatus{
		SessionState: SessionStateNone,
		Since:        &inst.CreatedAt,
	}

	if inst.SSH == nil {
		return status, nil
	}

	client := ssh.NewClient(inst.SSH.Host, inst.SSH.Port, inst.SSH.IdentityFile, inst.SSH.JumpHost, inst.SSH.SSHConfig)
	sessionName := e.sshSessionName()

	if e.remoteHasSession(client, sessionName) {
		status.SessionState = SessionStateExists
	} else if err := client.Check(ctx); err != nil {
		status.Message = fmt.Sprintf("host unreachable: %v", err)
	}

	return status, nil
}

// Exec runs a command on the remote in the workspace directory.
func (e *SSHWorkspace) Exec(ctx context.Context, cmd []string) error {
	def, err := e.loadDefinition()
	if err != nil {
		return err
	}

	client := e.newSSHClient(def)
	workspace, wsErr := resolveWorkspaceRemote(ctx, client, workspacePath(def))
	if wsErr != nil {
		return wsErr
	}

	remoteCmd := fmt.Sprintf("cd %q && %s", workspace, shellJoin(cmd))
	out, err := client.Run(ctx, remoteCmd)
	if err != nil {
		return err
	}

	if out != "" {
		fmt.Fprintln(os.Stdout, out)
	}
	return nil
}

// ExecOutput runs a command on the remote and returns stdout.
func (e *SSHWorkspace) ExecOutput(ctx context.Context, cmd []string) (string, error) {
	def, err := e.loadDefinition()
	if err != nil {
		return "", err
	}

	client := e.newSSHClient(def)
	workspace, wsErr := resolveWorkspaceRemote(ctx, client, workspacePath(def))
	if wsErr != nil {
		return "", wsErr
	}

	remoteCmd := fmt.Sprintf("cd %q && %s", workspace, shellJoin(cmd))
	return client.Run(ctx, remoteCmd)
}

// PipeChannel returns the pipe channel for this workspace.
func (e *SSHWorkspace) PipeChannel(_ context.Context) (PipeChannel, error) {
	e.pipeOnce.Do(func() {
		e.pipeCh = &execPipeChannel{name: e.name, execFn: e.Exec, execOutputFn: e.ExecOutput}
	})
	return e.pipeCh, nil
}

// DataChannel returns the data channel for this workspace.
func (e *SSHWorkspace) DataChannel(_ context.Context) (DataChannel, error) {
	e.dataOnce.Do(func() {
		def, err := e.loadDefinition()
		if err != nil {
			e.dataErr = err
			return
		}
		client := e.newSSHClient(def)
		e.dataCh = &sshDataChannel{
			name:     e.name,
			clientFn: func() *ssh.Client { return client },
			workspace: func(ctx context.Context) (string, error) {
				return resolveWorkspaceRemote(ctx, client, workspacePath(def))
			},
		}
	})
	return e.dataCh, e.dataErr
}

// GitChannel returns the git channel for this workspace.
func (e *SSHWorkspace) GitChannel(_ context.Context) (GitChannel, error) {
	e.gitOnce.Do(func() {
		inst, err := e.store.FindInstanceByName(e.name)
		if err != nil {
			e.gitErr = err
			return
		}
		if inst.SSH == nil {
			e.gitErr = fmt.Errorf("SSH fields missing for workspace %q", e.name)
			return
		}
		workspace := inst.SSH.Workspace
		if workspace == "" {
			workspace = defaultSSHWorkspace
		}
		e.gitCh = &sshGitChannel{
			name:      e.name,
			host:      inst.SSH.Host,
			workspace: workspace,
		}
	})
	return e.gitCh, e.gitErr
}

// Push synchronizes local files to the remote workspace via DataChannel.
func (e *SSHWorkspace) Push(ctx context.Context, opts SyncOpts) error {
	ch, err := e.DataChannel(ctx)
	if err != nil {
		return err
	}
	return ch.Push(ctx, opts)
}

// Pull synchronizes files from the remote workspace to local storage via DataChannel.
func (e *SSHWorkspace) Pull(ctx context.Context, opts SyncOpts) error {
	ch, err := e.DataChannel(ctx)
	if err != nil {
		return err
	}
	return ch.Pull(ctx, opts)
}

// Harvest retrieves git commits from the remote repository via GitChannel.
func (e *SSHWorkspace) Harvest(ctx context.Context, opts HarvestOpts) error {
	ch, err := e.GitChannel(ctx)
	if err != nil {
		return err
	}
	return ch.Fetch(ctx, opts)
}

// loadDefinition loads the workspace definition from the definition store.
func (e *SSHWorkspace) loadDefinition() (*WorkspaceDefinition, error) {
	if e.defs == nil {
		return nil, fmt.Errorf("no definition store available")
	}
	return e.defs.FindByName(e.name)
}

// remoteHasSession checks if a Zellij session with the given name exists
// on the remote host. Uses a short timeout to avoid blocking.
func (e *SSHWorkspace) remoteHasSession(client *ssh.Client, sessionName string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	out, err := client.Run(ctx, "zellij list-sessions -n")
	if err != nil {
		return false
	}

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == sessionName {
			return true
		}
		// Handle format "session-name (EXITED)" - skip exited sessions.
		if strings.HasPrefix(line, sessionName) && !strings.Contains(line, "(EXITED") {
			return true
		}
	}

	return false
}

// ReconcileSSHWorkspaces updates the state of all SSH workspace instances
// by querying their remote hosts for Zellij session status.
func ReconcileSSHWorkspaces(store *FileStateStore) error {
	instances, err := store.ListInstances(nil)
	if err != nil {
		return err
	}

	for _, inst := range instances {
		if inst.SSH == nil {
			continue
		}

		client := ssh.NewClient(inst.SSH.Host, inst.SSH.Port, inst.SSH.IdentityFile, inst.SSH.JumpHost, inst.SSH.SSHConfig)
		sessionName := sshSessionPrefix + inst.Name

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

		var newSess SessionStateValue = SessionStateNone
		if err := client.Check(ctx); err == nil {
			out, _ := client.Run(ctx, "zellij list-sessions -n")
			for _, line := range strings.Split(out, "\n") {
				line = strings.TrimSpace(line)
				if line == sessionName || (strings.HasPrefix(line, sessionName) && !strings.Contains(line, "(EXITED")) {
					newSess = SessionStateExists
					break
				}
			}
		}

		cancel()

		if inst.SessionState != newSess {
			inst.SessionState = newSess
			if err := store.UpdateInstance(inst); err != nil {
				return err
			}
		}
	}

	return nil
}
