package ws

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/cc-deck/cc-deck/internal/podman"
)

const (
	containerNamePrefix = "cc-deck-"
	defaultImage        = "quay.io/cc-deck/cc-deck-demo:latest"
)

// AuthMode controls how Claude Code authentication is detected and injected.
type AuthMode string

const (
	AuthModeAuto    AuthMode = "auto"    // Detect from host environment (default)
	AuthModeNone    AuthMode = "none"    // No auth passthrough
	AuthModeAPI     AuthMode = "api"     // Direct Anthropic API key
	AuthModeVertex  AuthMode = "vertex"  // Google Vertex AI
	AuthModeBedrock AuthMode = "bedrock" // Amazon Bedrock
)

// ContainerWorkspace manages a container-based workspace using podman.
type ContainerWorkspace struct {
	name  string
	store *FileStateStore
	defs  *DefinitionStore

	Auth        AuthMode
	Ports       []string
	AllPorts    bool
	Credentials map[string]string
	Mounts      []string // Additional bind mounts as "src:dst[:ro]"
	KeepVolumes bool

	Repos           []RepoEntry
	ExtraRemotes    map[string]string
	AutoDetectedURL string

	pipeCh PipeChannel
	dataCh DataChannel
	gitCh  GitChannel
}

func containerName(wsName string) string {
	return containerNamePrefix + wsName
}

func volumeName(wsName string) string {
	return containerNamePrefix + wsName + "-data"
}

func secretName(wsName, key string) string {
	return containerNamePrefix + wsName + "-" + strings.ToLower(strings.ReplaceAll(key, "_", "-"))
}

// Type returns WorkspaceTypeContainer.
func (e *ContainerWorkspace) Type() WorkspaceType {
	return WorkspaceTypeContainer
}

// Name returns the workspace name.
func (e *ContainerWorkspace) Name() string {
	return e.name
}

