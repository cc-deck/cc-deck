package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	// probeCalls counts reads that verify the endpoint. A listing must never
	// make one, which is what keeps it as cheap as an unshared listing.
	probeCalls int
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
	s.probeCalls++
	return s.status, nil
}

// Snapshot is the non-probing read. A listing uses only this one.
func (s *recordingShareService) Snapshot(context.Context) (sharing.SharingStatus, error) {
	s.statusCalls++
	return s.status, nil
}

// installShareService swaps the sharing service factory for the test's duration.
func installShareService(t *testing.T, service sharing.Service) {
	t.Helper()
	originalFactory := makeWorkspaceShareService
	makeWorkspaceShareService = func(*GlobalFlags, shareOptions) (sharing.Service, error) { return service, nil }
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
	makeWorkspaceShareService = func(*GlobalFlags, shareOptions) (sharing.Service, error) { return recorder, nil }
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
	makeWorkspaceShareService = func(*GlobalFlags, shareOptions) (sharing.Service, error) { return recorder, nil }
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

// --- the verification gate at the command layer ------------------------------

// stubWorkspace implements only what the share path touches. Every other
// method panics, which is the assertion that the share path touches nothing
// else.
type stubWorkspace struct {
	ws.Workspace
	name         string
	sessionState ws.SessionStateValue
	killed       bool
}

func (w *stubWorkspace) Name() string           { return w.name }
func (w *stubWorkspace) Type() ws.WorkspaceType { return ws.WorkspaceTypeLocal }
func (w *stubWorkspace) KillSession(context.Context) error {
	w.killed = true
	return nil
}
func (w *stubWorkspace) Status(context.Context) (*ws.WorkspaceStatus, error) {
	return &ws.WorkspaceStatus{SessionState: w.sessionState}, nil
}

// failingShareService fails Start the way a failed verification does.
type failingShareService struct {
	recordingShareService
	startErr error
}

func (s *failingShareService) Start(context.Context, sharing.StartRequest) ([]sharing.Invitation, error) {
	return nil, s.startErr
}

// useEnabledZellijConfig points the web-sharing check at a temporary config so
// no test ever reads or rewrites the developer's own Zellij configuration.
func useEnabledZellijConfig(t *testing.T) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.kdl")
	require.NoError(t, os.WriteFile(path, []byte("web_sharing \"on\"\n"), 0o600))
	t.Setenv("ZELLIJ_CONFIG_FILE", path)
}

// installEnsureReady swaps the readiness step for the test's duration.
func installEnsureReady(t *testing.T, result ws.ReadyResult) {
	t.Helper()
	original := ensureReady
	ensureReady = func(context.Context, ws.Workspace, ws.ReadyOptions) (ws.ReadyResult, error) {
		return result, nil
	}
	t.Cleanup(func() { ensureReady = original })
}

// T029: a share that fails verification must leave the workspace running and
// usable locally. Killing the session here destroyed work the user asked for.
func TestFailedShareLeavesTheWorkspaceRunningAndUsable(t *testing.T) {
	useEnabledZellijConfig(t)
	installEnsureReady(t, ws.ReadyResult{SessionCreated: true, SessionName: "cc-deck-scratch"})

	probeResult := sharing.ProbeResult{OK: false, FailedAt: sharing.StageWebSocket,
		Diagnostic: "refused the WebSocket upgrade", CheckedAt: time.Now()}
	service := &failingShareService{startErr: &sharing.ProbeFailedError{Result: probeResult, Endpoint: "https://dev.example.com"}}
	installShareService(t, service)

	workspace := &stubWorkspace{name: "scratch", sessionState: ws.SessionStateNone}
	invitations, _, err := readyAndMaybeShare(context.Background(), &GlobalFlags{}, workspace, true, shareOptions{})

	require.Error(t, err, "a failed verification must fail the command")
	require.Empty(t, invitations, "no invitation is printed when verification fails")
	require.False(t, workspace.killed, "the workspace the user asked for must survive a sharing failure")
}

