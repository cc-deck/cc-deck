package setup

import (
	"fmt"
	"os/exec"
)

// DetectRuntime finds the available container runtime.
// Prefers podman, falls back to docker.
func DetectRuntime() (string, error) {
	if path, err := exec.LookPath("podman"); err == nil {
		return path, nil
	}
	if path, err := exec.LookPath("docker"); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("neither podman nor docker found in PATH")
}
