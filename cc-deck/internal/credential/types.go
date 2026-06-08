package credential

import "github.com/cc-deck/cc-deck/internal/agent"

// AvailableMode pairs a credential spec with its resolved values from the
// host environment. Returned by Detect() for specs whose required credentials
// are all present.
type AvailableMode struct {
	Spec   agent.CredentialSpec
	Values map[string]string
}

// ResolvedCredentials holds the fully resolved credential set for a single
// auth mode, ready for injection into a workspace.
type ResolvedCredentials struct {
	EnvVars        map[string]string
	FileCredential *ResolvedFile
	UnsetVars      []string
}

// ResolvedFile describes a file-based credential with its resolved local path
// and the env var that should point to it in the workspace.
type ResolvedFile struct {
	EnvVar    string
	LocalPath string
}
