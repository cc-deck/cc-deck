package credential

import (
	"fmt"
	"os"
	"strings"

	"github.com/cc-deck/cc-deck/internal/agent"
)

// Validate checks that all required credentials for the given spec are present
// on the host. Returns nil if all credentials are available. Returns a
// descriptive error listing each missing credential by name.
//
// If externalCredentials is true, validation is skipped entirely (the
// credentials are expected to be provided by the runtime environment,
// e.g., K8s Secrets or OpenShell providers).
func Validate(spec agent.CredentialSpec, externalCredentials bool) error {
	if externalCredentials {
		return nil
	}

	var missing []string

	for _, ev := range spec.EnvVars {
		if ev.FixedValue != "" {
			continue
		}
		if ev.Required && os.Getenv(ev.Name) == "" {
			missing = append(missing, ev.Name)
		}
	}

	if spec.FileCredential != nil && spec.FileCredential.Required {
		path := resolveFilePath(spec.FileCredential)
		if path == "" {
			desc := spec.FileCredential.EnvVar
			if spec.FileCredential.DefaultPath != "" {
				desc += " (or file at " + spec.FileCredential.DefaultPath + ")"
			}
			missing = append(missing, desc)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing credentials for auth mode %q: %s", spec.Name, strings.Join(missing, ", "))
	}

	return nil
}
