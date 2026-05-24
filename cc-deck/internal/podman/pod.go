package podman

import (
	"context"
	"strings"
)

// PodCreate creates a new Podman pod with the given name and optional flags.
func PodCreate(ctx context.Context, name string, flags ...string) error {
	args := append([]string{"pod", "create", "--name", name}, flags...)
	_, err := run(ctx, args...)
	return err
}

// PodRemove removes a Podman pod and all its containers.
// Ignores not-found errors for idempotent cleanup.
func PodRemove(ctx context.Context, name string) error {
	_, err := run(ctx, "pod", "rm", "-f", name)
	if err != nil && (strings.Contains(err.Error(), "no such") || strings.Contains(err.Error(), "not found")) {
		return nil
	}
	return err
}

