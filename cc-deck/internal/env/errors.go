package env

import "errors"

var (
	// ErrNotSupported indicates that the requested operation is not
	// available for the given workspace type.
	ErrNotSupported = errors.New("operation not supported for this workspace type")

	// ErrNotImplemented indicates that the workspace type backend
	// has not been implemented yet.
	ErrNotImplemented = errors.New("workspace type not yet implemented")

	// ErrNameConflict indicates that a workspace with the same name
	// already exists in the state store.
	ErrNameConflict = errors.New("workspace with this name already exists")

	// ErrNotFound indicates that no workspace with the given name
	// exists in the state store.
	ErrNotFound = errors.New("workspace not found")

	// ErrInvalidName indicates that the provided workspace name does
	// not conform to the naming rules.
	ErrInvalidName = errors.New("invalid workspace name")

	// ErrZellijNotFound indicates that the zellij binary could not be
	// located in PATH.
	ErrZellijNotFound = errors.New("zellij binary not found in PATH")

	// ErrRunning indicates that the workspace is currently running and
	// cannot be deleted without the force flag.
	ErrRunning = errors.New("workspace is running; use --force to delete")

	// ErrPodmanNotFound indicates that the podman binary could not be
	// located in PATH.
	ErrPodmanNotFound = errors.New("podman binary not found in PATH")

	// ErrSSHNotFound indicates that the ssh binary could not be
	// located in PATH.
	ErrSSHNotFound = errors.New("ssh binary not found in PATH")

	// ErrKubectlNotFound indicates that the kubectl binary could not be
	// located in PATH.
	ErrKubectlNotFound = errors.New("kubectl binary not found in PATH")
)
