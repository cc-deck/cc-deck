package ws

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/cc-deck/cc-deck/internal/compose"
	"github.com/cc-deck/cc-deck/internal/network"
	"github.com/cc-deck/cc-deck/internal/podman"
)

const (
	ccDeckDir      = ".cc-deck"
	runSubdir      = "run"
	composeFile    = "compose.yaml"
	composeEnvFile = "env"
	proxySubdir    = "proxy"
)

// ComposeWorkspace manages a compose-based workspace using podman-compose.
type ComposeWorkspace struct {
	name  string
	store *FileStateStore
	defs  *DefinitionStore

	Auth           AuthMode
	Ports          []string
	AllPorts       bool
	Credentials    map[string]string
	Mounts         []string
	AllowedDomains []string
	ProjectDir     string
	Gitignore      bool

	Repos           []RepoEntry
	ExtraRemotes    map[string]string
	AutoDetectedURL string
}

// Type returns WorkspaceTypeCompose.
func (e *ComposeWorkspace) Type() WorkspaceType {
	return WorkspaceTypeCompose
}

// Name returns the workspace name.
func (e *ComposeWorkspace) Name() string {
	return e.name
}

func (e *ComposeWorkspace) composeProjectDir() string {
	return filepath.Join(e.projectDir(), ccDeckDir, runSubdir)
}

func (e *ComposeWorkspace) projectDir() string {
	if e.ProjectDir != "" {
		return e.ProjectDir
	}
	dir, _ := os.Getwd()
	return dir
}

func (e *ComposeWorkspace) sessionContainerName() string {
	return containerNamePrefix + e.name
}

func (e *ComposeWorkspace) proxyContainerName() string {
	return containerNamePrefix + e.name + "-proxy"
}

func (e *ComposeWorkspace) composeFilePath() string {
	return filepath.Join(e.composeProjectDir(), composeFile)
}

