package build

import (
	"fmt"
	"os/exec"
)

func DetectRuntime() (string, error) {
	if path, err := exec.LookPath("podman"); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("podman not found in PATH")
}
