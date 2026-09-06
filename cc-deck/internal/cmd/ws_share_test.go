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
	statusCalls     int
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
	s.statusCalls++
	return s.status, nil
}

// installShareService swaps the sharing service factory for the test's duration.
func installShareService(t *testing.T, service sharing.Service) {
	t.Helper()
	originalFactory := makeWorkspaceShareService
	makeWorkspaceShareService = func(*GlobalFlags) (sharing.Service, error) { return service, nil }
	t.Cleanup(func() { makeWorkspaceShareService = originalFactory })
}

// captureStdout runs fn and returns everything it wrote to os.Stdout.
func captureStdout(t *testing.T, fn func() error) string {
	t.Helper()
	originalStdout := os.Stdout
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = writer
	defer func() { os.Stdout = originalStdout }()
	runErr := fn()
	require.NoError(t, writer.Close())
	os.Stdout = originalStdout
	require.NoError(t, runErr)
	output, err := io.ReadAll(reader)
	require.NoError(t, err)
	return string(output)
}

func TestListQueriesSharingStatusOncePerInvocation(t *testing.T) {
	recorder := &recordingShareService{status: sharing.SharingStatus{State: sharing.StateInactive}}
	installShareService(t, recorder)

	instances := []*ws.WorkspaceInstance{
		{Name: "alpha", Type: ws.WorkspaceTypeLocal},
		{Name: "beta", Type: ws.WorkspaceTypeLocal},
		{Name: "gamma", Type: ws.WorkspaceTypeLocal},
	}
	defs := []*ws.WorkspaceDefinition{{Name: "delta", Type: ws.WorkspaceTypeContainer}}
	names := map[string]bool{"alpha": true, "beta": true, "gamma": true}

	captureStdout(t, func() error {
		return writeWsTableWithProjects(&GlobalFlags{}, instances, defs, names, "", map[string]string{}, false)
	})
	require.Equal(t, 1, recorder.statusCalls, "table listing must not query sharing status per row")

	recorder.statusCalls = 0
	captureStdout(t, func() error {
		return writeWsStructured(&GlobalFlags{}, "json", instances, defs, names, "", map[string]string{})
	})
	require.Equal(t, 1, recorder.statusCalls, "structured listing must not query sharing status per row")
}

func TestListSkipsSharingStatusWithoutLocalWorkspaces(t *testing.T) {
	recorder := &recordingShareService{}
	installShareService(t, recorder)

	instances := []*ws.WorkspaceInstance{{Name: "box", Type: ws.WorkspaceTypeContainer}}
	captureStdout(t, func() error {
		return writeWsTableWithProjects(&GlobalFlags{}, instances, nil, map[string]bool{"box": true}, "", map[string]string{}, false)
	})
	require.Equal(t, 0, recorder.statusCalls, "no local workspace means no sharing lookup")
}

func TestVerboseListShowsSharingEndpoint(t *testing.T) {
	recorder := &recordingShareService{status: sharing.SharingStatus{
		State: sharing.StateActive, Workspace: "alpha", EndpointURL: "https://public.example",
	}}
	installShareService(t, recorder)
	instances := []*ws.WorkspaceInstance{
		{Name: "alpha", Type: ws.WorkspaceTypeLocal},
		{Name: "beta", Type: ws.WorkspaceTypeLocal},
	}
	names := map[string]bool{"alpha": true, "beta": true}

	compact := captureStdout(t, func() error {
		return writeWsTableWithProjects(&GlobalFlags{}, instances, nil, names, "", map[string]string{}, false)
	})
	require.NotContains(t, compact, "ENDPOINT", "compact table must stay narrow")
	require.NotContains(t, compact, "https://public.example")

	verbose := captureStdout(t, func() error {
		return writeWsTableWithProjects(&GlobalFlags{}, instances, nil, names, "", map[string]string{}, true)
	})
	require.Contains(t, verbose, "ENDPOINT")
	lines := strings.Split(strings.TrimSpace(verbose), "\n")
	require.Len(t, lines, 3, "header plus two rows")
	require.Contains(t, lines[1], "https://public.example", "shared workspace shows its endpoint")
	require.Regexp(t, `^beta\s+local\s+\S+\s+\S+\s+private\s+-\s`, lines[2], "private workspace shows a dash")
}

func TestWsWithoutSubcommandListsWorkspaces(t *testing.T) {
	wsCmd := NewWsCmd(&GlobalFlags{})
	require.NotNil(t, wsCmd.RunE, "bare 'cc-deck ws' must list instead of printing help")
	require.Error(t, wsCmd.Args(wsCmd, []string{"strat"}), "unknown subcommand must still be rejected")
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
