package ws

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type readinessFake struct {
	Workspace
	kind          WorkspaceType
	infra         *InfraStateValue
	session       SessionStateValue
	infraStarts   int
	sessionStarts int
}

func (f *readinessFake) Type() WorkspaceType { return f.kind }
func (f *readinessFake) Name() string        { return "demo" }
func (f *readinessFake) Status(context.Context) (*WorkspaceStatus, error) {
	return &WorkspaceStatus{InfraState: f.infra, SessionState: f.session}, nil
}
func (f *readinessFake) Start(context.Context) error { f.infraStarts++; return nil }
func (f *readinessFake) Stop(context.Context) error  { return nil }
func (f *readinessFake) EnsureSession(context.Context, SessionStartOptions) (SessionStartResult, error) {
	f.sessionStarts++
	return SessionStartResult{Created: true, Name: "cc-deck-demo"}, nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func readinessInfraPtr(value InfraStateValue) *InfraStateValue { return &value }

func TestEnsureReadyConvergesOnlyMissingDimensions(t *testing.T) {
	tests := []struct {
		name        string
		kind        WorkspaceType
		infra       *InfraStateValue
		session     SessionStateValue
		wantInfra   bool
		wantSession bool
	}{
		{"stopped and absent", WorkspaceTypeContainer, readinessInfraPtr(InfraStateStopped), SessionStateNone, true, true},
		{"running and absent", WorkspaceTypeContainer, readinessInfraPtr(InfraStateRunning), SessionStateNone, false, true},
		{"already ready", WorkspaceTypeContainer, readinessInfraPtr(InfraStateRunning), SessionStateExists, false, false},
		{"local absent", WorkspaceTypeLocal, nil, SessionStateNone, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workspace := &readinessFake{kind: tt.kind, infra: tt.infra, session: tt.session}
			_, err := EnsureReady(context.Background(), workspace, ReadyOptions{})
			require.NoError(t, err)
			require.Equal(t, boolToInt(tt.wantInfra), workspace.infraStarts)
			require.Equal(t, boolToInt(tt.wantSession), workspace.sessionStarts)
		})
	}
}

func TestEnsureReadyRejectsSharingForNonLocalWorkspace(t *testing.T) {
	_, err := EnsureReady(context.Background(), &readinessFake{kind: WorkspaceTypeContainer, session: SessionStateNone}, ReadyOptions{Share: true})
	require.ErrorContains(t, err, "sharing is currently supported for local workspaces only")
}