// Create provisions a new container workspace.
func (e *ContainerWorkspace) Create(ctx context.Context, opts CreateOpts) error {
	if err := ValidateWsName(e.name); err != nil {
		return err
	}

	if !podman.Available() {
		return ErrPodmanNotFound
	}

	// Fail fast if a workspace with this name already exists.
	if _, err := e.store.FindInstanceByName(e.name); err == nil {
		return fmt.Errorf("instance %q: %w", e.name, ErrNameConflict)
	}

	// Check for existing definition and use as defaults.
	var def *WorkspaceDefinition
	if e.defs != nil {
		if d, err := e.defs.FindByName(e.name); err == nil {
			def = d
		}
	}

	// Resolve image: explicit opts > definition > config default > hardcoded.
	image := opts.Image
	if image == "" && def != nil && def.Image != "" {
		image = def.Image
	}
	if image == "" {
		log.Printf("WARNING: no image specified, using default %s", defaultImage)
		image = defaultImage
	}

	// Resolve storage type.
	storageType := opts.Storage.Type
	if storageType == "" && def != nil && def.Storage != nil {
		storageType = def.Storage.Type
	}
	if storageType == "" {
		storageType = StorageTypeNamedVolume
	}

	// Resolve ports from definition if not set explicitly.
	ports := e.Ports
	if len(ports) == 0 && def != nil {
		ports = def.Ports
	}

	// Resolve credentials: explicit --credential flags first.
	creds := e.Credentials
	if creds == nil {
		creds = make(map[string]string)
	}

	// Resolve credentials from definition keys (fill from host env).
	if def != nil {
		for _, key := range def.Credentials {
			if _, exists := creds[key]; !exists {
				if val := os.Getenv(key); val != "" {
					creds[key] = val
				}
			}
		}
	}

	// Auth mode: CLI flag > definition > auto-detect from host env.
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

	cName := containerName(e.name)

	// Remove any existing container with the same name (orphaned from a
	// previous run or failed cleanup).
	if info, _ := podman.Inspect(ctx, cName); info != nil {
		_ = podman.Remove(ctx, cName, true)
	}

	// Create volume if using named-volume storage.
	var volumes []string
	switch storageType {
	case StorageTypeNamedVolume:
		vName := volumeName(e.name)
		if err := podman.VolumeCreate(ctx, vName); err != nil {
			return fmt.Errorf("creating volume: %w", err)
		}
		volumes = append(volumes, vName+":/workspace")
	case StorageTypeHostPath:
		hostPath := opts.Storage.HostPath
		if hostPath == "" && def != nil && def.Storage != nil {
			hostPath = def.Storage.HostPath
		}
		if hostPath == "" {
			hostPath, _ = os.Getwd()
		}
		if hostPath != "" {
			volumes = append(volumes, hostPath+":/workspace")
		}
	}

	// Create podman secrets for credentials.
	var secrets []podman.SecretMount
	var envs []string
	var credentialKeys []string
	for key, val := range creds {
		sName := secretName(e.name, key)

		// Detect file-based credentials: if the value is a path to an
		// existing file, read the file content into the secret and mount
		// it as a file at /run/secrets/<name>. Set the env var to point
		// to the mounted file path.
		if info, statErr := os.Stat(val); statErr == nil && !info.IsDir() {
			data, readErr := os.ReadFile(val)
			if readErr != nil {
				return fmt.Errorf("reading credential file %q: %w", val, readErr)
			}
			if err := podman.SecretCreate(ctx, sName, data); err != nil {
				return fmt.Errorf("creating secret %q: %w", key, err)
			}
			secrets = append(secrets, podman.SecretMount{
				Name:   sName,
				AsFile: true,
			})
			envs = append(envs, fmt.Sprintf("%s=/run/secrets/%s", key, sName))
		} else {
			// Plain value: inject as env var via podman secret.
			if err := podman.SecretCreate(ctx, sName, []byte(val)); err != nil {
				return fmt.Errorf("creating secret %q: %w", key, err)
			}
			secrets = append(secrets, podman.SecretMount{
				Name:   sName,
				Target: key,
			})
		}
		credentialKeys = append(credentialKeys, key)
	}

	// Add mounts from definition (if not overridden by CLI).
	if len(e.Mounts) == 0 && def != nil {
		e.Mounts = def.Mounts
	}
	for _, m := range e.Mounts {
		volumes = append(volumes, m)
	}

	// Build run options.
	runOpts := podman.RunOpts{
		Name:     cName,
		Image:    image,
		Volumes:  volumes,
		Secrets:  secrets,
		Ports:    ports,
		AllPorts: e.AllPorts,
		Envs:     envs,
		Cmd:      []string{"sleep", "infinity"},
	}

	containerID, err := podman.Run(ctx, runOpts)
	if err != nil {
		return fmt.Errorf("creating container: %w", err)
	}

	// Clone repos into workspace if defined.
	if len(e.Repos) > 0 {
		creds := loadActiveGitCredentials()
		workspace := "/workspace"
		podmanRunner := func(ctx2 context.Context, cmd string) (string, error) {
			return podman.ExecOutput(ctx2, cName, cmd)
		}
		fmt.Fprintf(os.Stderr, "Cloning %d repo(s) into %s...\n", len(e.Repos), workspace)
		cloneRepos(ctx, podmanRunner, e.Repos, workspace, creds, e.ExtraRemotes, e.AutoDetectedURL)
	}

	// Update workspace definition with resolved runtime values.
	if e.defs != nil {
		if def != nil {
			def.Image = image
			def.Ports = ports
			def.Credentials = credentialKeys
			def.Mounts = e.Mounts
			if storageType != "" {
				def.Storage = &StorageConfig{
					Type:     storageType,
					HostPath: opts.Storage.HostPath,
				}
			}
			_ = e.defs.Update(def)
		} else {
			wsDef := &WorkspaceDefinition{
				Name: e.name,
				Type: WorkspaceTypeContainer,
				WorkspaceSpec: WorkspaceSpec{
					Image:       image,
					Ports:       ports,
					Credentials: credentialKeys,
					Mounts:      e.Mounts,
				},
			}
			if storageType != "" {
				wsDef.Storage = &StorageConfig{
					Type:     storageType,
					HostPath: opts.Storage.HostPath,
				}
			}
			_ = e.defs.Add(wsDef)
		}
	}

	// Write workspace instance to state store.
	inst := &WorkspaceInstance{
		Name:      e.name,
		Type:      WorkspaceTypeContainer,
		State:     WorkspaceStateRunning,
		CreatedAt: time.Now().UTC(),
		Container: &ContainerFields{
			ContainerID:   containerID,
			ContainerName: cName,
			Image:         image,
			Ports:         ports,
		},
	}

	return e.store.AddInstance(inst)
}

