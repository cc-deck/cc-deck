package env

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/cc-deck/cc-deck/internal/ssh"
)

const (
	sshSessionPrefix   = "cc-deck-"
	defaultSSHWorkspace = "~/workspace"
)

// SSHEnvironment manages a remote development environment over SSH.
type SSHEnvironment struct {
	name  string
	store *FileStateStore
	defs  *DefinitionStore

	Repos           []RepoEntry
	ExtraRemotes    map[string]string
	AutoDetectedURL string
}

// Type returns EnvironmentTypeSSH.
func (e *SSHEnvironment) Type() EnvironmentType {
	return EnvironmentTypeSSH
}

// Name returns the environment name.
func (e *SSHEnvironment) Name() string {
	return e.name
}

// sshSessionName returns the Zellij session name for this environment.
func (e *SSHEnvironment) sshSessionName() string {
	return sshSessionPrefix + e.name
}

// newSSHClient creates an SSH client from the environment definition.
func (e *SSHEnvironment) newSSHClient(def *EnvironmentDefinition) *ssh.Client {
	return ssh.NewClient(def.Host, def.Port, def.IdentityFile, def.JumpHost, def.SSHConfig)
}

// workspacePath returns the configured workspace path (before remote resolution).
func workspacePath(def *EnvironmentDefinition) string {
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

// Create provisions a new SSH environment.
func (e *SSHEnvironment) Create(ctx context.Context, _ CreateOpts) error {
	if err := ValidateEnvName(e.name); err != nil {
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
		return fmt.Errorf("SSH host is required for environment %q", e.name)
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

	inst := &EnvironmentInstance{
		Name:      e.name,
		Type:      EnvironmentTypeSSH,
		State:     EnvironmentStateRunning,
		CreatedAt: time.Now().UTC(),
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
func (e *SSHEnvironment) Attach(ctx context.Context) error {
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

	// Replace current process with SSH to attach to the remote Zellij session.
	attachCmd := fmt.Sprintf("zellij attach %s", sessionName)
	return client.RunInteractive(attachCmd)
}

// Delete removes the SSH environment.
func (e *SSHEnvironment) Delete(ctx context.Context, force bool) error {
	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil {
		return err
	}

	if !force && inst.State == EnvironmentStateRunning {
		return ErrRunning
	}

	// Best-effort: try to kill the remote Zellij session.
	if force && inst.SSH != nil {
		client := ssh.NewClient(inst.SSH.Host, inst.SSH.Port, inst.SSH.IdentityFile, inst.SSH.JumpHost, inst.SSH.SSHConfig)
		killCmd := fmt.Sprintf("zellij kill-session %q", e.sshSessionName())
		if _, err := client.Run(ctx, killCmd); err != nil {
			log.Printf("WARNING: could not kill remote session: %v", err)
		}
	}

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

// KillRemoteSession kills the Zellij session on the remote host (best-effort).
func (e *SSHEnvironment) KillRemoteSession(ctx context.Context) {
	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil || inst.SSH == nil {
		return
	}
	client := ssh.NewClient(inst.SSH.Host, inst.SSH.Port, inst.SSH.IdentityFile, inst.SSH.JumpHost, inst.SSH.SSHConfig)
	killCmd := fmt.Sprintf("zellij delete-session %q -f", e.sshSessionName())
	if _, err := client.Run(ctx, killCmd); err != nil {
		log.Printf("WARNING: could not kill remote session: %v", err)
	}
}

// Start is not supported for SSH environments.
func (e *SSHEnvironment) Start(_ context.Context) error {
	return ErrNotSupported
}

// Stop is not supported for SSH environments.
func (e *SSHEnvironment) Stop(_ context.Context) error {
	return ErrNotSupported
}

// Status queries the remote for current environment status.
func (e *SSHEnvironment) Status(ctx context.Context) (*EnvironmentStatus, error) {
	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil {
		return nil, err
	}

	status := &EnvironmentStatus{
		State: inst.State,
		Since: inst.CreatedAt,
	}

	if inst.SSH == nil {
		return status, nil
	}

	client := ssh.NewClient(inst.SSH.Host, inst.SSH.Port, inst.SSH.IdentityFile, inst.SSH.JumpHost, inst.SSH.SSHConfig)
	sessionName := e.sshSessionName()

	if e.remoteHasSession(client, sessionName) {
		status.State = EnvironmentStateRunning
	} else if err := client.Check(ctx); err != nil {
		status.State = EnvironmentStateError
		status.Message = fmt.Sprintf("host unreachable: %v", err)
	} else {
		status.State = EnvironmentStateStopped
	}

	return status, nil
}

// Exec runs a command on the remote in the workspace directory.
func (e *SSHEnvironment) Exec(ctx context.Context, cmd []string) error {
	def, err := e.loadDefinition()
	if err != nil {
		return err
	}

	client := e.newSSHClient(def)
	workspace, wsErr := resolveWorkspaceRemote(ctx, client, workspacePath(def))
	if wsErr != nil {
		return wsErr
	}

	remoteCmd := fmt.Sprintf("cd %q && %s", workspace, strings.Join(cmd, " "))
	out, err := client.Run(ctx, remoteCmd)
	if err != nil {
		return err
	}

	if out != "" {
		fmt.Fprintln(os.Stdout, out)
	}
	return nil
}

// Push synchronizes local files to the remote environment.
func (e *SSHEnvironment) Push(ctx context.Context, opts SyncOpts) error {
	def, err := e.loadDefinition()
	if err != nil {
		return err
	}

	if opts.LocalPath == "" {
		return fmt.Errorf("local path is required for push")
	}

	client := e.newSSHClient(def)
	remotePath := opts.RemotePath
	if remotePath == "" {
		resolved, wsErr := resolveWorkspaceRemote(ctx, client, workspacePath(def))
		if wsErr != nil {
			return wsErr
		}
		remotePath = resolved
	}

	return client.Rsync(ctx, opts.LocalPath, def.Host+":"+remotePath, opts.Excludes, true)
}

// Pull synchronizes files from the remote environment to local storage.
func (e *SSHEnvironment) Pull(ctx context.Context, opts SyncOpts) error {
	def, err := e.loadDefinition()
	if err != nil {
		return err
	}

	if opts.RemotePath == "" {
		return fmt.Errorf("remote path is required for pull")
	}

	client := e.newSSHClient(def)
	localPath := opts.LocalPath
	if localPath == "" {
		localPath = "."
	}

	return client.Rsync(ctx, def.Host+":"+opts.RemotePath, localPath, opts.Excludes, false)
}

// Harvest retrieves git commits from the remote repository.
func (e *SSHEnvironment) Harvest(ctx context.Context, opts HarvestOpts) error {
	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil {
		return err
	}

	if inst.SSH == nil {
		return fmt.Errorf("SSH fields missing for environment %q", e.name)
	}

	workspace := inst.SSH.Workspace
	if workspace == "" {
		workspace = defaultSSHWorkspace
	}

	remoteName := "cc-deck-" + e.name
	remoteURL := fmt.Sprintf("ssh://%s%s", inst.SSH.Host, workspace)

	// Add temporary remote.
	addCmd := exec.CommandContext(ctx, "git", "remote", "add", remoteName, remoteURL)
	if out, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("adding temporary remote: %s: %w", strings.TrimSpace(string(out)), err)
	}

	// Fetch from remote.
	fetchCmd := exec.CommandContext(ctx, "git", "fetch", remoteName)
	if out, err := fetchCmd.CombinedOutput(); err != nil {
		_ = exec.CommandContext(ctx, "git", "remote", "remove", remoteName).Run()
		return fmt.Errorf("fetching from remote: %s: %w", strings.TrimSpace(string(out)), err)
	}

	// Remove temporary remote.
	_ = exec.CommandContext(ctx, "git", "remote", "remove", remoteName).Run()

	fmt.Fprintf(os.Stdout, "Harvested commits from %s\n", e.name)

	if opts.CreatePR {
		branch := opts.Branch
		if branch == "" {
			branch = fmt.Sprintf("%s/main", remoteName)
		}
		prCmd := exec.CommandContext(ctx, "gh", "pr", "create", "--head", branch, "--fill")
		prCmd.Stdout = os.Stdout
		prCmd.Stderr = os.Stderr
		if err := prCmd.Run(); err != nil {
			return fmt.Errorf("creating PR: %w", err)
		}
	}

	return nil
}

// loadDefinition loads the environment definition from the definition store.
func (e *SSHEnvironment) loadDefinition() (*EnvironmentDefinition, error) {
	if e.defs == nil {
		return nil, fmt.Errorf("no definition store available")
	}
	return e.defs.FindByName(e.name)
}

// remoteHasSession checks if a Zellij session with the given name exists
// on the remote host. Uses a short timeout to avoid blocking.
func (e *SSHEnvironment) remoteHasSession(client *ssh.Client, sessionName string) bool {
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

// ReconcileSSHEnvs updates the state of all SSH environment instances
// by querying their remote hosts for Zellij session status.
func ReconcileSSHEnvs(store *FileStateStore) error {
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

		var newState EnvironmentState
		if err := client.Check(ctx); err != nil {
			newState = EnvironmentStateError
		} else {
			out, _ := client.Run(ctx, "zellij list-sessions -n")
			found := false
			for _, line := range strings.Split(out, "\n") {
				line = strings.TrimSpace(line)
				if line == sessionName || (strings.HasPrefix(line, sessionName) && !strings.Contains(line, "(EXITED")) {
					found = true
					break
				}
			}
			if found {
				newState = EnvironmentStateRunning
			} else {
				newState = EnvironmentStateStopped
			}
		}

		cancel() // Release timeout resources after all operations complete.

		if inst.State != newState {
			inst.State = newState
			if err := store.UpdateInstance(inst); err != nil {
				return err
			}
		}
	}

	return nil
}