// T026: the failure message names the failing stage first, explains what that
// layer means, and points at the guide.
func TestVerificationFailureNamesTheStageFirst(t *testing.T) {
	probeResult := sharing.ProbeResult{OK: false, FailedAt: sharing.StageWebSocket,
		Diagnostic: "serves the web client but refused the WebSocket upgrade on /ws/control", CheckedAt: time.Now()}
	err := explainShareFailure(&sharing.ProbeFailedError{Result: probeResult, Endpoint: "https://dev.example.com"})

	require.Error(t, err)
	message := err.Error()
	require.True(t, strings.HasPrefix(message, "endpoint verification failed at the websocket stage"),
		"the stage is always named first, got %q", message)
	require.Contains(t, message, "https://dev.example.com")
	require.Contains(t, message, "Upgrade and Connection headers")
	require.Contains(t, message, "sharing guide")
}

func TestVerificationFailureExplainsEveryStage(t *testing.T) {
	for _, stage := range []sharing.ProbeStage{
		sharing.StageDNS, sharing.StageTLS, sharing.StageHTTP, sharing.StageAuth, sharing.StageWebSocket,
	} {
		t.Run(string(stage), func(t *testing.T) {
			err := explainShareFailure(&sharing.ProbeFailedError{
				Result:   sharing.ProbeResult{OK: false, FailedAt: stage, CheckedAt: time.Now()},
				Endpoint: "https://dev.example.com",
			})
			require.Contains(t, err.Error(), "failed at the "+string(stage)+" stage")
			require.NotContains(t, err.Error(), "The endpoint could not be verified.",
				"every declared stage needs its own explanation, not the fallback")
		})
	}
}

// T030: --endpoint applies to one command only and never edits configuration.
func TestEndpointFlagOverridesConfigurationForOneCommandOnly(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("sharing:\n  endpoint: https://configured.example\n"), 0o600))
	gf := &GlobalFlags{ConfigFile: configPath}

	cfg, err := loadSharingConfig(gf)
	require.NoError(t, err)
	overridden, err := sharing.NewStaticEndpoint(cfg.Sharing, "https://override.example", "", nil).Resolve(context.Background())
	require.NoError(t, err)
	require.Equal(t, "https://override.example", overridden.BaseURL)

	// The next command, without the flag, sees the configured value untouched.
	cfg, err = loadSharingConfig(gf)
	require.NoError(t, err)
	configured, err := sharing.NewStaticEndpoint(cfg.Sharing, "", "", nil).Resolve(context.Background())
	require.NoError(t, err)
	require.Equal(t, "https://configured.example", configured.BaseURL)

	raw, err := os.ReadFile(configPath)
	require.NoError(t, err)
	require.Contains(t, string(raw), "https://configured.example")
	require.NotContains(t, string(raw), "override.example", "a flag must never write itself into configuration")
}

func TestEndpointAndEndpointNameAreMutuallyExclusive(t *testing.T) {
	useEnabledZellijConfig(t)
	workspace := &stubWorkspace{name: "scratch"}
	_, _, err := readyAndMaybeShare(context.Background(), &GlobalFlags{}, workspace, true,
		shareOptions{endpoint: "https://a.example", endpointName: "work"})
	require.ErrorContains(t, err, "cannot be used together")
}

// T031: a --no-verify share reports no verification age, and introduces no
// third sharing state.
func TestNoVerifyShareReportsNoVerificationAge(t *testing.T) {
	require.Equal(t, "shared", sharingColumn(ws.SharingShared, nil),
		"a share created with --no-verify has no age to report")

	verified := &sharing.ProbeResult{OK: true, CheckedAt: time.Now().Add(-12 * time.Minute)}
	require.Equal(t, "shared (verified 12m ago)", sharingColumn(ws.SharingShared, verified))

	failed := &sharing.ProbeResult{OK: false, FailedAt: sharing.StageWebSocket, CheckedAt: time.Now().Add(-3 * time.Minute)}
	require.Equal(t, "degraded (websocket, 3m ago)", sharingColumn(ws.SharingDegraded, failed))
}

