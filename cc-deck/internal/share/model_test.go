package share

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestSharingOperationTransitions(t *testing.T) {
	now := time.Now()
	op := &SharingOperation{State: StateStarting}
	require.True(t, op.Transition(StateActive, now))
	require.Equal(t, StateActive, op.State)
	require.False(t, op.Transition(StateStarting, now))
	require.True(t, op.Transition(StateStopping, now))
}
func TestSharingOperationRejectsTerminalShortcut(t *testing.T) {
	op := &SharingOperation{State: StateStarting}
	require.False(t, op.Transition(StateStopping, time.Now()))
}
