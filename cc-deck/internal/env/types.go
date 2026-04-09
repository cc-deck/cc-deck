package env

import "time"

// EnvironmentType identifies the kind of environment.
type EnvironmentType string

const (
	EnvironmentTypeLocal     EnvironmentType = "local"
	EnvironmentTypeContainer EnvironmentType = "container"
	EnvironmentTypeCompose    EnvironmentType = "compose"
	EnvironmentTypeK8sDeploy  EnvironmentType = "k8s-deploy"
	EnvironmentTypeK8sSandbox EnvironmentType = "k8s-sandbox"
	EnvironmentTypeSSH        EnvironmentType = "ssh"
)

// EnvironmentState represents the current state of an environment.
type EnvironmentState string

const (
	EnvironmentStateRunning  EnvironmentState = "running"
	EnvironmentStateStopped  EnvironmentState = "stopped"
	EnvironmentStateCreating EnvironmentState = "creating"
	EnvironmentStateError    EnvironmentState = "error"
	EnvironmentStateUnknown  EnvironmentState = "unknown"
)

// StorageType identifies the storage backend for an environment.
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

// StorageConfig describes the storage backend for an environment.
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

// K8sFields holds Kubernetes-specific fields for a K8sDeploy environment.
type K8sFields struct {
	Namespace   string `yaml:"namespace,omitempty"`
	StatefulSet string `yaml:"stateful_set,omitempty"`
	Profile     string `yaml:"profile,omitempty"`
	Kubeconfig  string `yaml:"kubeconfig,omitempty"`
}

// ContainerFields holds container-specific fields for a container environment.
type ContainerFields struct {
	ContainerID   string   `yaml:"container_id,omitempty"`
	ContainerName string   `yaml:"container_name,omitempty"`
	Image         string   `yaml:"image,omitempty"`
	Ports         []string `yaml:"ports,omitempty"`
}

// ComposeFields holds compose-specific fields for a compose environment.
type ComposeFields struct {
	ProjectDir    string `yaml:"project_dir"`
	ContainerName string `yaml:"container_name"`
	ProxyName     string `yaml:"proxy_name,omitempty"`
}

// SSHFields holds SSH-specific fields for an SSH environment.
type SSHFields struct {
	Host         string `yaml:"host"`
	Port         int    `yaml:"port,omitempty"`
	IdentityFile string `yaml:"identity_file,omitempty"`
	JumpHost     string `yaml:"jump_host,omitempty"`
	SSHConfig    string `yaml:"ssh_config,omitempty"`
	Workspace    string `yaml:"workspace,omitempty"`
}

// SandboxFields holds fields for a K8sSandbox environment.
type SandboxFields struct {
	Namespace  string     `yaml:"namespace,omitempty"`
	PodName    string     `yaml:"pod_name,omitempty"`
	Profile    string     `yaml:"profile,omitempty"`
	Kubeconfig string     `yaml:"kubeconfig,omitempty"`
	ExpiresAt  *time.Time `yaml:"expires_at,omitempty"`
}

// EnvironmentInstance is the runtime state for an environment.
// Definition details (storage, sync) live in the DefinitionStore.
type EnvironmentInstance struct {
	Name         string            `yaml:"name"`
	Type         EnvironmentType   `yaml:"type"`
	State        EnvironmentState  `yaml:"state"`
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
	State         EnvironmentState  `yaml:"state"`
	ContainerName string            `yaml:"container_name"`
	CreatedAt     time.Time         `yaml:"created_at"`
	LastAttached  *time.Time        `yaml:"last_attached,omitempty"`
	Overrides     map[string]string `yaml:"overrides,omitempty"`
}

// StateFile is the top-level structure of the environment state file.
type StateFile struct {
	Version   int                   `yaml:"version"`
	Instances []EnvironmentInstance `yaml:"instances,omitempty"`
	Projects  []ProjectEntry       `yaml:"projects,omitempty"`
}
