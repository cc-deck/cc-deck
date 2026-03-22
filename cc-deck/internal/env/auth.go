package env

import (
	"context"
	"os"
	"os/exec"
	"strings"
)

// DetectAuthMode determines which Claude Code authentication mode the host
// is using by checking environment variables.
func DetectAuthMode() AuthMode {
	if os.Getenv("CLAUDE_CODE_USE_VERTEX") == "1" {
		return AuthModeVertex
	}
	if os.Getenv("CLAUDE_CODE_USE_BEDROCK") == "1" {
		return AuthModeBedrock
	}
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		return AuthModeAPI
	}
	return AuthModeNone
}

// DetectAuthCredentials populates the credentials map with the environment
// variables required for the given auth mode. Existing entries (from
// explicit --credential flags) are not overwritten.
func DetectAuthCredentials(mode AuthMode, creds map[string]string) {
	inject := func(key string) {
		if _, exists := creds[key]; !exists {
			if val := os.Getenv(key); val != "" {
				creds[key] = val
			}
		}
	}

	switch mode {
	case AuthModeAPI:
		inject("ANTHROPIC_API_KEY")

	case AuthModeVertex:
		creds["CLAUDE_CODE_USE_VERTEX"] = "1"
		inject("ANTHROPIC_VERTEX_PROJECT_ID")
		inject("CLOUD_ML_REGION")
		inject("ANTHROPIC_MODEL")
		// For GOOGLE_APPLICATION_CREDENTIALS: inject if set, otherwise
		// check for the default ADC file from 'gcloud auth application-default login'.
		inject("GOOGLE_APPLICATION_CREDENTIALS")
		if _, exists := creds["GOOGLE_APPLICATION_CREDENTIALS"]; !exists {
			home, _ := os.UserHomeDir()
			defaultADC := home + "/.config/gcloud/application_default_credentials.json"
			if _, err := os.Stat(defaultADC); err == nil {
				creds["GOOGLE_APPLICATION_CREDENTIALS"] = defaultADC
			}
		}

	case AuthModeBedrock:
		creds["CLAUDE_CODE_USE_BEDROCK"] = "1"
		inject("AWS_REGION")
		inject("AWS_ACCESS_KEY_ID")
		inject("AWS_SECRET_ACCESS_KEY")
		inject("AWS_SESSION_TOKEN")
		inject("AWS_PROFILE")
		inject("ANTHROPIC_MODEL")
	}

	// Always include API key if present (useful as fallback alongside Vertex/Bedrock).
	inject("ANTHROPIC_API_KEY")

	// Common optional variables for all modes.
	inject("ANTHROPIC_DEFAULT_SONNET_MODEL")
	inject("ANTHROPIC_DEFAULT_OPUS_MODEL")
	inject("ANTHROPIC_DEFAULT_HAIKU_MODEL")
}

// ContainerHasZellijSession checks whether any active (non-exited) Zellij
// session is running inside the container.
func ContainerHasZellijSession(ctx context.Context, containerName string) bool {
	cmd := exec.CommandContext(ctx, "podman", "exec", containerName, "zellij", "list-sessions", "-n")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.Contains(line, "(EXITED") {
			return true
		}
	}
	return false
}
