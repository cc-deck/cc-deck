package agent

// CredentialSpec represents one auth mode for an agent. Each agent declares
// one or more specs via CredentialSpecs(). The credential package uses these
// to detect available modes, resolve values, and inject into workspaces.
type CredentialSpec struct {
	Name           string              `json:"name"`
	EnvVars        []EnvVarSpec        `json:"env_vars,omitempty"`
	FileCredential *FileCredentialSpec `json:"file_credential,omitempty"`
	Endpoints      []Endpoint          `json:"endpoints,omitempty"`
	UnsetVars      []string            `json:"unset_vars,omitempty"`
	Priority       int                 `json:"priority"`
}

// EnvVarSpec describes a single environment variable within a credential spec.
type EnvVarSpec struct {
	Name       string `json:"name"`
	FixedValue string `json:"fixed_value,omitempty"`
	Required   bool   `json:"required,omitempty"`
}

// FileCredentialSpec describes a file-based credential within a credential spec.
type FileCredentialSpec struct {
	EnvVar      string `json:"env_var"`
	DefaultPath string `json:"default_path,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// Endpoint is a network endpoint needed by an auth mode.
type Endpoint struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}
