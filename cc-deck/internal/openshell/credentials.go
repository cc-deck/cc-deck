package openshell

import (
	"context"

	v1 "github.com/rhuss/openshell-sdk-go/openshell/v1"

	"github.com/cc-deck/cc-deck/internal/credential"
)

// OpenShellClientAdapter wraps v1.ClientInterface to satisfy
// credential.OpenShellClient for credential injection.
type OpenShellClientAdapter struct {
	Client v1.ClientInterface
}

var _ credential.OpenShellClient = (*OpenShellClientAdapter)(nil)

func (a *OpenShellClientAdapter) ExecRun(ctx context.Context, sandboxID string, cmd []string) error {
	_, err := a.Client.Exec().Run(ctx, sandboxID, cmd)
	return err
}

func (a *OpenShellClientAdapter) FileUpload(ctx context.Context, sandboxID, localPath, remotePath string) error {
	return a.Client.Files().Upload(ctx, sandboxID, localPath, remotePath)
}