// T047: the three listing output shapes the CLI contract fixes.
func TestListingRendersTheThreeSharingShapes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status sharing.SharingStatus
		want   string
		absent string
	}{
		{
			name: "verified with age",
			status: sharing.SharingStatus{State: sharing.StateActive, Workspace: "alpha",
				LastProbe: &sharing.ProbeResult{OK: true, CheckedAt: time.Now().Add(-12 * time.Minute)}},
			want: "shared (verified 12m ago)",
		},
		{
			name: "failed names the stage and its age",
			status: sharing.SharingStatus{State: sharing.StateActive, Workspace: "alpha",
				LastProbe: &sharing.ProbeResult{OK: false, FailedAt: sharing.StageWebSocket, CheckedAt: time.Now().Add(-3 * time.Minute)}},
			want: "degraded (websocket, 3m ago)",
		},
		{
			name:   "no age at all",
			status: sharing.SharingStatus{State: sharing.StateActive, Workspace: "alpha"},
			want:   "shared",
			absent: "verified",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			installShareService(t, &recordingShareService{status: tc.status})
			instances := []*ws.WorkspaceInstance{{Name: "alpha", Type: ws.WorkspaceTypeLocal}}
			output := captureStdout(t, func() error {
				return writeWsTableWithProjects(&GlobalFlags{}, instances, nil, map[string]bool{"alpha": true}, "", map[string]string{}, false)
			})
			require.Contains(t, output, tc.want)
			if tc.absent != "" {
				require.NotContains(t, output, tc.absent)
			}
		})
	}
}

// T046a and T049: a listing performs zero probes, and resolves sharing state
// once for the whole listing rather than once per row. This is the measurement
// method for the listing cost criterion: a counting fake rather than a clock,
// so the assertion cannot go flaky under load.
func TestListingPerformsZeroProbesAndOneStatusLookup(t *testing.T) {
	const workspaces = 20
	recorder := &recordingShareService{status: sharing.SharingStatus{
		State: sharing.StateActive, Workspace: "ws-0",
		LastProbe: &sharing.ProbeResult{OK: true, CheckedAt: time.Now().Add(-time.Minute)},
	}}
	installShareService(t, recorder)

	var instances []*ws.WorkspaceInstance
	names := map[string]bool{}
	for i := 0; i < workspaces; i++ {
		name := fmt.Sprintf("ws-%d", i)
		instances = append(instances, &ws.WorkspaceInstance{Name: name, Type: ws.WorkspaceTypeLocal})
		names[name] = true
	}

	captureStdout(t, func() error {
		return writeWsTableWithProjects(&GlobalFlags{}, instances, nil, names, "", map[string]string{}, false)
	})

	require.Equal(t, 1, recorder.statusCalls,
		"a listing of %d workspaces resolves sharing state once, not once per row", workspaces)
	require.Zero(t, recorder.probeCalls, "a listing must never probe an endpoint")
}

// The counter above proves the listing chose the non-probing read. This proves
// the stronger claim: the verifying read is never reachable from a listing at
// all. The service fails the test the moment a listing calls Status, so a
// future change that quietly routes a listing back through verification is
// caught by the test rather than by a slow listing in the field.
type probeForbiddenShareService struct {
	recordingShareService
	t *testing.T
}

func (s *probeForbiddenShareService) Status(context.Context) (sharing.SharingStatus, error) {
	s.t.Helper()
	s.t.Fatal("a listing called the verifying read; listings must never probe an endpoint")
	return sharing.SharingStatus{}, nil
}

