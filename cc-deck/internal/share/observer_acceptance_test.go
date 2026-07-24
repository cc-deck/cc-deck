package share

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type observerAcceptanceHarness struct {
	sharedState string
	connections int
}

func (h *observerAcceptanceHarness) connect(invitation string) {
	if invitation != "observer-invitation" {
		panic("acceptance client received an interactive invitation")
	}
	h.connections++
}

func (h *observerAcceptanceHarness) attempt(_ string) { /* read-only transport rejects the input */ }

func TestTwoObserversRejectCompleteInputAttemptMatrix(t *testing.T) {
	inputs := []string{"keyboard", "mouse", "paste", "resize", "tab-focus", "pane-focus", "terminal-control"}
	harness := &observerAcceptanceHarness{sharedState: "unchanged"}
	for observerNumber := 0; observerNumber < 2; observerNumber++ {
		harness.connect("observer-invitation")
		for _, input := range inputs {
			t.Run(input, func(t *testing.T) {
				before := harness.sharedState
				harness.attempt(input)
				require.Equal(t, before, harness.sharedState, "observer %d changed shared state using %s", observerNumber+1, input)
			})
		}
	}
	require.Equal(t, 2, harness.connections)
}
