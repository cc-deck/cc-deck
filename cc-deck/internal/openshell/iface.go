package openshell

import "context"

// Client defines the interface for communicating with the OpenShell gateway.
// The default implementation wraps the openshell CLI binary.
type Client interface {
	Address() string
	CreateSandbox(ctx context.Context, image, command, policy, provider string) (string, error)
	DeleteSandbox(ctx context.Context, sandboxName string) error
	GetSandbox(ctx context.Context, sandboxName string) (*SandboxInfo, error)
	ExecSandbox(ctx context.Context, sandboxName string, cmd []string) (*ExecResult, error)
	ExecSandboxStream(ctx context.Context, sandboxName string, cmd []string) error
	AttachExec(ctx context.Context, sandboxName string, cmd []string) error
	Upload(ctx context.Context, sandboxName, localPath, remotePath string) error
	Download(ctx context.Context, sandboxName, remotePath, localPath string) error
}