// Create provisions a new compose workspace.
func (e *ComposeWorkspace) Create(ctx context.Context, opts CreateOpts) error {
	if err := ValidateWsName(e.name); err != nil {
		return err
	}

	// Fail fast if a workspace with this name already exists.
	if _, err := e.store.FindInstanceByName(e.name); err == nil {
		return fmt.Errorf("instance %q: %w", e.name, ErrNameConflict)
	}

	runtime, err := compose.Available()
	if err != nil {
		return fmt.Errorf("compose runtime not available: %w", err)
	}

	// Check for existing definition and use as defaults.
	var def *WorkspaceDefinition
	if e.defs != nil {
		if d, defErr := e.defs.FindByName(e.name); defErr == nil {
			def = d
		}
	}

	// Resolve image.
	image := opts.Image
	if image == "" && def != nil && def.Image != "" {
		image = def.Image
	}
	if image == "" {
		log.Printf("WARNING: no image specified, using default %s", defaultImage)
		image = defaultImage
	}

	// Resolve storage type (compose defaults to host-path).
	storageType := opts.Storage.Type
	if storageType == "" && def != nil && def.Storage != nil {
		storageType = def.Storage.Type
	}
	if storageType == "" {
		storageType = StorageTypeHostPath
	}

	// Resolve project directory.
	projDir := e.projectDir()
	if def != nil && def.ProjectDir != "" && e.ProjectDir == "" {
		projDir = def.ProjectDir
		e.ProjectDir = projDir
	}

	// Resolve ports.
	ports := e.Ports
	if len(ports) == 0 && def != nil {
		ports = def.Ports
	}

	// Resolve credentials.
	creds := e.Credentials
	if creds == nil {
		creds = make(map[string]string)
	}

	// Resolve credentials from definition keys.
	if def != nil {
		for _, key := range def.Credentials {
			if _, exists := creds[key]; !exists {
				if val := os.Getenv(key); val != "" {
					creds[key] = val
				}
			}
		}
	}

	// Auth mode detection.
	authMode := e.Auth
	if authMode == "" && def != nil && def.Auth != "" {
		authMode = AuthMode(def.Auth)
	}
	if authMode == "" || authMode == AuthModeAuto {
		authMode = DetectAuthMode()
	}
	if authMode != AuthModeNone {
		DetectAuthCredentials(authMode, creds)
	}

	// Resolve allowed domains.
	allowedDomains := e.AllowedDomains
	if len(allowedDomains) == 0 && def != nil {
		allowedDomains = def.AllowedDomains
	}

	// Expand domain groups (load user-defined groups from config).
	var resolvedDomains []string
	if len(allowedDomains) > 0 {
		userGroups, loadErr := network.LoadUserConfig()
		if loadErr != nil {
			return fmt.Errorf("loading domain config: %w", loadErr)
		}
		resolver := network.NewResolver(userGroups)
		resolved, resolveErr := resolver.ExpandAll(allowedDomains)
		if resolveErr != nil {
			return fmt.Errorf("resolving domain groups: %w", resolveErr)
		}
		resolvedDomains = resolved
	}

	// Create .cc-deck/ directory.
	ccDeckDir := e.composeProjectDir()
	if info, statErr := os.Stat(ccDeckDir); statErr == nil && info.IsDir() {
		log.Printf("WARNING: regenerating compose files in %s", ccDeckDir)
	}
	if err := os.MkdirAll(ccDeckDir, 0o755); err != nil {
		return fmt.Errorf("creating %s directory: %w", filepath.Join(ccDeckDir, runSubdir), err)
	}

	// Build volumes.
	var volumes []string
	switch storageType {
	case StorageTypeHostPath:
		// Bind mount project dir at /workspace. UID mapping is handled by
		// userns_mode: keep-id in the compose service, so no :U flag needed
		// (which would fail on read-only files like .git/objects/pack/).
		volumes = append(volumes, "./../..:/workspace")
	case StorageTypeNamedVolume:
		vName := volumeName(e.name)
		if err := podman.VolumeCreate(ctx, vName); err != nil {
			e.cleanupOnFailure(ccDeckDir)
			return fmt.Errorf("creating volume: %w", err)
		}
		volumes = append(volumes, vName+":/workspace")
	}

	// Add extra mounts from definition.
	if len(e.Mounts) == 0 && def != nil {
		e.Mounts = def.Mounts
	}
	volumes = append(volumes, e.Mounts...)

	// Write credentials to env file. File-based credentials are mounted
	// directly from the host with :ro,U (read-only, ownership mapped to
	// container user). This keeps the host file as the single source of
	// truth, avoiding credential drift after re-authentication.
	var credentialKeys []string
	var envLines []string

	for key, val := range creds {
		credentialKeys = append(credentialKeys, key)

		// File-based credential: bind mount the original file read-only.
		if info, statErr := os.Stat(val); statErr == nil && !info.IsDir() {
			secretName := strings.ToLower(strings.ReplaceAll(key, "_", "-"))
			containerPath := "/run/secrets/" + secretName
			volumes = append(volumes, val+":"+containerPath+":ro")
			envLines = append(envLines, fmt.Sprintf("%s=%s", key, containerPath))
		} else {
			envLines = append(envLines, fmt.Sprintf("%s=%s", key, val))
		}
	}

	// Write env file.
	envContent := strings.Join(envLines, "\n")
	if envContent != "" {
		envContent += "\n"
	}
	if err := os.WriteFile(filepath.Join(ccDeckDir, composeEnvFile), []byte(envContent), 0o600); err != nil {
		e.cleanupOnFailure(ccDeckDir)
		return fmt.Errorf("writing env file: %w", err)
	}

	// Generate compose output.
	genOpts := compose.GenerateOptions{
		SessionName: e.sessionContainerName(),
		ImageRef:    image,
		Domains:     resolvedDomains,
		Volumes:     volumes,
		Ports:       ports,
	}

	output, err := compose.Generate(genOpts)
	if err != nil {
		e.cleanupOnFailure(ccDeckDir)
		return fmt.Errorf("generating compose files: %w", err)
	}

	// Write compose.yaml.
	if err := os.WriteFile(e.composeFilePath(), []byte(output.ComposeYAML), 0o644); err != nil {
		e.cleanupOnFailure(ccDeckDir)
		return fmt.Errorf("writing compose.yaml: %w", err)
	}

	// Write proxy config files if filtering is active.
	if output.TinyproxyConf != "" {
		proxyDir := filepath.Join(ccDeckDir, proxySubdir)
		if err := os.MkdirAll(proxyDir, 0o755); err != nil {
			e.cleanupOnFailure(ccDeckDir)
			return fmt.Errorf("creating proxy directory: %w", err)
		}
		if err := os.WriteFile(filepath.Join(proxyDir, "tinyproxy.conf"), []byte(output.TinyproxyConf), 0o644); err != nil {
			e.cleanupOnFailure(ccDeckDir)
			return fmt.Errorf("writing tinyproxy.conf: %w", err)
		}
		if err := os.WriteFile(filepath.Join(proxyDir, "whitelist"), []byte(output.Whitelist), 0o644); err != nil {
			e.cleanupOnFailure(ccDeckDir)
			return fmt.Errorf("writing whitelist: %w", err)
		}
	}

	// Ensure .cc-deck/.gitignore exists with run/ entry.
	if err := EnsureCCDeckGitignore(projDir); err != nil {
		log.Printf("WARNING: could not ensure .cc-deck/.gitignore: %v", err)
	}

	// Run podman-compose up -d.
	if err := e.composeUp(ctx, runtime); err != nil {
		e.cleanupOnFailure(ccDeckDir)
		return fmt.Errorf("starting compose project: %w", err)
	}

	// Clone repos into workspace if defined.
	if len(e.Repos) > 0 {
		creds := loadActiveGitCredentials()
		workspace := "/workspace"
		sessionContainer := e.sessionContainerName()
		podmanRunner := func(ctx2 context.Context, cmd string) (string, error) {
			return podman.ExecOutput(ctx2, sessionContainer, cmd)
		}
		fmt.Fprintf(os.Stderr, "Cloning %d repo(s) into %s...\n", len(e.Repos), workspace)
		cloneRepos(ctx, podmanRunner, e.Repos, workspace, creds, e.ExtraRemotes, e.AutoDetectedURL)
	}

	// Write workspace definition.
	if e.defs != nil {
		wsDef := &WorkspaceDefinition{
			Name: e.name,
			Type: WorkspaceTypeCompose,
			WorkspaceSpec: WorkspaceSpec{
				Image:          image,
				Ports:          ports,
				Credentials:    credentialKeys,
				Mounts:         e.Mounts,
				AllowedDomains: allowedDomains,
				ProjectDir:     projDir,
			},
		}
		if storageType != "" {
			wsDef.Storage = &StorageConfig{
				Type:     storageType,
				HostPath: opts.Storage.HostPath,
			}
		}
		if def != nil {
			_ = e.defs.Update(wsDef)
		} else {
			_ = e.defs.Add(wsDef)
		}
	}

	// Write workspace instance to state store.
	inst := &WorkspaceInstance{
		Name:      e.name,
		Type:      WorkspaceTypeCompose,
		State:     WorkspaceStateRunning,
		CreatedAt: time.Now().UTC(),
		Compose: &ComposeFields{
			ProjectDir:    projDir,
			ContainerName: e.sessionContainerName(),
		},
	}
	if len(resolvedDomains) > 0 {
		inst.Compose.ProxyName = e.proxyContainerName()
	}

	return e.store.AddInstance(inst)
}

