package compose

import (
	"fmt"
	"os/exec"
	"strings"
)

// Available detects the compose runtime in PATH.
// Detection order: podman-compose, docker compose (v2 plugin), docker-compose (legacy).
// Returns the binary path (or command) and nil on success, or an error if none found.
func Available() (string, error) {
	// 1. podman-compose (preferred)
	if path, err := exec.LookPath("podman-compose"); err == nil {
		return path, nil
	}

	// 2. docker compose (v2 plugin)
	if dockerPath, err := exec.LookPath("docker"); err == nil {
		cmd := exec.Command(dockerPath, "compose", "version")
		if out, runErr := cmd.Output(); runErr == nil && strings.Contains(string(out), "Docker Compose") {
			return dockerPath + " compose", nil
		}
	}

	// 3. docker-compose (legacy standalone)
	if path, err := exec.LookPath("docker-compose"); err == nil {
		return path, nil
	}

	return "", fmt.Errorf("compose runtime not found; install podman-compose or docker compose")
}

// RuntimeCmd splits the compose runtime path into command and args suitable for exec.
// For "podman-compose" returns ["podman-compose"].
// For "/usr/bin/docker compose" returns ["docker", "compose"].
func RuntimeCmd(runtime string) []string {
	return strings.Fields(runtime)
}
