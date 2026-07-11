package openshell

import (
	"context"
	"fmt"
	"testing"

	v1 "github.com/rhuss/openshell-sdk-go/openshell/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockExecClient implements v1.ExecInterface for testing.
type mockExecClient struct {
	runCount int
	lastCmd  []string
}

func (m *mockExecClient) Run(_ context.Context, _ string, cmd []string, _ ...v1.ExecOptions) (*v1.ExecResult, error) {
	m.runCount++
	m.lastCmd = cmd
	return &v1.ExecResult{}, nil
}
func (m *mockExecClient) Stream(_ context.Context, _ string, _ []string, _ ...v1.ExecOptions) (v1.ExecStream, error) {
	return nil, nil
}
func (m *mockExecClient) Interactive(_ context.Context, _ string, _ []string, _, _ uint32, _ ...v1.ExecOptions) (v1.InteractiveSession, error) {
	return nil, nil
}

// mockFileClient implements v1.FileInterface for testing.
type mockFileClient struct {
	uploadErr      error
	uploadedLocal  string
	uploadedRemote string
}

func (m *mockFileClient) Upload(_ context.Context, _, localPath, remotePath string) error {
	if m.uploadErr != nil {
		return m.uploadErr
	}
	m.uploadedLocal = localPath
	m.uploadedRemote = remotePath
	return nil
}
func (m *mockFileClient) Download(_ context.Context, _, _, _ string) error { return nil }

// mockSDKClient implements v1.ClientInterface for testing.
type mockSDKClient struct {
	exec  *mockExecClient
	files *mockFileClient
}

func newMockSDKClient() *mockSDKClient {
	return &mockSDKClient{
		exec:  &mockExecClient{},
		files: &mockFileClient{},
	}
}

func (m *mockSDKClient) Sandboxes() v1.SandboxInterface { return nil }
func (m *mockSDKClient) Providers() v1.ProviderInterface { return nil }
func (m *mockSDKClient) Services() v1.ServiceInterface   { return nil }
func (m *mockSDKClient) Exec() v1.ExecInterface          { return m.exec }
func (m *mockSDKClient) Files() v1.FileInterface          { return m.files }
func (m *mockSDKClient) Health() v1.HealthInterface       { return nil }
func (m *mockSDKClient) SSH() v1.SSHInterface             { return nil }
func (m *mockSDKClient) TCP() v1.TCPInterface             { return nil }
func (m *mockSDKClient) Config() v1.ConfigInterface       { return nil }
func (m *mockSDKClient) Policy() v1.PolicyInterface       { return nil }
func (m *mockSDKClient) Close() error                     { return nil }

func TestOpenShellClientAdapter_ExecRun(t *testing.T) {
	sdk := newMockSDKClient()
	adapter := &OpenShellClientAdapter{Client: sdk}

	err := adapter.ExecRun(context.Background(), "sb-123", []string{"echo", "hello"})
	require.NoError(t, err)
	assert.Equal(t, 1, sdk.exec.runCount)
	assert.Equal(t, []string{"echo", "hello"}, sdk.exec.lastCmd)
}

func TestOpenShellClientAdapter_FileUpload(t *testing.T) {
	sdk := newMockSDKClient()
	adapter := &OpenShellClientAdapter{Client: sdk}

	err := adapter.FileUpload(context.Background(), "sb-123", "/local/path", "/remote/path")
	require.NoError(t, err)
	assert.Equal(t, "/local/path", sdk.files.uploadedLocal)
	assert.Equal(t, "/remote/path", sdk.files.uploadedRemote)
}

func TestOpenShellClientAdapter_FileUploadError(t *testing.T) {
	sdk := &mockSDKClient{
		exec:  &mockExecClient{},
		files: &mockFileClient{uploadErr: fmt.Errorf("gateway unreachable")},
	}
	adapter := &OpenShellClientAdapter{Client: sdk}

	err := adapter.FileUpload(context.Background(), "sb-123", "/local/path", "/remote/path")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gateway unreachable")
}