// Attach opens an interactive Zellij session inside the session container.
func (e *ComposeWorkspace) Attach(ctx context.Context) error {
	if os.Getenv("ZELLIJ") != "" {
		fmt.Fprintf(os.Stderr, "Already inside Zellij. Detach first (Ctrl+o d), then run:\n")
		fmt.Fprintf(os.Stderr, "  cc-deck ws attach %s\n", e.name)
		return nil
	}

	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil {
		return err
	}

	// Auto-start stopped workspaces.
	if inst.State == WorkspaceStateStopped {
		if startErr := e.Start(ctx); startErr != nil {
			return fmt.Errorf("auto-starting compose workspace: %w", startErr)
		}
		inst.State = WorkspaceStateRunning
	}

	// Update LastAttached timestamp.
	now := time.Now().UTC()
	inst.LastAttached = &now
	_ = e.store.UpdateInstance(inst)

	cName := e.sessionContainerName()

	if ContainerHasZellijSession(ctx, cName) {
		return podman.Exec(ctx, cName, []string{"zellij", "attach"}, true)
	}
	return podman.Exec(ctx, cName, []string{
		"zellij", "-n", "cc-deck",
	}, true)
}

// Start starts a stopped compose workspace.
func (e *ComposeWorkspace) Start(ctx context.Context) error {
	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil {
		return err
	}

	if err := e.composeCmdOrFallback(ctx, "start"); err != nil {
		return err
	}

	inst.State = WorkspaceStateRunning
	return e.store.UpdateInstance(inst)
}

// Stop stops a running compose workspace.
func (e *ComposeWorkspace) Stop(ctx context.Context) error {
	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil {
		return err
	}

	if err := e.composeCmdOrFallback(ctx, "stop"); err != nil {
		return err
	}

	inst.State = WorkspaceStateStopped
	return e.store.UpdateInstance(inst)
}

