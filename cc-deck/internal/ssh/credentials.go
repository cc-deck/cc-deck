package ssh

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"sort"
	"strings"
)

// BuildCredentialSet resolves credentials based on auth mode and local environment.
// Returns a map of environment variable names to their values.
func BuildCredentialSet(authMode string, credentials []string, envVars map[string]string) (map[string]string, error) {
	creds := make(map[string]string)

	// Add explicit env vars from definition.
	for k, v := range envVars {
		creds[k] = v
	}

	// Add explicit credential keys from definition (resolve from local env).
	for _, key := range credentials {
		if val := os.Getenv(key); val != "" {
			creds[key] = val
		}
	}

	// Resolve auth mode.
	mode := authMode
	if mode == "" || mode == "auto" {
		mode = detectAuthMode()
	}

	if mode == "none" {
		return creds, nil
	}

	inject := func(key string) {
		if _, exists := creds[key]; !exists {
			if val := os.Getenv(key); val != "" {
				creds[key] = val
			}
		}
	}

	switch mode {
	case "api":
		inject("ANTHROPIC_API_KEY")

	case "vertex":
		creds["CLAUDE_CODE_USE_VERTEX"] = "1"
		inject("ANTHROPIC_VERTEX_PROJECT_ID")
		inject("CLOUD_ML_REGION")
		inject("ANTHROPIC_MODEL")
		inject("GOOGLE_APPLICATION_CREDENTIALS")
		if _, exists := creds["GOOGLE_APPLICATION_CREDENTIALS"]; !exists {
			home, _ := os.UserHomeDir()
			defaultADC := home + "/.config/gcloud/application_default_credentials.json"
			if _, err := os.Stat(defaultADC); err == nil {
				creds["GOOGLE_APPLICATION_CREDENTIALS"] = defaultADC
			}
		}

	case "bedrock":
		creds["CLAUDE_CODE_USE_BEDROCK"] = "1"
		inject("AWS_REGION")
		inject("AWS_ACCESS_KEY_ID")
		inject("AWS_SECRET_ACCESS_KEY")
		inject("AWS_SESSION_TOKEN")
		inject("AWS_PROFILE")
		inject("ANTHROPIC_MODEL")
	}

	// Always include API key if present.
	inject("ANTHROPIC_API_KEY")

	// Common optional model variables.
	inject("ANTHROPIC_DEFAULT_SONNET_MODEL")
	inject("ANTHROPIC_DEFAULT_OPUS_MODEL")
	inject("ANTHROPIC_DEFAULT_HAIKU_MODEL")

	return creds, nil
}

// detectAuthMode determines the auth mode from the local environment.
// Priority order per spec: ANTHROPIC_API_KEY first, then Vertex, then Bedrock.
func detectAuthMode() string {
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		return "api"
	}
	if os.Getenv("CLAUDE_CODE_USE_VERTEX") == "1" {
		return "vertex"
	}
	if os.Getenv("CLAUDE_CODE_USE_BEDROCK") == "1" {
		return "bedrock"
	}
	return "none"
}

// WriteCredentialFile writes a credential environment file on the remote host.
// The file is written to ~/.config/cc-deck/credentials.env with mode 600.
func WriteCredentialFile(ctx context.Context, client *Client, creds map[string]string) error {
	if len(creds) == 0 {
		return nil
	}

	// Sort keys for deterministic output.
	keys := make([]string, 0, len(creds))
	for key := range creds {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var lines []string
	for _, key := range keys {
		val := creds[key]
		// Check if value is a file path that should be copied instead.
		if info, err := os.Stat(val); err == nil && !info.IsDir() {
			// This is a file-based credential, handle via CopyCredentialFile.
			remoteName := key
			if copyErr := CopyCredentialFile(ctx, client, val, remoteName); copyErr != nil {
				return fmt.Errorf("copying credential file for %s: %w", key, copyErr)
			}
			// Set the env var to point to the remote file location.
			lines = append(lines, fmt.Sprintf("export %s=\"$HOME/.config/cc-deck/%s\"", key, remoteName))
			continue
		}
		lines = append(lines, fmt.Sprintf("export %s=%q", key, val))
	}

	content := strings.Join(lines, "\n") + "\n"

	// Use base64 encoding to safely transfer credential content,
	// avoiding shell injection via heredoc delimiter manipulation.
	encoded := base64.StdEncoding.EncodeToString([]byte(content))
	writeCmd := fmt.Sprintf(
		"mkdir -p ~/.config/cc-deck && echo %q | base64 -d > ~/.config/cc-deck/credentials.env && chmod 600 ~/.config/cc-deck/credentials.env",
		encoded)

	_, err := client.Run(ctx, writeCmd)
	if err != nil {
		return fmt.Errorf("writing credential file on remote: %w", err)
	}

	return nil
}

// CopyCredentialFile copies a local file to the remote host at
// ~/.config/cc-deck/<remoteName>.
func CopyCredentialFile(ctx context.Context, client *Client, localPath, remoteName string) error {
	// Ensure directory exists.
	if _, err := client.Run(ctx, "mkdir -p ~/.config/cc-deck"); err != nil {
		return fmt.Errorf("creating remote config directory: %w", err)
	}

	remotePath := fmt.Sprintf("~/.config/cc-deck/%s", remoteName)
	if err := client.Upload(ctx, localPath, remotePath); err != nil {
		return fmt.Errorf("uploading credential file: %w", err)
	}

	// Set permissions.
	if _, err := client.Run(ctx, fmt.Sprintf("chmod 600 %s", remotePath)); err != nil {
		return fmt.Errorf("setting credential file permissions: %w", err)
	}

	return nil
}
