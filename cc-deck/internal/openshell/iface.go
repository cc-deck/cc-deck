package openshell

import "context"

// Client defines the interface for communicating with the OpenShell gateway.
// The default implementation wraps the openshell CLI binary.
type Client interface {
	Address() string
	CreateSandbox(ctx context.Context, image, command, policy string, providers []string) (string, error)
	DeleteSandbox(ctx context.Context, sandboxName string) error
	GetSandbox(ctx context.Context, sandboxName string) (*SandboxInfo, error)
	ExecSandbox(ctx context.Context, sandboxName string, cmd []string) (*ExecResult, error)
	ExecSandboxStream(ctx context.Context, sandboxName string, cmd []string) error
	AttachExec(ctx context.Context, sandboxName string, cmd []string) error
	Upload(ctx context.Context, sandboxName, localPath, remotePath string) error
	Download(ctx context.Context, sandboxName, remotePath, localPath string) error

	// Provider management methods for credential injection.
	CreateProvider(ctx context.Context, name, providerType string, fromExisting bool, credentials map[string]string) error
	UpdateProvider(ctx context.Context, name, providerType string, fromExisting bool, credentials map[string]string) error
	DeleteProvider(ctx context.Context, name string) error
	EnsureProvider(ctx context.Context, name, providerType string, fromExisting bool, credentials map[string]string) error
}
