package share

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestSharingOperationTransitions(t *testing.T) {
	now := time.Now()
	op := &SharingOperation{State: StateStarting}
	require.True(t, op.Transition(StateActive, now))
	require.Equal(t, StateActive, op.State)
	require.False(t, op.Transition(StateStarting, now))
	require.True(t, op.Transition(StateStopping, now))
}

func TestSharingOperationPersistsInvitationMetadataWithoutSecrets(t *testing.T) {
	op := SharingOperation{
		Workspace: "demo",
		Invitations: []InvitationRecord{{
			Label: "brave-otter", Role: RoleInteractive, State: InvitationActive,
		}},
	}
	raw, err := yaml.Marshal(op)
	require.NoError(t, err)
	require.Contains(t, string(raw), "brave-otter")
	require.NotContains(t, string(raw), "token")
}
func TestSharingOperationRejectsTerminalShortcut(t *testing.T) {
	op := &SharingOperation{State: StateStarting}
	require.False(t, op.Transition(StateStopping, time.Now()))
}
