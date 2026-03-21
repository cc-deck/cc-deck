package env

import "time"

// EnvironmentType identifies the kind of environment.
type EnvironmentType string

const (
	EnvironmentTypeLocal     EnvironmentType = "local"
	EnvironmentTypeContainer EnvironmentType = "container"
	EnvironmentTypeK8sDeploy EnvironmentType = "k8s-deploy"
	EnvironmentTypeK8sSandbox EnvironmentType = "k8s-sandbox"
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

// SandboxFields holds fields for a K8sSandbox environment.
type SandboxFields struct {
	Namespace  string     `yaml:"namespace,omitempty"`
	PodName    string     `yaml:"pod_name,omitempty"`
	Profile    string     `yaml:"profile,omitempty"`
	Kubeconfig string     `yaml:"kubeconfig,omitempty"`
	ExpiresAt  *time.Time `yaml:"expires_at,omitempty"`
}

// EnvironmentRecord is the persistent representation of an environment
// stored in the state file.
type EnvironmentRecord struct {
	Name         string            `yaml:"name"`
	Type         EnvironmentType   `yaml:"type"`
	State        EnvironmentState  `yaml:"state"`
	CreatedAt    time.Time         `yaml:"created_at"`
	LastAttached *time.Time        `yaml:"last_attached,omitempty"`
	Storage      *StorageConfig    `yaml:"storage,omitempty"`
	Sync         *SyncConfig       `yaml:"sync,omitempty"`
	Container    *ContainerFields   `yaml:"container,omitempty"`
	K8s          *K8sFields        `yaml:"k8s,omitempty"`
	Sandbox      *SandboxFields    `yaml:"sandbox,omitempty"`
}

// EnvironmentInstance is the slim runtime state for a v2 state file.
// It holds only runtime-relevant fields; definition details live in the
// DefinitionStore.
type EnvironmentInstance struct {
	Name         string            `yaml:"name"`
	State        EnvironmentState  `yaml:"state"`
	CreatedAt    time.Time         `yaml:"created_at"`
	LastAttached *time.Time        `yaml:"last_attached,omitempty"`
	Container    *ContainerFields  `yaml:"container,omitempty"`
	K8s          *K8sFields        `yaml:"k8s,omitempty"`
	Sandbox      *SandboxFields    `yaml:"sandbox,omitempty"`
}

// StateFile is the top-level structure of the environment state file.
// Version 1 files use Environments ([]EnvironmentRecord).
// Version 2 files use Instances ([]EnvironmentInstance).
type StateFile struct {
	Version      int                   `yaml:"version"`
	Environments []EnvironmentRecord   `yaml:"environments,omitempty"`
	Instances    []EnvironmentInstance  `yaml:"instances,omitempty"`
}
