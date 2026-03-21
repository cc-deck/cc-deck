package podman

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// SecretCreate creates or replaces a secret from the given data.
func SecretCreate(ctx context.Context, name string, data []byte) error {
	cmd := exec.CommandContext(ctx, "podman", "secret", "create", "--replace", name, "-")
	cmd.Stdin = bytes.NewReader(data)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("podman secret create: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// SecretRemove removes a secret. Ignores not-found errors.
func SecretRemove(ctx context.Context, name string) error {
	_, err := run(ctx, "secret", "rm", name)
	if err != nil && (strings.Contains(err.Error(), "no such") || strings.Contains(err.Error(), "not found")) {
		return nil
	}
	return err
}

// SecretExists returns true if the named secret exists.
func SecretExists(ctx context.Context, name string) bool {
	_, err := run(ctx, "secret", "inspect", name)
	return err == nil
}