// Attach opens an interactive Zellij session inside the container.
// Mirrors the local workspace pattern: check for nested Zellij,
// create session with cc-deck layout if needed, then attach.
func (e *ContainerWorkspace) Attach(ctx context.Context) error {
	// Inside Zellij on the host: cannot nest sessions.
	if os.Getenv("ZELLIJ") != "" {
		fmt.Fprintf(os.Stderr, "Already inside Zellij. Detach first (Ctrl+o d), then run:\n")
		fmt.Fprintf(os.Stderr, "  cc-deck ws attach %s\n", e.name)
		return nil
	}

	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil {
		return err
	}

	// Auto-start stopped containers (FR-018).
	if inst.State == WorkspaceStateStopped {
		if startErr := e.Start(ctx); startErr != nil {
			return fmt.Errorf("auto-starting container: %w", startErr)
		}
		inst.State = WorkspaceStateRunning
	}

	// Update LastAttached timestamp.
	now := time.Now().UTC()
	inst.LastAttached = &now
	_ = e.store.UpdateInstance(inst)

	cName := containerName(e.name)

	// Set terminal background color for remote sessions if configured.
	remoteBG := LoadRemoteBG(e.name, e.defs)
	if remoteBG != "" {
		SetRemoteBG(remoteBG)
	}

	// If any Zellij session exists inside the container, attach to it.
	// Otherwise, create a new session using the cc-deck layout (sidebar plugin).
	// Uses -n (--new-session-with-layout) which reliably starts with the layout.
	if ContainerHasZellijSession(ctx, cName) {
		return podman.ExecWithCleanup(ctx, cName, []string{"zellij", "attach"}, ResetBGEscape)
	}
	return podman.ExecWithCleanup(ctx, cName, []string{
		"zellij", "-n", "cc-deck",
	}, ResetBGEscape)
}


