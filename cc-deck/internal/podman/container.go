package podman

import (
	"context"
	"fmt"
	"strings"
)

// Run creates and starts a new container in detached mode.
func Run(ctx context.Context, opts RunOpts) (string, error) {
	args := []string{"run", "-d"}

	if opts.Name != "" {
		args = append(args, "--name", opts.Name)
	}

	for _, v := range opts.Volumes {
		args = append(args, "-v", v)
	}

	for _, s := range opts.Secrets {
		if s.AsFile {
			// Mount as file at /run/secrets/<Name> (podman default behavior).
			args = append(args, "--secret", s.Name)
		} else {
			// Inject as environment variable.
			args = append(args, "--secret", fmt.Sprintf("%s,type=env,target=%s", s.Name, s.Target))
		}
	}

	for _, e := range opts.Envs {
		args = append(args, "-e", e)
	}

	if opts.AllPorts {
		args = append(args, "-P")
	} else {
		for _, p := range opts.Ports {
			args = append(args, "-p", p)
		}
	}

	args = append(args, opts.Image)

	cmd := opts.Cmd
	if len(cmd) == 0 {
		cmd = []string{"sleep", "infinity"}
	}
	args = append(args, cmd...)

	return run(ctx, args...)
}

// Start starts a stopped container.
func Start(ctx context.Context, nameOrID string) error {
	_, err := run(ctx, "start", nameOrID)
	return err
}

// Stop stops a running container.
func Stop(ctx context.Context, nameOrID string) error {
	_, err := run(ctx, "stop", nameOrID)
	return err
}

// Remove removes a container. If force is true, removes even if running.
func Remove(ctx context.Context, nameOrID string, force bool) error {
	args := []string{"rm"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, nameOrID)
	_, err := run(ctx, args...)
	return err
}

// Inspect returns container info, or nil, nil if the container is not found.
func Inspect(ctx context.Context, nameOrID string) (*ContainerInfo, error) {
	out, err := run(ctx, "inspect", "--format", "{{.State.Status}}\t{{.State.Running}}\t{{.Id}}\t{{.Name}}", nameOrID)
	if err != nil {
		if strings.Contains(err.Error(), "no such") || strings.Contains(err.Error(), "not found") {
			return nil, nil
		}
		return nil, err
	}

	parts := strings.SplitN(out, "\t", 4)
	if len(parts) < 4 {
		return nil, fmt.Errorf("unexpected inspect output: %s", out)
	}

	return &ContainerInfo{
		State:   parts[0],
		Running: parts[1] == "true",
		ID:      parts[2],
		Name:    parts[3],
	}, nil
}
