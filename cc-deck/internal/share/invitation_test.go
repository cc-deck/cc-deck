package share

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildInvitationsEscapesURLAndShellContexts(t *testing.T) {
	got, err := BuildInvitations("https://example.test/base?discard=yes", "demo space/#?%'", "interactive ' token", "observer token")
	require.NoError(t, err)
	require.Contains(t, got.InteractiveBrowser, "https://example.test/base/demo%20space/%23%3F%25%27")
	require.NotContains(t, got.InteractiveBrowser, "discard")
	require.Contains(t, got.InteractiveTerminal, `'interactive '"'"' token'`)
	require.Contains(t, got.InteractiveTerminal, " --insecure")
	require.NotEqual(t, got.InteractiveTerminal, got.ObserverTerminal)
}

func TestInvitationsUseCorrectRoleTokensAndWarnings(t *testing.T) {
	got, err := BuildInvitations("https://example.test", "selected", "CONTROL_SECRET", "WATCH_SECRET")
	require.NoError(t, err)
	for _, invitation := range []string{got.InteractiveBrowser, got.InteractiveTerminal} {
		require.Contains(t, invitation, "CONTROL_SECRET")
		require.NotContains(t, invitation, "WATCH_SECRET")
	}
	for _, invitation := range []string{got.ObserverBrowser, got.ObserverTerminal} {
		require.Contains(t, invitation, "WATCH_SECRET")
		require.NotContains(t, invitation, "CONTROL_SECRET")
	}
	require.Equal(t, []string{TrustedControlWarning, TerminalTLSWarning}, got.Warnings)
	require.Contains(t, got.ObserverTerminal, "/selected")
}

func TestBuildInvitationsRejectsNonAbsoluteEndpoint(t *testing.T) {
	_, err := BuildInvitations("localhost:8082", "session", "one", "two")
	require.ErrorContains(t, err, "invalid sharing endpoint")
}