// Delete removes the container and its resources.
func (e *ContainerWorkspace) Delete(ctx context.Context, force bool) error {
	cName := containerName(e.name)

	// Check if running and force not set.
	if !force {
		info, err := podman.Inspect(ctx, cName)
		if err != nil {
			return err
		}
		if info != nil && info.Running {
			return ErrRunning
		}
	}

	// Remove container (best-effort, force).
	if err := podman.Remove(ctx, cName, true); err != nil {
		log.Printf("WARNING: removing container %s: %v", cName, err)
	}

	// Remove volume if not keeping.
	if !e.KeepVolumes {
		if err := podman.VolumeRemove(ctx, volumeName(e.name)); err != nil {
			log.Printf("WARNING: removing volume: %v", err)
		}
	}

	// Remove secrets (best-effort, iterate definition credentials).
	if e.defs != nil {
		if def, err := e.defs.FindByName(e.name); err == nil {
			for _, key := range def.Credentials {
				if err := podman.SecretRemove(ctx, secretName(e.name, key)); err != nil {
					log.Printf("WARNING: removing secret %s: %v", key, err)
				}
			}
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

// Start starts a stopped container.
func (e *ContainerWorkspace) Start(ctx context.Context) error {
	if err := podman.Start(ctx, containerName(e.name)); err != nil {
		return err
	}

	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil {
		return err
	}
	inst.State = WorkspaceStateRunning
	return e.store.UpdateInstance(inst)
}

// Stop stops a running container.
func (e *ContainerWorkspace) Stop(ctx context.Context) error {
	if err := podman.Stop(ctx, containerName(e.name)); err != nil {
		return err
	}

	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil {
		return err
	}
	inst.State = WorkspaceStateStopped
	return e.store.UpdateInstance(inst)
}

// Status returns the current state and metadata for the container.
func (e *ContainerWorkspace) Status(ctx context.Context) (*WorkspaceStatus, error) {
	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil {
		return nil, err
	}

	cName := containerName(e.name)
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

	status := &WorkspaceStatus{
		State: state,
		Since: inst.CreatedAt,
	}

	return status, nil
}

// Exec runs a command inside the container.
func (e *ContainerWorkspace) Exec(ctx context.Context, cmd []string) error {
	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil {
		return err
	}
	if inst.State != WorkspaceStateRunning {
		return fmt.Errorf("container is not running (state: %s)", inst.State)
	}

	return podman.Exec(ctx, containerName(e.name), cmd, false)
}

// ExecOutput runs a command inside the container and returns stdout.
func (e *ContainerWorkspace) ExecOutput(ctx context.Context, cmd []string) (string, error) {
	return podman.ExecOutput(ctx, containerName(e.name), strings.Join(cmd, " "))
}

// PipeChannel returns the pipe channel for this workspace.
func (e *ContainerWorkspace) PipeChannel(_ context.Context) (PipeChannel, error) {
	if e.pipeCh == nil {
		e.pipeCh = &execPipeChannel{name: e.name, execFn: e.Exec}
	}
	return e.pipeCh, nil
}

// DataChannel returns the data channel for this workspace.
func (e *ContainerWorkspace) DataChannel(_ context.Context) (DataChannel, error) {
	if e.dataCh == nil {
		e.dataCh = &podmanDataChannel{
			name:          e.name,
			containerName: func() string { return containerName(e.name) },
		}
	}
	return e.dataCh, nil
}

// GitChannel returns the git channel for this workspace.
func (e *ContainerWorkspace) GitChannel(_ context.Context) (GitChannel, error) {
	if e.gitCh == nil {
		e.gitCh = &podmanGitChannel{
			name:          e.name,
			containerName: func() string { return containerName(e.name) },
			workspacePath: "/workspace",
		}
	}
	return e.gitCh, nil
}

// Push copies local files into the container via DataChannel.
func (e *ContainerWorkspace) Push(ctx context.Context, opts SyncOpts) error {
	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil {
		return err
	}
	if inst.State != WorkspaceStateRunning {
		return fmt.Errorf("container is not running (state: %s)", inst.State)
	}

	ch, chErr := e.DataChannel(ctx)
	if chErr != nil {
		return chErr
	}
	return ch.Push(ctx, opts)
}

// Pull copies files from the container to local storage via DataChannel.
func (e *ContainerWorkspace) Pull(ctx context.Context, opts SyncOpts) error {
	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil {
		return err
	}
	if inst.State != WorkspaceStateRunning {
		return fmt.Errorf("container is not running (state: %s)", inst.State)
	}

	ch, chErr := e.DataChannel(ctx)
	if chErr != nil {
		return chErr
	}
	return ch.Pull(ctx, opts)
}

// Harvest extracts git commits from the container via GitChannel.
func (e *ContainerWorkspace) Harvest(ctx context.Context, opts HarvestOpts) error {
	ch, err := e.GitChannel(ctx)
	if err != nil {
		return err
	}
	return ch.Fetch(ctx, opts)
}

// baseNameFromPath returns the last element of a path, handling both
// forward and backward slashes.
func baseNameFromPath(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[i+1:]
		}
	}
	return path
}

// ReconcileContainerWorkspaces updates the state of all container workspace
// instances by inspecting their actual container state via podman.
func ReconcileContainerWorkspaces(store *FileStateStore, defs *DefinitionStore) error {
	instances, err := store.ListInstances(nil)
	if err != nil {
		return err
	}

	for _, inst := range instances {
		if inst.Container == nil {
			continue
		}

		cName := inst.Container.ContainerName
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

// CleanupOrphanedContainer removes podman resources that match the cc-deck
// naming convention for the given workspace name. Used when state records
// are missing but podman resources still exist. Returns true if any
// resources were found and cleaned up.
func CleanupOrphanedContainer(ctx context.Context, wsName string, keepVolumes bool) bool {
	cName := containerName(wsName)
	vName := volumeName(wsName)
	cleaned := false

	// Check if container exists.
	if info, _ := podman.Inspect(ctx, cName); info != nil {
		_ = podman.Remove(ctx, cName, true)
		cleaned = true
	}

	// Check if volume exists.
	if !keepVolumes && podman.VolumeExists(ctx, vName) {
		_ = podman.VolumeRemove(ctx, vName)
		cleaned = true
	}

	return cleaned
}


