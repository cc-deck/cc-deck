package share

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildInvitationEscapesURLAndStripsQuery(t *testing.T) {
	got, err := BuildInvitation("https://example.test/base?discard=yes", "demo space/#?%'", "brave-otter", "CONTROL_SECRET", RoleInteractive)
	require.NoError(t, err)
	require.Contains(t, got.URL, "https://example.test/base/demo%20space/%23%3F%25%27")
	require.Contains(t, got.Browser, "https://example.test/base/demo%20space/%23%3F%25%27")
	require.NotContains(t, got.Browser, "discard")
}

func TestBuildInvitationUsesRoleSpecificTokens(t *testing.T) {
	interactive, err := BuildInvitation("https://example.test", "selected", "brave-otter", "CONTROL_SECRET", RoleInteractive)
	require.NoError(t, err)
	require.Contains(t, interactive.Browser, "CONTROL_SECRET")
	require.NotContains(t, interactive.Browser, "WATCH_SECRET")

	observer, err := BuildInvitation("https://example.test", "selected", "calm-fox", "WATCH_SECRET", RoleObserver)
	require.NoError(t, err)
	require.Contains(t, observer.Browser, "WATCH_SECRET")
	require.NotContains(t, observer.Browser, "CONTROL_SECRET")
	require.Contains(t, observer.URL, "/selected")
}

func TestBuildInvitationRejectsNonAbsoluteEndpoint(t *testing.T) {
	_, err := BuildInvitation("localhost:8082", "session", "label", "token", RoleInteractive)
	require.ErrorContains(t, err, "invalid sharing endpoint")
}

func TestBuildInvitationUsesRoleSpecificWarnings(t *testing.T) {
	interactive, err := BuildInvitation("https://example.test", "selected", "brave-otter", "CONTROL_SECRET", RoleInteractive)
	require.NoError(t, err)
	require.Equal(t, "brave-otter", interactive.Label)
	require.Equal(t, RoleInteractive, interactive.Role)
	require.Equal(t, []string{TrustedControlWarning}, interactive.Warnings)

	observer, err := BuildInvitation("https://example.test", "selected", "calm-fox", "WATCH_SECRET", RoleObserver)
	require.NoError(t, err)
	// An observer link controls nothing, so it carries no warning at all.
	require.Empty(t, observer.Warnings)
	require.Contains(t, observer.Browser, "WATCH_SECRET")
}
