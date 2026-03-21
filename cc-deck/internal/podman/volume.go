package podman

import (
	"context"
	"strings"
)

// VolumeCreate creates a named volume. If the volume already exists, it is
// not an error (idempotent).
func VolumeCreate(ctx context.Context, name string) error {
	_, err := run(ctx, "volume", "create", name)
	if err != nil && strings.Contains(err.Error(), "already exists") {
		return nil
	}
	return err
}

// VolumeRemove removes a named volume. Ignores not-found errors.
func VolumeRemove(ctx context.Context, name string) error {
	_, err := run(ctx, "volume", "rm", name)
	if err != nil && (strings.Contains(err.Error(), "no such") || strings.Contains(err.Error(), "not found")) {
		return nil
	}
	return err
}

// VolumeExists returns true if the named volume exists.
func VolumeExists(ctx context.Context, name string) bool {
	_, err := run(ctx, "volume", "inspect", name)
	return err == nil
}
