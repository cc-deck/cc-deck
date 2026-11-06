package ws

import (
	"context"
	"fmt"
)

// CheckShareSupported reports whether the workspace type supports session sharing.
func CheckShareSupported(workspace Workspace) error {
	if workspace.Type() != WorkspaceTypeLocal {
		return fmt.Errorf("sharing is currently supported for local workspaces only (workspace %q is %s)", workspace.Name(), workspace.Type())
	}
	return nil
}

// EnsureReady independently converges workspace infrastructure and its canonical session.
func EnsureReady(ctx context.Context, workspace Workspace, opts ReadyOptions) (ReadyResult, error) {
	if opts.Share {
		if err := CheckShareSupported(workspace); err != nil {
			return ReadyResult{}, err
		}
	}

	status, err := workspace.Status(ctx)
	if err != nil {
		return ReadyResult{}, err
	}

	var result ReadyResult
	if status.InfraState != nil && *status.InfraState != InfraStateRunning {
		infra, ok := workspace.(InfraManager)
		if !ok {
			return result, fmt.Errorf("workspace %q cannot start infrastructure", workspace.Name())
		}
		if err := infra.Start(ctx); err != nil {
			return result, err
		}
		result.InfrastructureStarted = true
	}

	if status.SessionState == SessionStateNone {
		sessions, ok := workspace.(SessionManager)
		if !ok {
			return result, fmt.Errorf("workspace %q cannot create its canonical session", workspace.Name())
		}
		created, err := sessions.EnsureSession(ctx, SessionStartOptions{WebSharing: opts.Share})
		if err != nil {
			return result, err
		}
		result.SessionCreated = created.Created
		result.SessionName = created.Name
	}

	return result, nil
}
