package share

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestZellijCapabilitiesAndSessionIsolation(t *testing.T) {
	r := &fakeRunner{outputs: map[string][]byte{
		key("zellij", []string{"--version"}):                        []byte("zellij 0.44.3"),
		key("zellij", []string{"web", "--help"}):                    []byte("--start --status --daemonize --create-token --token-name --create-read-only-token --revoke-token --stop"),
		key("zellij", []string{"attach", "--help"}):                 []byte("--token"),
		key("zellij", []string{"options", "--help"}):                []byte("--web-sharing"),
		key("zellij", []string{"list-sessions", "--no-formatting"}): []byte("alpha one [Created 1m ago] (EXITED - attach to resurrect)\nbeta & prod [Created now] (current)"),
	}, errors: map[string]error{}}
	z := NewZellij(r)
	require.NoError(t, z.ValidateCapabilities(context.Background()))
	got, e := z.ResolveSession(context.Background(), "beta & prod")
	require.NoError(t, e)
	require.Equal(t, "beta & prod", got)
	got, e = z.ResolveSession(context.Background(), "")
	require.NoError(t, e)
	require.Equal(t, "beta & prod", got)
	_, e = z.ResolveSession(context.Background(), "alpha one")
	require.Error(t, e)
}
func TestZellijRejectsOldVersion(t *testing.T) {
	r := &fakeRunner{outputs: map[string][]byte{key("zellij", []string{"--version"}): []byte("zellij 0.43.1")}, errors: map[string]error{}}
	require.ErrorContains(t, NewZellij(r).ValidateCapabilities(context.Background()), "0.44.3")
}
func TestZellijRejectsMissingRequiredCapability(t *testing.T) {
	r := &fakeRunner{outputs: map[string][]byte{key("zellij", []string{"--version"}): []byte("zellij 0.44.3"), key("zellij", []string{"web", "--help"}): []byte("--start --status --daemonize --create-token --token-name --revoke-token --stop")}, errors: map[string]error{}}
	require.ErrorContains(t, NewZellij(r).ValidateCapabilities(context.Background()), "read-only")
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

func TestZellijWebLifecycle(t *testing.T) {
	statusCalls := 0
	r := &fakeRunner{runFn: func(_ string, args []string) ([]byte, error) {
		if len(args) == 2 && args[0] == "web" && args[1] == "--status" {
			statusCalls++
			if statusCalls == 1 {
				return []byte("offline"), nil
			}
			return []byte("online at http://127.0.0.1:8082"), nil
		}
		return []byte("ok"), nil
	}}
	z := NewZellij(r)
	url, started, err := z.EnsureWebServer(context.Background())
	require.NoError(t, err)
	require.True(t, started)
	require.Equal(t, "http://127.0.0.1:8082", url)
	require.NoError(t, z.StopWebServer(context.Background()))
}
