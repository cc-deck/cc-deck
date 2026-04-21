package ws

import "time"

// WorkspaceType identifies the kind of workspace.
type WorkspaceType string

const (
	WorkspaceTypeLocal     WorkspaceType = "local"
	WorkspaceTypeContainer WorkspaceType = "container"
	WorkspaceTypeCompose    WorkspaceType = "compose"
	WorkspaceTypeK8sDeploy  WorkspaceType = "k8s-deploy"
	WorkspaceTypeK8sSandbox WorkspaceType = "k8s-sandbox"
	WorkspaceTypeSSH        WorkspaceType = "ssh"
)

// WorkspaceState represents the current state of a workspace.
type WorkspaceState string

const (
	WorkspaceStateRunning   WorkspaceState = "running"
	WorkspaceStateStopped   WorkspaceState = "stopped"
	WorkspaceStateAvailable WorkspaceState = "available"
	WorkspaceStateCreating  WorkspaceState = "creating"
	WorkspaceStateError     WorkspaceState = "error"
	WorkspaceStateUnknown   WorkspaceState = "unknown"
)

// StorageType identifies the storage backend for a workspace.
type StorageType string

const (
	StorageTypeHostPath    StorageType = "host-path"
	StorageTypeNamedVolume StorageType = "named-volume"
	StorageTypeEmptyDir    StorageType = "empty-dir"
	StorageTypePVC         StorageType = "pvc"
)

// SyncStrategy identifies how workspace synchronization is performed.
type SyncStrategy string

const (
	SyncStrategyCopy       SyncStrategy = "copy"
	SyncStrategyGitHarvest SyncStrategy = "git-harvest"
	SyncStrategyRemoteGit  SyncStrategy = "remote-git"
)

// StorageConfig describes the storage backend for a workspace.
type StorageConfig struct {
	Type         StorageType `yaml:"type"`
	Size         string      `yaml:"size,omitempty"`
	StorageClass string      `yaml:"storage_class,omitempty"`
	HostPath     string      `yaml:"host_path,omitempty"`
}

// SyncConfig describes workspace synchronization settings.
type SyncConfig struct {
	Strategy    SyncStrategy `yaml:"strategy"`
	Workspace   string       `yaml:"workspace,omitempty"`
	Excludes    []string     `yaml:"excludes,omitempty"`
	LastPush    *time.Time   `yaml:"last_push,omitempty"`
	LastHarvest *time.Time   `yaml:"last_harvest,omitempty"`
}

// K8sFields holds Kubernetes-specific fields for a K8sDeploy workspace.
type K8sFields struct {
	Namespace   string `yaml:"namespace,omitempty"`
	StatefulSet string `yaml:"stateful_set,omitempty"`
	Profile     string `yaml:"profile,omitempty"`
	Kubeconfig  string `yaml:"kubeconfig,omitempty"`
}

// ContainerFields holds container-specific fields for a container workspace.
type ContainerFields struct {
	ContainerID   string   `yaml:"container_id,omitempty"`
	ContainerName string   `yaml:"container_name,omitempty"`
	Image         string   `yaml:"image,omitempty"`
	Ports         []string `yaml:"ports,omitempty"`
}

// ComposeFields holds compose-specific fields for a compose workspace.
type ComposeFields struct {
	ProjectDir    string `yaml:"project_dir"`
	ContainerName string `yaml:"container_name"`
	ProxyName     string `yaml:"proxy_name,omitempty"`
}

// SSHFields holds SSH-specific fields for an SSH workspace.
type SSHFields struct {
	Host         string `yaml:"host"`
	Port         int    `yaml:"port,omitempty"`
	IdentityFile string `yaml:"identity_file,omitempty"`
	JumpHost     string `yaml:"jump_host,omitempty"`
	SSHConfig    string `yaml:"ssh_config,omitempty"`
	Workspace    string `yaml:"workspace,omitempty"`
}

// SandboxFields holds fields for a K8sSandbox workspace.
type SandboxFields struct {
	Namespace  string     `yaml:"namespace,omitempty"`
	PodName    string     `yaml:"pod_name,omitempty"`
	Profile    string     `yaml:"profile,omitempty"`
	Kubeconfig string     `yaml:"kubeconfig,omitempty"`
	ExpiresAt  *time.Time `yaml:"expires_at,omitempty"`
}

// WorkspaceInstance is the runtime state for a workspace.
// Definition details (storage, sync) live in the DefinitionStore.
type WorkspaceInstance struct {
	Name         string            `yaml:"name"`
	Type         WorkspaceType   `yaml:"type"`
	State        WorkspaceState  `yaml:"state"`
	CreatedAt    time.Time         `yaml:"created_at"`
	LastAttached *time.Time        `yaml:"last_attached,omitempty"`
	Container    *ContainerFields  `yaml:"container,omitempty"`
	Compose      *ComposeFields    `yaml:"compose,omitempty"`
	K8s          *K8sFields        `yaml:"k8s,omitempty"`
	Sandbox      *SandboxFields    `yaml:"sandbox,omitempty"`
	SSH          *SSHFields        `yaml:"ssh,omitempty"`
}

// ProjectEntry is a global registry entry for a project directory
// stored in state.yaml under the projects section.
type ProjectEntry struct {
	Path     string    `yaml:"path"`
	LastSeen time.Time `yaml:"last_seen"`
}

// ProjectStatusFile holds per-project runtime state stored at
// .cc-deck/status.yaml (gitignored).
type ProjectStatusFile struct {
	Variant       string            `yaml:"variant,omitempty"`
	State         WorkspaceState  `yaml:"state"`
	ContainerName string            `yaml:"container_name"`
	CreatedAt     time.Time         `yaml:"created_at"`
	LastAttached  *time.Time        `yaml:"last_attached,omitempty"`
	Overrides     map[string]string `yaml:"overrides,omitempty"`
}

// StateFile is the top-level structure of the workspace state file.
type StateFile struct {
	Version   int                   `yaml:"version"`
	Instances []WorkspaceInstance `yaml:"instances,omitempty"`
	Projects  []ProjectEntry       `yaml:"projects,omitempty"`
}
