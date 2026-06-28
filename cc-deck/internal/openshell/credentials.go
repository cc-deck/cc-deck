package openshell

import (
	"context"
	"fmt"
	"log"
	"os"
)

// KnownProviderProfile maps a credential type to its OpenShell provider profile
// and the environment variables used for detection and resolution.
type KnownProviderProfile struct {
	Type         string
	DetectVars   []string
	RequiredVars []string
	ExtraEnvVars []string
	FileVar      string
	Endpoints    []ProviderEndpoint
}

// ProviderEndpoint is a host:port pair for network policy generation.
type ProviderEndpoint struct {
	Host string
	Port int
}

// ProviderConfig holds the resolved configuration for creating an OpenShell provider.
type ProviderConfig struct {
	Name           string
	Type           string
	FromExisting   bool
	SkipProvider   bool
	Credentials    map[string]string
	FileVar        string
	FilePath       string
	EnvVarsToInject map[string]string
}

// CredentialInput represents a credential entry passed from the manifest layer.
// This avoids a circular import with the build package.
type CredentialInput struct {
	Type    string
	EnvVars []string
	File    string
}

// DetectedCredential represents a credential detected from the host environment.
type DetectedCredential struct {
	Type    string
	EnvVars []string
	File    string
}

// Deprecated: KnownProviderProfiles is superseded by agent-declared CredentialSpecs.
// Use credential.DetectAll() for multi-agent credential detection.
var KnownProviderProfiles = map[string]KnownProviderProfile{
	"claude": {
		Type:         "claude",
		DetectVars:   []string{"ANTHROPIC_API_KEY"},
		RequiredVars: []string{"ANTHROPIC_API_KEY"},
	},
	"claude-vertex": {
		Type:         "google-cloud",
		DetectVars:   []string{"CLAUDE_CODE_USE_VERTEX", "ANTHROPIC_VERTEX_PROJECT_ID"},
		RequiredVars: []string{"ANTHROPIC_VERTEX_PROJECT_ID"},
		ExtraEnvVars: []string{"CLOUD_ML_REGION", "ANTHROPIC_MODEL"},
	},
	"anthropic": {
		Type:         "anthropic",
		DetectVars:   []string{"ANTHROPIC_API_KEY"},
		RequiredVars: []string{"ANTHROPIC_API_KEY"},
	},
	"github": {
		Type:         "github",
		DetectVars:   []string{"GITHUB_TOKEN", "GH_TOKEN"},
		RequiredVars: []string{"GITHUB_TOKEN", "GH_TOKEN"},
	},
	"gitlab": {
		Type:         "gitlab",
		DetectVars:   []string{"GITLAB_TOKEN", "GLAB_TOKEN"},
		RequiredVars: []string{"GITLAB_TOKEN", "GLAB_TOKEN"},
	},
	"openai": {
		Type:         "openai",
		DetectVars:   []string{"OPENAI_API_KEY"},
		RequiredVars: []string{"OPENAI_API_KEY"},
	},
	"nvidia": {
		Type:         "nvidia",
		DetectVars:   []string{"NVIDIA_API_KEY"},
		RequiredVars: []string{"NVIDIA_API_KEY"},
	},
	"generic": {
		Type: "generic",
	},
}

// ResolveDefaultEnvVars returns the default environment variable names for a
// known credential type. Returns nil for unknown or generic types (which require
// explicit env_vars in the manifest).
func ResolveDefaultEnvVars(credType string) []string {
	profile, ok := KnownProviderProfiles[credType]
	if !ok || credType == "generic" {
		return nil
	}
	result := make([]string, len(profile.DetectVars))
	copy(result, profile.DetectVars)
	result = append(result, profile.ExtraEnvVars...)
	return result
}

// ResolveCredentials processes credential inputs from the manifest and returns
// provider configs ready for creation. Entries with missing required env vars
// are skipped with a warning.
func ResolveCredentials(entries []CredentialInput, wsName string) []ProviderConfig {
	var configs []ProviderConfig

	for _, entry := range entries {
		envVars := entry.EnvVars
		if len(envVars) == 0 {
			envVars = ResolveDefaultEnvVars(entry.Type)
		}

		profile, known := KnownProviderProfiles[entry.Type]
		providerName := fmt.Sprintf("cc-deck-%s-%s", wsName, entry.Type)

		// Check if at least one required var is set.
		hasRequired := false
		requiredVars := profile.RequiredVars
		if !known {
			requiredVars = envVars
		}

		for _, v := range requiredVars {
			if os.Getenv(v) != "" {
				hasRequired = true
				break
			}
		}

		if !hasRequired && entry.Type != "generic" {
			varList := ""
			if len(requiredVars) > 0 {
				varList = requiredVars[0]
				for i := 1; i < len(requiredVars); i++ {
					varList += ", " + requiredVars[i]
				}
			}
			log.Printf("WARNING: skipping credential %q: required env var(s) not set (%s)", entry.Type, varList)
			continue
		}

		// Determine the OpenShell provider type. Variant profiles (e.g.,
		// "claude-vertex") use the base type for provider creation.
		providerType := profile.Type
		if providerType == "" {
			providerType = entry.Type
		}

		cfg := ProviderConfig{
			Name:         providerName,
			Type:         providerType,
			FromExisting: true,
		}

		// claude-vertex uses OpenShell's native google-cloud provider.
		// Pass project_id and region as provider config; inject Claude Code
		// env vars (non-secret flags) into the sandbox shell rc.
		if entry.Type == "claude-vertex" {
			cfg.Credentials = make(map[string]string)
			if projectID := os.Getenv("ANTHROPIC_VERTEX_PROJECT_ID"); projectID != "" {
				cfg.Credentials["project_id"] = projectID
			}
			if region := os.Getenv("CLOUD_ML_REGION"); region != "" {
				cfg.Credentials["region"] = region
			} else {
				cfg.Credentials["region"] = "global"
			}

			cfg.EnvVarsToInject = map[string]string{
				"CLAUDE_CODE_USE_VERTEX": "1",
			}
			for _, v := range envVars {
				if val := os.Getenv(v); val != "" {
					cfg.EnvVarsToInject[v] = val
				}
			}
			for _, v := range profile.ExtraEnvVars {
				if val := os.Getenv(v); val != "" {
					cfg.EnvVarsToInject[v] = val
				}
			}

			configs = append(configs, cfg)
			continue
		}

		// For file-based credentials, record the file info.
		fileVar := entry.File
		if fileVar == "" && known {
			fileVar = profile.FileVar
		}
		if fileVar != "" {
			filePath := os.Getenv(fileVar)
			if filePath != "" {
				cfg.FileVar = fileVar
				cfg.FilePath = filePath
			}
		}

		// For generic type or unknown types, build explicit credentials.
		if entry.Type == "generic" || !known {
			cfg.FromExisting = false
			creds := make(map[string]string)
			for _, v := range envVars {
				if val := os.Getenv(v); val != "" {
					creds[v] = val
				}
			}
			cfg.Credentials = creds
		}

		configs = append(configs, cfg)
	}

	return configs
}

