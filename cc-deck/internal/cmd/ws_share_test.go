package cmd

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sharing "github.com/cc-deck/cc-deck/internal/share"
	"github.com/cc-deck/cc-deck/internal/ws"
	"github.com/stretchr/testify/require"
)

type recordingShareService struct {
	inviteWorkspace string
	revokeWorkspace string
	stopWorkspace   string
	status          sharing.SharingStatus
}

func (*recordingShareService) Start(context.Context, sharing.StartRequest) ([]sharing.Invitation, error) {
	return nil, nil
}
func (s *recordingShareService) Invite(_ context.Context, req sharing.InviteRequest) (sharing.Invitation, error) {
	s.inviteWorkspace = req.Workspace
	return sharing.Invitation{}, nil
}
func (s *recordingShareService) Revoke(_ context.Context, workspace, _ string) (sharing.SharingStatus, error) {
	s.revokeWorkspace = workspace
	return sharing.SharingStatus{}, nil
}
func (s *recordingShareService) Status(context.Context) (sharing.SharingStatus, error) {
	return s.status, nil
}

func TestStructuredListIncludesSafeSharingMetadata(t *testing.T) {
	recorder := &recordingShareService{status: sharing.SharingStatus{
		State:       sharing.StateDegraded,
		Workspace:   "alpha",
		EndpointURL: "https://public.example",
		Invitations: []sharing.InvitationRecord{{Label: "amber-otter", Role: sharing.RoleObserver, State: sharing.InvitationActive}},
		Residuals:   []string{"endpoint cleanup pending"},
	}}
	originalFactory := makeWorkspaceShareService
	makeWorkspaceShareService = func(*GlobalFlags) (sharing.Service, error) { return recorder, nil }
	t.Cleanup(func() { makeWorkspaceShareService = originalFactory })

	originalStdout := os.Stdout
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = writer
	t.Cleanup(func() { os.Stdout = originalStdout })

	instance := &ws.WorkspaceInstance{Name: "alpha", Type: ws.WorkspaceTypeLocal}
	err = writeWsStructured(&GlobalFlags{}, "json", []*ws.WorkspaceInstance{instance}, nil, map[string]bool{"alpha": true}, "", map[string]string{})
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	os.Stdout = originalStdout
	output, err := io.ReadAll(reader)
	require.NoError(t, err)

	text := string(output)
	require.Contains(t, text, `"sharing_state": "degraded"`)
	require.Contains(t, text, `"sharing_endpoint": "https://public.example"`)
	require.Contains(t, text, `"label": "amber-otter"`)
	require.Contains(t, text, `"role": "observer"`)
	require.Contains(t, text, "endpoint cleanup pending")
	require.NotContains(t, text, "SECRET")
}
func (s *recordingShareService) Stop(_ context.Context, workspace string) (sharing.SharingStatus, error) {
	s.stopWorkspace = workspace
	return sharing.SharingStatus{}, nil
}

func TestSharingCommandsPassResolvedWorkspaceToService(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "state.yaml")
	t.Setenv("CC_DECK_STATE_FILE", stateFile)
	store := ws.NewStateStore(stateFile)
	require.NoError(t, store.AddInstance(&ws.WorkspaceInstance{Name: "beta", Type: ws.WorkspaceTypeLocal}))

	recorder := &recordingShareService{}
	originalFactory := makeWorkspaceShareService
	makeWorkspaceShareService = func(*GlobalFlags) (sharing.Service, error) { return recorder, nil }
	t.Cleanup(func() { makeWorkspaceShareService = originalFactory })

	for _, tc := range []struct {
		use  string
		args []string
	}{
		{"invite", []string{"beta", "--role", "observer"}},
		{"revoke", []string{"beta", "alice"}},
		{"unshare", []string{"beta"}},
	} {
		var commandFound bool
		for _, command := range newWsSharingCommands(&GlobalFlags{}) {
			if strings.HasPrefix(command.Use, tc.use) {
				commandFound = true
				command.SetArgs(tc.args)
				require.NoError(t, command.Execute())
				break
			}
		}
		require.True(t, commandFound, "command %s not found", tc.use)
	}

	require.Equal(t, "beta", recorder.inviteWorkspace)
	require.Equal(t, "beta", recorder.revokeWorkspace)
	require.Equal(t, "beta", recorder.stopWorkspace)
}