func TestListingNeverReachesTheVerifyingRead(t *testing.T) {
	service := &probeForbiddenShareService{t: t}
	service.status = sharing.SharingStatus{
		State: sharing.StateActive, Workspace: "alpha",
		LastProbe: &sharing.ProbeResult{OK: true, CheckedAt: time.Now().Add(-5 * time.Minute)},
	}
	installShareService(t, service)

	instances := []*ws.WorkspaceInstance{
		{Name: "alpha", Type: ws.WorkspaceTypeLocal},
		{Name: "beta", Type: ws.WorkspaceTypeLocal},
	}
	names := map[string]bool{"alpha": true, "beta": true}

	table := captureStdout(t, func() error {
		return writeWsTableWithProjects(&GlobalFlags{}, instances, nil, names, "", map[string]string{}, false)
	})
	require.Contains(t, table, "shared (verified 5m ago)")

	captureStdout(t, func() error {
		return writeWsStructured(&GlobalFlags{}, "json", instances, nil, names, "", map[string]string{})
	})
}

// A degraded share exits non-zero so scripts can detect it, while changing
// nothing. The message names the failing layer and says the share is intact.
func TestDegradedSharingExitsNonZeroAndSaysTheShareIsIntact(t *testing.T) {
	probe := &sharing.ProbeResult{OK: false, FailedAt: sharing.StageWebSocket, CheckedAt: time.Now()}
	err := degradedSharingError(probe, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed at the websocket stage")
	require.Contains(t, err.Error(), "The share is intact")
	require.Contains(t, err.Error(), "Upgrade and Connection headers")
}

func TestDegradedSharingWithoutAProbeReportsItsResiduals(t *testing.T) {
	err := degradedSharingError(nil, []string{"sharing state could not be verified: zellij is not responding"})
	require.ErrorContains(t, err, "sharing state could not be verified")

	require.ErrorContains(t, degradedSharingError(nil, nil), "sharing is degraded")
}

// A share whose last recorded check failed reads as degraded, and one that was
// never checked reads as shared. There is no third value.
func TestSharingStateHasExactlyTwoValuesForASharedWorkspace(t *testing.T) {
	for _, tc := range []struct {
		name  string
		probe *sharing.ProbeResult
		want  ws.WorkspaceSharingState
	}{
		{"never checked", nil, ws.SharingShared},
		{"last check passed", &sharing.ProbeResult{OK: true, CheckedAt: time.Now()}, ws.SharingShared},
		{"last check failed", &sharing.ProbeResult{OK: false, FailedAt: sharing.StageAuth, CheckedAt: time.Now()}, ws.SharingDegraded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			installShareService(t, &recordingShareService{status: sharing.SharingStatus{
				State: sharing.StateActive, Workspace: "alpha", LastProbe: tc.probe,
			}})
			state, _, _, _, probe := newSharingSnapshot(&GlobalFlags{}).detailsWithProbe("alpha", ws.WorkspaceTypeLocal)
			require.Equal(t, tc.want, state)
			require.Equal(t, tc.probe, probe)
		})
	}
}

// tempSharingConfig writes a minimal config so endpoint resolution in tests
// never reads the developer's real configuration.
func tempSharingConfig(t *testing.T, body string) *GlobalFlags {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return &GlobalFlags{ConfigFile: path}
}

func activeShare(workspace, endpoint string) sharing.SharingStatus {
	return sharing.SharingStatus{
		State:       sharing.StateActive,
		Workspace:   workspace,
		Session:     ws.ZellijSessionName(workspace),
		EndpointURL: endpoint,
		Invitations: []sharing.InvitationRecord{
			{Label: "proud-panda", Role: sharing.RoleInteractive, State: sharing.InvitationActive},
		},
		LastProbe: &sharing.ProbeResult{OK: true, CheckedAt: time.Now()},
	}
}