// DetectCredentials scans the host environment for known credential env vars
// and returns a list of detected credential entries for the capture wizard.
func DetectCredentials() []DetectedCredential {
	var detected []DetectedCredential
	seen := make(map[string]bool)

	// Check profiles in a deterministic order. More specific variants first
	// (claude-vertex before claude) so we detect the right auth mode.
	order := []string{"claude-vertex", "claude", "github", "gitlab", "openai", "nvidia"}

	for _, name := range order {
		providerType := KnownProviderProfiles[name].Type
		if seen[providerType] {
			continue
		}
		profile := KnownProviderProfiles[name]

		// For variant profiles (e.g., claude-vertex), ALL detect vars must be set.
		// For standard profiles, ANY detect var triggers detection.
		isVariant := name != providerType
		matched := false
		if isVariant {
			matched = true
			for _, v := range profile.DetectVars {
				if os.Getenv(v) == "" {
					matched = false
					break
				}
			}
		} else {
			for _, v := range profile.DetectVars {
				if os.Getenv(v) != "" {
					matched = true
					break
				}
			}
		}

		if matched {
			seen[providerType] = true
			if isVariant {
				// Variant supersedes its base profile. Mark the base profile's
				// type as seen to prevent duplicate provider creation.
				// E.g., claude-vertex (google-cloud) suppresses claude (claude).
				baseName := providerType // fallback
				for _, candidate := range order {
					cp := KnownProviderProfiles[candidate]
					if candidate != name && cp.Type == candidate {
						// candidate is a base profile; check if our name starts with it
						if len(name) > len(candidate)+1 && name[:len(candidate)+1] == candidate+"-" {
							baseName = cp.Type
							break
						}
					}
				}
				seen[baseName] = true
			}
			entry := DetectedCredential{
				Type:    name,
				EnvVars: ResolveDefaultEnvVars(name),
			}
			if profile.FileVar != "" {
				entry.File = profile.FileVar
			}
			detected = append(detected, entry)
		}
	}

	return detected
}

// InjectEnvVars writes environment variable exports into the sandbox's shell
// rc files. Used for configuration flags (like CLAUDE_CODE_USE_VERTEX) that
// are not secrets and don't go through the provider proxy.
func InjectEnvVars(ctx context.Context, client Client, sandboxID string, vars map[string]string) error {
	for k, v := range vars {
		exportLine := fmt.Sprintf("export %s=%q", k, v)
		for _, rcFile := range []string{".bashrc", ".zshrc"} {
			cmd := []string{"bash", "-c", fmt.Sprintf("echo %q >> /sandbox/%s", exportLine, rcFile)}
			if _, err := client.ExecSandbox(ctx, sandboxID, cmd); err != nil {
				log.Printf("WARNING: failed to set %s in %s: %v", k, rcFile, err)
			}
		}
	}
	return nil
}

// UploadFileCredential uploads a local file into the sandbox and sets the
// corresponding environment variable in the sandbox's shell rc files.
func UploadFileCredential(ctx context.Context, client Client, sandboxID, localPath, remotePath, envVarName string) error {
	if _, err := os.Stat(localPath); err != nil {
		return fmt.Errorf("credential file %q does not exist: %w", localPath, err)
	}

	if err := client.Upload(ctx, sandboxID, localPath, remotePath); err != nil {
		return fmt.Errorf("uploading credential file to sandbox: %w", err)
	}

	// Set the env var in shell rc files.
	exportLine := fmt.Sprintf("export %s=%q", envVarName, remotePath)
	for _, rcFile := range []string{".bashrc", ".zshrc"} {
		cmd := []string{"bash", "-c", fmt.Sprintf("echo %q >> /sandbox/%s", exportLine, rcFile)}
		if _, err := client.ExecSandbox(ctx, sandboxID, cmd); err != nil {
			log.Printf("WARNING: failed to set %s in %s: %v", envVarName, rcFile, err)
		}
	}

	return nil
}
