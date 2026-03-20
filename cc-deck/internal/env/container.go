package env

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/cc-deck/cc-deck/internal/podman"
)

const (
	containerNamePrefix = "cc-deck-"
	defaultImage        = "quay.io/cc-deck/cc-deck-demo:latest"
)

// ContainerEnvironment manages a container-based environment using podman.
type ContainerEnvironment struct {
	name  string
	store *FileStateStore
	defs  *DefinitionStore

	Ports       []string
	AllPorts    bool
	Credentials map[string]string
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

	cName := containerName(e.name)

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
		if hostPath != "" {
			volumes = append(volumes, hostPath+":/workspace")
		}
	}

	// Build run options.
	runOpts := podman.RunOpts{
		Name:     cName,
		Image:    image,
		Volumes:  volumes,
		Ports:    ports,
		AllPorts: e.AllPorts,
		Cmd:      []string{"sleep", "infinity"},
	}

	containerID, err := podman.Run(ctx, runOpts)
	if err != nil {
		return fmt.Errorf("creating container: %w", err)
	}

	// Write environment definition.
	if e.defs != nil {
		envDef := &EnvironmentDefinition{
			Name:  e.name,
			Type:  EnvironmentTypeContainer,
			Image: image,
			Ports: ports,
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

// Attach opens an interactive session into the container.
func (e *ContainerEnvironment) Attach(ctx context.Context) error {
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

	return podman.Exec(ctx, containerName(e.name), []string{"zellij", "attach", "cc-deck", "--create"}, true)
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
		state = EnvironmentStateUnknown
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
	return fmt.Errorf("container environments do not support harvest; use push/pull for file transfer: %w", ErrNotSupported)
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
	instances, err := store.ListInstances()
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
			newState = EnvironmentStateUnknown
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