// Delete removes the compose workspace and all its resources.
func (e *ComposeWorkspace) Delete(ctx context.Context, force bool) error {
	cName := e.sessionContainerName()

	// Check if running and force not set.
	if !force {
		info, err := podman.Inspect(ctx, cName)
		if err == nil && info != nil && info.Running {
			return ErrRunning
		}
	}

	// Determine project dir from instance or definition.
	projDir := ""
	if inst, err := e.store.FindInstanceByName(e.name); err == nil && inst.Compose != nil {
		projDir = inst.Compose.ProjectDir
	}
	if projDir == "" {
		if def, err := e.defs.FindByName(e.name); err == nil {
			projDir = def.ProjectDir
		}
	}

	// Run compose down (best-effort).
	runDir := ""
	if projDir != "" {
		runDir = filepath.Join(projDir, ccDeckDir, runSubdir)
		composePath := filepath.Join(runDir, composeFile)
		if _, statErr := os.Stat(composePath); statErr == nil {
			if runtime, runtimeErr := compose.Available(); runtimeErr == nil {
				cmdParts := compose.RuntimeCmd(runtime)
				args := append(cmdParts[1:], "-f", composePath, "down")
				cmd := exec.CommandContext(ctx, cmdParts[0], args...)
				cmd.Dir = runDir
				if out, err := cmd.CombinedOutput(); err != nil {
					log.Printf("WARNING: compose down: %v: %s", err, string(out))
				}
			}
		}
	}

	// Remove containers directly (best-effort, in case compose down missed them).
	if err := podman.Remove(ctx, cName, true); err != nil {
		log.Printf("WARNING: removing container %s: %v", cName, err)
	}
	proxyName := e.proxyContainerName()
	if info, _ := podman.Inspect(ctx, proxyName); info != nil {
		if err := podman.Remove(ctx, proxyName, true); err != nil {
			log.Printf("WARNING: removing proxy container %s: %v", proxyName, err)
		}
	}

	// Remove volume if using named-volume.
	vName := volumeName(e.name)
	if podman.VolumeExists(ctx, vName) {
		if err := podman.VolumeRemove(ctx, vName); err != nil {
			log.Printf("WARNING: removing volume: %v", err)
		}
	}

	// Remove generated artifacts (.cc-deck/run/) and status file,
	// but preserve committed files (workspace.yaml, image/, .gitignore).
	if runDir != "" {
		if err := os.RemoveAll(runDir); err != nil {
			log.Printf("WARNING: removing %s: %v", runDir, err)
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

// Status returns the current state for the compose workspace.
func (e *ComposeWorkspace) Status(ctx context.Context) (*WorkspaceStatus, error) {
	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil {
		return nil, err
	}

	cName := e.sessionContainerName()
	state := inst.State

	// Reconcile with actual container state.
	info, inspectErr := podman.Inspect(ctx, cName)
	if inspectErr == nil && info != nil {
		if info.Running {
			state = WorkspaceStateRunning
		} else {
			state = WorkspaceStateStopped
		}
	} else if inspectErr == nil && info == nil {
		state = WorkspaceStateError
	}

	return &WorkspaceStatus{
		State: state,
		Since: inst.CreatedAt,
	}, nil
}

// Exec runs a command inside the session container.
func (e *ComposeWorkspace) Exec(ctx context.Context, cmd []string) error {
	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil {
		return err
	}
	if inst.State != WorkspaceStateRunning {
		return fmt.Errorf("compose workspace is not running (state: %s)", inst.State)
	}

	return podman.Exec(ctx, e.sessionContainerName(), cmd, false)
}

// Push copies local files into the session container.
func (e *ComposeWorkspace) Push(ctx context.Context, opts SyncOpts) error {
	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil {
		return err
	}
	if inst.State != WorkspaceStateRunning {
		return fmt.Errorf("compose workspace is not running (state: %s)", inst.State)
	}

	localPath := opts.LocalPath
	if localPath == "" {
		return fmt.Errorf("local path is required for push")
	}

	remotePath := opts.RemotePath
	if remotePath == "" {
		remotePath = "/workspace/" + baseNameFromPath(localPath)
	}

	return podman.Cp(ctx, localPath, e.sessionContainerName()+":"+remotePath)
}

// Pull copies files from the session container to local storage.
func (e *ComposeWorkspace) Pull(ctx context.Context, opts SyncOpts) error {
	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil {
		return err
	}
	if inst.State != WorkspaceStateRunning {
		return fmt.Errorf("compose workspace is not running (state: %s)", inst.State)
	}

	remotePath := opts.RemotePath
	if remotePath == "" {
		return fmt.Errorf("remote path is required for pull")
	}

	localPath := opts.LocalPath
	if localPath == "" {
		localPath = "."
	}

	return podman.Cp(ctx, e.sessionContainerName()+":"+remotePath, localPath)
}

// Harvest is not supported for compose workspaces.
func (e *ComposeWorkspace) Harvest(_ context.Context, _ HarvestOpts) error {
	return fmt.Errorf("compose workspaces do not support harvest; use push/pull for file transfer: %w", ErrNotSupported)
}

// ReconcileComposeWorkspaces updates the state of all compose workspace
// instances by inspecting their actual container state via podman.
func ReconcileComposeWorkspaces(store *FileStateStore) error {
	instances, err := store.ListInstances(nil)
	if err != nil {
		return err
	}

	for _, inst := range instances {
		if inst.Compose == nil {
			continue
		}

		cName := inst.Compose.ContainerName
		if cName == "" {
			continue
		}

		info, err := podman.Inspect(context.Background(), cName)
		if err != nil {
			continue
		}

		var newState WorkspaceState
		if info == nil {
			newState = WorkspaceStateError
		} else if info.Running {
			newState = WorkspaceStateRunning
		} else {
			newState = WorkspaceStateStopped
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

// composeUp runs podman-compose up -d in the .cc-deck/ directory.
// It first tears down any stale resources from a previous run to avoid
// conflicts with orphaned containers, pods, or networks.
func (e *ComposeWorkspace) composeUp(ctx context.Context, runtime string) error {
	cmdParts := compose.RuntimeCmd(runtime)
	composePath := e.composeFilePath()
	dir := e.composeProjectDir()

	// Tear down stale resources (best-effort).
	downArgs := append(cmdParts[1:], "-f", composePath, "down", "--remove-orphans")
	downCmd := exec.CommandContext(ctx, cmdParts[0], downArgs...)
	downCmd.Dir = dir
	_ = downCmd.Run()

	args := append(cmdParts[1:], "-f", composePath, "up", "-d", "--force-recreate", "--remove-orphans")
	cmd := exec.CommandContext(ctx, cmdParts[0], args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, string(out))
	}
	return nil
}

// composeCmdOrFallback tries compose start/stop, falling back to direct
// podman if the compose file or .cc-deck/ directory is missing.
func (e *ComposeWorkspace) composeCmdOrFallback(ctx context.Context, action string) error {
	runtime, runtimeErr := compose.Available()
	if runtimeErr == nil {
		if err := e.composeCmd(ctx, runtime, action); err == nil {
			return nil
		}
	}

	// Fallback: operate on the session container directly via podman.
	cName := e.sessionContainerName()
	switch action {
	case "start":
		return podman.Start(ctx, cName)
	case "stop":
		return podman.Stop(ctx, cName)
	default:
		return fmt.Errorf("unsupported fallback action: %s", action)
	}
}

// composeCmd runs a compose command (start, stop) on the project.
func (e *ComposeWorkspace) composeCmd(ctx context.Context, runtime string, action string) error {
	// Find compose file path from instance.
	composePath := ""
	if inst, err := e.store.FindInstanceByName(e.name); err == nil && inst.Compose != nil {
		composePath = filepath.Join(inst.Compose.ProjectDir, ccDeckDir, runSubdir, composeFile)
	}
	if composePath == "" {
		if def, err := e.defs.FindByName(e.name); err == nil && def.ProjectDir != "" {
			composePath = filepath.Join(def.ProjectDir, ccDeckDir, runSubdir, composeFile)
		}
	}
	if composePath == "" {
		return fmt.Errorf("cannot find compose file for workspace %q", e.name)
	}

	cmdParts := compose.RuntimeCmd(runtime)
	args := append(cmdParts[1:], "-f", composePath, action)
	cmd := exec.CommandContext(ctx, cmdParts[0], args...)
	cmd.Dir = filepath.Dir(composePath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %s", action, err, string(out))
	}
	return nil
}

// cleanupOnFailure removes the .cc-deck/ directory on creation failure.
func (e *ComposeWorkspace) cleanupOnFailure(ccDeckDir string) {
	if err := os.RemoveAll(ccDeckDir); err != nil {
		log.Printf("WARNING: cleanup failed: %v", err)
	}
}
