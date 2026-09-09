package credential

import (
	"fmt"
	"os"

	"github.com/cc-deck/cc-deck/internal/config"
)

// ErrCredentialUnavailable is returned when a credential reference (env var or
// file path) cannot be resolved from the host environment.
type ErrCredentialUnavailable struct {
	Profile   string
	Reference string
}

func (e ErrCredentialUnavailable) Error() string {
	return fmt.Sprintf("profile %q: credential %s is not available", e.Profile, e.Reference)
}

// ErrSecretSourceUnsupported is returned when a profile uses a Kubernetes
// secret source but the resolver is running on the local host.
type ErrSecretSourceUnsupported struct {
	Profile string
}

func (e ErrSecretSourceUnsupported) Error() string {
	return fmt.Sprintf("profile %q: secret source is only available on Kubernetes", e.Profile)
}

// ResolveProfile resolves the credential sources declared in a profile into
// concrete values from the host environment. Secret sources are rejected
// because they require a Kubernetes runtime.
func ResolveProfile(name string, p config.Profile) (*ResolvedCredentials, error) {
	result := &ResolvedCredentials{
		EnvVars: map[string]string{},
	}

	auth := p.EffectiveAuth()

	// Resolve auth.api_key
	if auth.APIKey != nil {
		if err := resolveSource(name, "api_key", auth.APIKey, result); err != nil {
			return nil, err
		}
	}

	// Resolve auth.credentials
	if auth.Credentials != nil {
		if err := resolveSource(name, "credentials", auth.Credentials, result); err != nil {
			return nil, err
		}
	}

	// auth.Login requires no credentials to resolve.

	return result, nil
}

// resolveSource reads a single CredentialSource and populates the resolved
// result accordingly. envVar is the logical name used for file-based mounts.
func resolveSource(profile, envVar string, cs *config.CredentialSource, result *ResolvedCredentials) error {
	switch cs.Kind() {
	case config.SourceEnv:
		val := os.Getenv(cs.Env)
		if val == "" {
			return ErrCredentialUnavailable{Profile: profile, Reference: cs.Env}
		}
		result.EnvVars[cs.Env] = val

	case config.SourceFile:
		if _, err := os.ReadFile(cs.File); err != nil {
			return ErrCredentialUnavailable{Profile: profile, Reference: cs.File}
		}
		result.FileCredentials = append(result.FileCredentials, &ResolvedFile{
			EnvVar:    envVar,
			LocalPath: cs.File,
		})

	case config.SourceSecret:
		return ErrSecretSourceUnsupported{Profile: profile}

	case config.SourceNone:
		// Nothing to resolve.
	}

	return nil
}
