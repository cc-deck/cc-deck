package credential

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/cc-deck/cc-deck/internal/agent"
)

// Detect checks the host environment against each credential spec and returns
// the specs whose required credentials are all present. Each AvailableMode
// includes the spec and a map of resolved values for non-fixed env vars.
func Detect(specs []agent.CredentialSpec) []AvailableMode {
	var available []AvailableMode
	for _, spec := range specs {
		if mode, ok := checkAvailability(spec); ok {
			available = append(available, mode)
		}
	}
	return available
}

func checkAvailability(spec agent.CredentialSpec) (AvailableMode, bool) {
	values := make(map[string]string)

	for _, ev := range spec.EnvVars {
		if ev.FixedValue != "" {
			values[ev.Name] = ev.FixedValue
			continue
		}
		val := os.Getenv(ev.Name)
		if val != "" {
			values[ev.Name] = val
		} else if ev.Required {
			return AvailableMode{}, false
		}
	}

	if spec.FileCredential != nil {
		path := resolveFilePath(spec.FileCredential)
		if path != "" {
			values[spec.FileCredential.EnvVar] = path
		} else if spec.FileCredential.Required {
			return AvailableMode{}, false
		}
	}

	return AvailableMode{Spec: spec, Values: values}, true
}

// resolveFilePath returns the resolved file path for a file credential, or
// empty string if the file does not exist.
func resolveFilePath(fc *agent.FileCredentialSpec) string {
	if envVal := os.Getenv(fc.EnvVar); envVal != "" {
		if _, err := os.Stat(envVal); err == nil {
			return envVal
		}
	}
	if fc.DefaultPath != "" {
		expanded := expandTilde(fc.DefaultPath)
		if _, err := os.Stat(expanded); err == nil {
			return expanded
		}
	}
	return ""
}

// expandTilde replaces a leading ~ with the user's home directory.
func expandTilde(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[1:])
}

// Resolve produces a complete ResolvedCredentials for a single spec by reading
// the host environment. All env vars (required and optional) are resolved,
// fixed values are injected, and file credentials are located.
func Resolve(spec agent.CredentialSpec) ResolvedCredentials {
	envVars := make(map[string]string)

	for _, ev := range spec.EnvVars {
		if ev.FixedValue != "" {
			envVars[ev.Name] = ev.FixedValue
			continue
		}
		if val := os.Getenv(ev.Name); val != "" {
			envVars[ev.Name] = val
		}
	}

	var fileCred *ResolvedFile
	if spec.FileCredential != nil {
		path := resolveFilePath(spec.FileCredential)
		if path != "" {
			fileCred = &ResolvedFile{
				EnvVar:    spec.FileCredential.EnvVar,
				LocalPath: path,
			}
		}
	}

	return ResolvedCredentials{
		EnvVars:        envVars,
		FileCredential: fileCred,
		UnsetVars:      spec.UnsetVars,
	}
}
