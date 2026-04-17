package env

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

// ContainerEnvironment manages a container-based environment using podman.
type ContainerEnvironment struct {
	name  string
	store *FileStateStore
	defs  *DefinitionStore

	Auth        AuthMode
	Ports       []string
	AllPorts    bool
	Credentials map[string]string
	Mounts      []string // Additional bind mounts as "src:dst[:ro]"
	KeepVolumes bool
}

func containerName(envName string) string {
	return containerNamePrefix + envName
}

func volumeName(envName string) string {
	return containerNamePrefix + envName + "-data"
}

func secretName(envName, key string) string {
	return containerNamePrefix + envName + "-" + strings.ToLower(strings.ReplaceAll(key, "_", "-"))
}

// Type returns EnvironmentTypeContainer.
func (e *ContainerEnvironment) Type() EnvironmentType {
	return EnvironmentTypeContainer
}

// Name returns the environment name.
func (e *ContainerEnvironment) Name() string {
	return e.name
}

// Create provisions a new container environment.
func (e *ContainerEnvironment) Create(ctx context.Context, opts CreateOpts) error {
	if err := ValidateEnvName(e.name); err != nil {
		return err
	}

	if !podman.Available() {
		return ErrPodmanNotFound
	}

	// Fail fast if an environment with this name already exists.
	if _, err := e.store.FindInstanceByName(e.name); err == nil {
		return fmt.Errorf("instance %q: %w", e.name, ErrNameConflict)
	}

	// Check for existing definition and use as defaults.
	var def *EnvironmentDefinition
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
	if def != nil && len(def.Repos) > 0 {
		creds := loadActiveGitCredentials()
		workspace := "/workspace"
		if def.Workspace != "" {
			workspace = def.Workspace
		}
		podmanRunner := func(ctx2 context.Context, cmd string) (string, error) {
			return podman.ExecOutput(ctx2, cName, cmd)
		}
		fmt.Fprintf(os.Stderr, "Cloning %d repo(s) into %s...\n", len(def.Repos), workspace)
		cloneRepos(ctx, podmanRunner, def.Repos, workspace, creds, def.ExtraRemotes, def.AutoDetectedURL)
	}

	// Write environment definition.
	if e.defs != nil {
		envDef := &EnvironmentDefinition{
			Name:        e.name,
			Type:        EnvironmentTypeContainer,
			Image:       image,
			Ports:       ports,
			Credentials: credentialKeys,
			Mounts:      e.Mounts,
		}
		if storageType != "" {
			envDef.Storage = &StorageConfig{
				Type:     storageType,
				HostPath: opts.Storage.HostPath,
			}
		}
		if def != nil {
			_ = e.defs.Update(envDef)
		} else {
			_ = e.defs.Add(envDef)
		}
	}

	// Write environment instance to state store.
	inst := &EnvironmentInstance{
		Name:      e.name,
		Type:      EnvironmentTypeContainer,
		State:     EnvironmentStateRunning,
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
// Mirrors the local environment pattern: check for nested Zellij,
// create session with cc-deck layout if needed, then attach.
func (e *ContainerEnvironment) Attach(ctx context.Context) error {
	// Inside Zellij on the host: cannot nest sessions.
	if os.Getenv("ZELLIJ") != "" {
		fmt.Fprintf(os.Stderr, "Already inside Zellij. Detach first (Ctrl+o d), then run:\n")
		fmt.Fprintf(os.Stderr, "  cc-deck env attach %s\n", e.name)
		return nil
	}

	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil {
		return err
	}

	// Auto-start stopped containers (FR-018).
	if inst.State == EnvironmentStateStopped {
		if startErr := e.Start(ctx); startErr != nil {
			return fmt.Errorf("auto-starting container: %w", startErr)
		}
		inst.State = EnvironmentStateRunning
	}

	// Update LastAttached timestamp.
	now := time.Now().UTC()
	inst.LastAttached = &now
	_ = e.store.UpdateInstance(inst)

	cName := containerName(e.name)

	// If any Zellij session exists inside the container, attach to it.
	// Otherwise, create a new session using the cc-deck layout (sidebar plugin).
	// Uses -n (--new-session-with-layout) which reliably starts with the layout.
	if ContainerHasZellijSession(ctx, cName) {
		return podman.Exec(ctx, cName, []string{"zellij", "attach"}, true)
	}
	return podman.Exec(ctx, cName, []string{
		"zellij", "-n", "cc-deck",
	}, true)
}


// Delete removes the container and its resources.
func (e *ContainerEnvironment) Delete(ctx context.Context, force bool) error {
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
func (e *ContainerEnvironment) Start(ctx context.Context) error {
	if err := podman.Start(ctx, containerName(e.name)); err != nil {
		return err
	}

	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil {
		return err
	}
	inst.State = EnvironmentStateRunning
	return e.store.UpdateInstance(inst)
}

// Stop stops a running container.
func (e *ContainerEnvironment) Stop(ctx context.Context) error {
	if err := podman.Stop(ctx, containerName(e.name)); err != nil {
		return err
	}

	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil {
		return err
	}
	inst.State = EnvironmentStateStopped
	return e.store.UpdateInstance(inst)
}

// Status returns the current state and metadata for the container.
func (e *ContainerEnvironment) Status(ctx context.Context) (*EnvironmentStatus, error) {
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
			state = EnvironmentStateRunning
		} else {
			state = EnvironmentStateStopped
		}
	} else if inspectErr == nil && info == nil {
		state = EnvironmentStateError
	}

	status := &EnvironmentStatus{
		State: state,
		Since: inst.CreatedAt,
	}

	return status, nil
}

// Exec runs a command inside the container.
func (e *ContainerEnvironment) Exec(ctx context.Context, cmd []string) error {
	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil {
		return err
	}
	if inst.State != EnvironmentStateRunning {
		return fmt.Errorf("container is not running (state: %s)", inst.State)
	}

	return podman.Exec(ctx, containerName(e.name), cmd, false)
}

// Push copies local files into the container.
func (e *ContainerEnvironment) Push(ctx context.Context, opts SyncOpts) error {
	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil {
		return err
	}
	if inst.State != EnvironmentStateRunning {
		return fmt.Errorf("container is not running (state: %s)", inst.State)
	}

	localPath := opts.LocalPath
	if localPath == "" {
		return fmt.Errorf("local path is required for push")
	}

	remotePath := opts.RemotePath
	if remotePath == "" {
		remotePath = "/workspace/" + baseNameFromPath(localPath)
	}

	return podman.Cp(ctx, localPath, containerName(e.name)+":"+remotePath)
}

// Pull copies files from the container to local storage.
func (e *ContainerEnvironment) Pull(ctx context.Context, opts SyncOpts) error {
	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil {
		return err
	}
	if inst.State != EnvironmentStateRunning {
		return fmt.Errorf("container is not running (state: %s)", inst.State)
	}

	remotePath := opts.RemotePath
	if remotePath == "" {
		return fmt.Errorf("remote path is required for pull")
	}

	localPath := opts.LocalPath
	if localPath == "" {
		localPath = "."
	}

	return podman.Cp(ctx, containerName(e.name)+":"+remotePath, localPath)
}

// Harvest is not supported for container environments. Use push/pull instead.
func (e *ContainerEnvironment) Harvest(_ context.Context, _ HarvestOpts) error {
	return fmt.Errorf("container environments do not support harvest; use push/pull for file transfer, or use --type compose for multi-container setups: %w", ErrNotSupported)
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

// ReconcileContainerEnvs updates the state of all container environment
// instances by inspecting their actual container state via podman.
func ReconcileContainerEnvs(store *FileStateStore, defs *DefinitionStore) error {
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

		var newState EnvironmentState
		if info == nil {
			newState = EnvironmentStateError
		} else if info.Running {
			newState = EnvironmentStateRunning
		} else {
			newState = EnvironmentStateStopped
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
// naming convention for the given environment name. Used when state records
// are missing but podman resources still exist. Returns true if any
// resources were found and cleaned up.
func CleanupOrphanedContainer(ctx context.Context, envName string, keepVolumes bool) bool {
	cName := containerName(envName)
	vName := volumeName(envName)
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