// An endpoint flag that cannot be applied must be refused rather than ignored.
// Reporting success while the share still runs through the previous endpoint is
// what made this look like it had worked.
func TestStartRefusesAnEndpointThatConflictsWithTheLiveShare(t *testing.T) {
	useEnabledZellijConfig(t)
	installEnsureReady(t, ws.ReadyResult{SessionName: "cc-deck-scratch"})
	installShareService(t, &recordingShareService{status: activeShare("scratch", "http://127.0.0.1:8082")})

	workspace := &stubWorkspace{name: "scratch", sessionState: ws.SessionStateExists}
	_, _, err := readyAndMaybeShare(context.Background(), tempSharingConfig(t, "{}\n"), workspace, true,
		shareOptions{endpoint: "https://other.example.com"})

	require.Error(t, err, "a conflicting endpoint must not be silently dropped")
	message := err.Error()
	require.Contains(t, message, "already shared through http://127.0.0.1:8082")
	require.Contains(t, message, "https://other.example.com was not applied")
	require.Contains(t, message, "cc-deck ws update scratch --endpoint https://other.example.com",
		"the error names the command that does apply it")
}

// Repeating the same request is not a conflict, so it stays a success.
func TestStartWithTheSameEndpointStaysIdempotent(t *testing.T) {
	useEnabledZellijConfig(t)
	installEnsureReady(t, ws.ReadyResult{SessionName: "cc-deck-scratch"})
	installShareService(t, &recordingShareService{status: activeShare("scratch", "http://127.0.0.1:8082")})

	workspace := &stubWorkspace{name: "scratch", sessionState: ws.SessionStateExists}
	output := captureStdout(t, func() error {
		_, _, err := readyAndMaybeShare(context.Background(), tempSharingConfig(t, "{}\n"), workspace, true,
			shareOptions{endpoint: "http://127.0.0.1:8082"})
		return err
	})

	require.Contains(t, output, `already shared through http://127.0.0.1:8082`)
	require.Contains(t, output, `invitation "proud-panda"`,
		"a repeat says what the live share is instead of nothing")
	require.Contains(t, output, "Login tokens are not stored",
		"tokens are never persisted, so the reason they cannot be reprinted is stated")
}

// Passing no endpoint at all must not be treated as a conflict, even when the
// configured default has drifted since the share started.
func TestStartWithoutAnEndpointFlagIsNeverAConflict(t *testing.T) {
	useEnabledZellijConfig(t)
	installEnsureReady(t, ws.ReadyResult{SessionName: "cc-deck-scratch"})
	installShareService(t, &recordingShareService{status: activeShare("scratch", "http://127.0.0.1:8082")})

	workspace := &stubWorkspace{name: "scratch", sessionState: ws.SessionStateExists}
	_, _, err := readyAndMaybeShare(context.Background(),
		tempSharingConfig(t, "sharing:\n  endpoint: https://drifted.example.com\n"),
		workspace, true, shareOptions{})

	require.NoError(t, err, "a drifted config default is not a conflict the user raised here")
}

func TestRetargetRefusesWhenTheWorkspaceIsNotShared(t *testing.T) {
	installShareService(t, &recordingShareService{status: sharing.SharingStatus{State: sharing.StateInactive}})

	err := runWsRetargetShare(context.Background(), tempSharingConfig(t, "{}\n"), "scratch",
		shareOptions{endpoint: "https://other.example.com"}, nil)

	require.Error(t, err)
	require.Contains(t, err.Error(), "is not shared, so there is no endpoint to move")
}

func TestRetargetIsANoOpWhenTheEndpointAlreadyMatches(t *testing.T) {
	recorder := &recordingShareService{status: activeShare("scratch", "http://127.0.0.1:8082")}
	installShareService(t, recorder)

	output := captureStdout(t, func() error {
		return runWsRetargetShare(context.Background(), tempSharingConfig(t, "{}\n"), "scratch",
			shareOptions{endpoint: "http://127.0.0.1:8082"}, nil)
	})

	require.Contains(t, output, "nothing to move")
	require.Empty(t, recorder.stopWorkspace, "an unchanged endpoint must not tear the share down")
}

func TestRetargetRejectsBothEndpointFlagsTogether(t *testing.T) {
	err := runWsRetargetShare(context.Background(), tempSharingConfig(t, "{}\n"), "scratch",
		shareOptions{endpoint: "https://a.example.com", endpointName: "home"}, nil)

	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot be used together")
}
