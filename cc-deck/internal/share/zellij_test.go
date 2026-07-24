package share

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestZellijCapabilitiesAndSessionIsolation(t *testing.T) {
	r := &fakeRunner{outputs: map[string][]byte{key("zellij", []string{"--version"}): []byte("zellij 0.44.3"), key("zellij", []string{"web", "--help"}): []byte("create-read-only-token"), key("zellij", []string{"list-sessions", "--no-formatting"}): []byte("alpha [Created]\nbeta")}, errors: map[string]error{}}
	z := NewZellij(r)
	require.NoError(t, z.ValidateCapabilities(context.Background()))
	got, e := z.ResolveSession(context.Background(), "beta")
	require.NoError(t, e)
	require.Equal(t, "beta", got)
	_, e = z.ResolveSession(context.Background(), "")
	require.Error(t, e)
}
func TestZellijRejectsOldVersion(t *testing.T) {
	r := &fakeRunner{outputs: map[string][]byte{key("zellij", []string{"--version"}): []byte("zellij 0.43.1")}, errors: map[string]error{}}
	require.ErrorContains(t, NewZellij(r).ValidateCapabilities(context.Background()), "0.44.3")
}
func TestZellijUsesSeparateTokenRolesAndSessionCommands(t *testing.T) {
	r := &fakeRunner{outputs: map[string][]byte{}, errors: map[string]error{}}
	z := NewZellij(r)
	ctx := context.Background()
	_, _ = z.CreateToken(ctx, "interactive", false)
	_, _ = z.CreateToken(ctx, "observer", true)
	require.NoError(t, z.ShareSession(ctx, "my session"))
	require.NoError(t, z.UnshareSession(ctx, "my session"))
	require.Equal(t, []string{"web", "--create-token", "--token-name", "interactive"}, r.calls[0].args)
	require.Equal(t, []string{"web", "--create-read-only-token", "--token-name", "observer"}, r.calls[1].args)
	require.Equal(t, []string{"--session", "my session", "options", "--web-sharing", "on"}, r.calls[2].args)
}
