package share

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Zellij answers an empty session list with a message and a non-zero exit.
// That is a positive answer, and the only failure of list-sessions that is.
func TestSessionExistsTreatsAnEmptyListAsPositivelyAbsent(t *testing.T) {
	listSessions := key("zellij", []string{"list-sessions", "--no-formatting"})
	r := &fakeRunner{
		outputs: map[string][]byte{listSessions: []byte("No active zellij sessions found.\n")},
		errors:  map[string]error{listSessions: errors.New("exit status 1")},
	}

	got, err := NewZellij(r).SessionExists(context.Background(), "cc-deck-demo")

	require.NoError(t, err)
	require.False(t, got)
}

// Revocation is idempotent: a token Zellij no longer has is the outcome
// wanted, so its "does not exist" answer is success, and only that answer is.
func TestRevokeTokenTreatsAnUnknownTokenAsAlreadyRevoked(t *testing.T) {
	revoke := key("zellij", []string{"web", "--revoke-token", "gone"})
	r := &fakeRunner{
		outputs: map[string][]byte{revoke: []byte("Token by that name does not exist.\n")},
		errors:  map[string]error{revoke: errors.New("exit status 2")},
	}
	require.NoError(t, NewZellij(r).RevokeToken(context.Background(), "gone"))

	failing := &fakeRunner{
		outputs: map[string][]byte{revoke: []byte("Error occurred: could not open the token store\n")},
		errors:  map[string]error{revoke: errors.New("exit status 1")},
	}
	require.ErrorContains(t, NewZellij(failing).RevokeToken(context.Background(), "gone"), "revoke-token")
}

func TestSessionExistsPropagatesEveryOtherFailure(t *testing.T) {
	listSessions := key("zellij", []string{"list-sessions", "--no-formatting"})
	for name, output := range map[string]string{
		"error with output":    "Error occurred: failed to read the session directory\n",
		"error without output": "",
	} {
		t.Run(name, func(t *testing.T) {
			r := &fakeRunner{
				outputs: map[string][]byte{listSessions: []byte(output)},
				errors:  map[string]error{listSessions: errors.New("exit status 1")},
			}

			got, err := NewZellij(r).SessionExists(context.Background(), "cc-deck-demo")

			require.Error(t, err)
			require.ErrorContains(t, err, "list-sessions")
			require.False(t, got)
		})
	}
}

// deadlineRunner records the context deadline each call was given, and can
// simulate a command that outlives it.
type deadlineRunner struct {
	deadlines []time.Time
	hadNone   bool
	block     bool
}

func (r *deadlineRunner) Run(ctx context.Context, _ string, _ ...string) ([]byte, error) {
	if deadline, ok := ctx.Deadline(); ok {
		r.deadlines = append(r.deadlines, deadline)
	} else {
		r.hadNone = true
	}
	if r.block {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return []byte("ok"), nil
}

func TestZellijCallsAreBoundedByATimeout(t *testing.T) {
	runner := &deadlineRunner{}
	_, err := NewZellij(runner).run(context.Background(), "list-sessions")
	require.NoError(t, err)
	require.False(t, runner.hadNone, "an unbounded context must gain a deadline")
	require.Len(t, runner.deadlines, 1)
	require.WithinDuration(t, time.Now().Add(zellijCommandTimeout), runner.deadlines[0], 2*time.Second)
}

func TestZellijKeepsAnEarlierCallerDeadline(t *testing.T) {
	runner := &deadlineRunner{}
	callerDeadline := time.Now().Add(50 * time.Millisecond)
	ctx, cancel := context.WithDeadline(context.Background(), callerDeadline)
	defer cancel()

	_, err := NewZellij(runner).run(ctx, "list-sessions")
	require.NoError(t, err)
	require.Len(t, runner.deadlines, 1)
	require.WithinDuration(t, callerDeadline, runner.deadlines[0], time.Millisecond,
		"a caller's own deadline must not be replaced")
}

func TestZellijReportsAnUnresponsiveServerOnTimeout(t *testing.T) {
	original := zellijCommandTimeout
	zellijCommandTimeout = 20 * time.Millisecond
	t.Cleanup(func() { zellijCommandTimeout = original })

	_, err := NewZellij(&deadlineRunner{block: true}).run(context.Background(), "list-sessions")
	require.Error(t, err)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.ErrorContains(t, err, "not responding")
	require.ErrorContains(t, err, "list-sessions")
}

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
	got, e := z.SessionExists(context.Background(), "beta & prod")
	require.NoError(t, e)
	require.True(t, got)
	got, e = z.SessionExists(context.Background(), "alpha one")
	require.NoError(t, e)
	require.False(t, got)
}
func TestZellijRejectsOldVersion(t *testing.T) {
	r := &fakeRunner{outputs: map[string][]byte{key("zellij", []string{"--version"}): []byte("zellij 0.43.1")}, errors: map[string]error{}}
	require.ErrorContains(t, NewZellij(r).ValidateCapabilities(context.Background()), "0.44.3")
}
func TestZellijRejectsMissingRequiredCapability(t *testing.T) {
	r := &fakeRunner{outputs: map[string][]byte{key("zellij", []string{"--version"}): []byte("zellij 0.44.3"), key("zellij", []string{"web", "--help"}): []byte("--start --status --daemonize --create-token --token-name --revoke-token --stop")}, errors: map[string]error{}}
	require.ErrorContains(t, NewZellij(r).ValidateCapabilities(context.Background()), "read-only")
}
func TestZellijUsesSeparateTokenRoles(t *testing.T) {
	r := &fakeRunner{outputs: map[string][]byte{
		key("zellij", []string{"web", "--create-token"}):           []byte("Created token successfully\n\nswift-seal: INTERACTIVE_SECRET"),
		key("zellij", []string{"web", "--create-read-only-token"}): []byte("Created token successfully\n\nquiet-otter: OBSERVER_SECRET (read-only)"),
	}, errors: map[string]error{}}
	z := NewZellij(r)
	ctx := context.Background()
	interactive, err := z.CreateToken(ctx, "interactive", false)
	require.NoError(t, err)
	observer, err := z.CreateToken(ctx, "observer", true)
	require.NoError(t, err)
	require.Equal(t, TokenCredential{Name: "swift-seal", Secret: "INTERACTIVE_SECRET"}, interactive)
	require.Equal(t, TokenCredential{Name: "quiet-otter", Secret: "OBSERVER_SECRET"}, observer)
	require.Equal(t, []string{"web", "--create-token"}, r.calls[0].args)
	require.Equal(t, []string{"web", "--create-read-only-token"}, r.calls[1].args)
}

func TestZellijRejectsMalformedTokenOutput(t *testing.T) {
	r := &fakeRunner{outputs: map[string][]byte{
		key("zellij", []string{"web", "--create-token"}): []byte("Created token successfully"),
	}, errors: map[string]error{}}
	_, err := NewZellij(r).CreateToken(context.Background(), "interactive", false)
	require.ErrorContains(t, err, "parse Zellij token output")
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
