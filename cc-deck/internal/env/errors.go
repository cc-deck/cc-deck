package env

import "errors"

var (
	// ErrNotSupported indicates that the requested operation is not
	// available for the given environment type.
	ErrNotSupported = errors.New("operation not supported for this environment type")

	// ErrNotImplemented indicates that the environment type backend
	// has not been implemented yet.
	ErrNotImplemented = errors.New("environment type not yet implemented")

	// ErrNameConflict indicates that an environment with the same name
	// already exists in the state store.
	ErrNameConflict = errors.New("environment with this name already exists")

	// ErrNotFound indicates that no environment with the given name
	// exists in the state store.
	ErrNotFound = errors.New("environment not found")

	// ErrInvalidName indicates that the provided environment name does
	// not conform to the naming rules.
	ErrInvalidName = errors.New("invalid environment name")

	// ErrZellijNotFound indicates that the zellij binary could not be
	// located in PATH.
	ErrZellijNotFound = errors.New("zellij binary not found in PATH")

	// ErrRunning indicates that the environment is currently running and
	// cannot be deleted without the force flag.
	ErrRunning = errors.New("environment is running; use --force to delete")

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
