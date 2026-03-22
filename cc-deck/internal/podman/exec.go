package podman

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// Exec runs a command inside a container. If interactive is true, it replaces
// the current process using syscall.Exec to connect stdin/stdout/stderr.
func Exec(ctx context.Context, nameOrID string, cmd []string, interactive bool) error {
	if interactive {
		binary, err := exec.LookPath("podman")
		if err != nil {
			return ErrPodmanNotFound
		}
		args := append([]string{"podman", "exec", "-it", nameOrID}, cmd...)
		return syscall.Exec(binary, args, os.Environ())
	}

	args := append([]string{"exec", nameOrID}, cmd...)
	c := exec.CommandContext(ctx, "podman", args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

// Cp copies files between a container and the local filesystem.
func Cp(ctx context.Context, src, dst string) error {
	// Resolve symlinks in local paths (e.g., /tmp -> /private/tmp on macOS)
	// to avoid "too many levels of symbolic links" errors from podman cp.
	src = resolveLocalPath(src)
	dst = resolveLocalPath(dst)

	out, err := exec.CommandContext(ctx, "podman", "cp", src, dst).CombinedOutput()
	if err != nil {
		return fmt.Errorf("podman cp: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// resolveLocalPath resolves symlinks in a local path. Container paths
// (containing ":") are returned unchanged.
func resolveLocalPath(path string) string {
	if strings.Contains(path, ":") {
		return path
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	// If the target doesn't exist yet, resolve the parent directory.
	dir := filepath.Dir(path)
	if resolvedDir, err := filepath.EvalSymlinks(dir); err == nil {
		return filepath.Join(resolvedDir, filepath.Base(path))
	}
	return path
}
