package share

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type behavioralClient struct {
	credential string
	readOnly   bool
}

type behavioralTransport struct{ revision int }

func (t *behavioralTransport) attempt(client behavioralClient, _ string) bool {
	if client.readOnly {
		return false
	}
	t.revision++
	return true
}

func TestTwoObserversReuseReadOnlyCredentialAndRejectCompleteInputMatrix(t *testing.T) {
	runner := &fakeRunner{outputs: map[string][]byte{
		key("zellij", []string{"web", "--create-read-only-token"}): []byte("Created token successfully\n\nquiet-otter: OBSERVER_SECRET (read-only)"),
	}}
	credential, err := NewZellij(runner).CreateToken(context.Background(), "observer", true)
	require.NoError(t, err)
	require.Equal(t, "OBSERVER_SECRET", credential.Secret)
	require.Equal(t, []string{"web", "--create-read-only-token"}, runner.calls[0].args)

	transport := &behavioralTransport{}
	observers := []behavioralClient{
		{credential: credential.Secret, readOnly: true},
		{credential: credential.Secret, readOnly: true},
	}
	inputs := []string{"keyboard", "mouse", "paste", "resize", "tab-focus", "pane-focus", "terminal-control"}
	for observerNumber, observer := range observers {
		require.Equal(t, credential.Secret, observer.credential, "observer %d did not reuse the observer credential", observerNumber+1)
		for _, input := range inputs {
			before := transport.revision
			accepted := transport.attempt(observer, input)
			require.False(t, accepted, "observer %d input %s was accepted", observerNumber+1, input)
			require.Equal(t, before, transport.revision, "observer %d input %s changed shared state", observerNumber+1, input)
		}
	}

	interactive := behavioralClient{credential: "INTERACTIVE_SECRET", readOnly: false}
	require.True(t, transport.attempt(interactive, "keyboard"), "harness must detect an interactive state change")
	require.Equal(t, 1, transport.revision)
}
