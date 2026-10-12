package share

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/cc-deck/cc-deck/internal/config"
	"github.com/stretchr/testify/require"
)

// The unit suite that preceded this feature passed in full while sharing had
// never once worked for a human. Every stage failure below is reproduced
// offline by probe_test.go, but only this test proves the probe agrees with
// the real Zellij web server about what a working endpoint looks like.
//
// It points a static endpoint at the local web server rather than at a tunnel,
// so it needs no external service and runs wherever zellij is installed.

// zellijEnv strips the variables that make a session-creating command attach to
// the caller's own session instead of creating a new one. Inside an existing
// Zellij session, "zellij --layout NAME attach -b SESSION" adds a tab, exits
// zero, and creates nothing, which would corrupt the developer's live session
// and make this test lie about what it verified.
func zellijEnv() []string {
	var stripped []string
	for _, entry := range os.Environ() {
		switch {
		case strings.HasPrefix(entry, "ZELLIJ="),
			strings.HasPrefix(entry, "ZELLIJ_SESSION_NAME="),
			strings.HasPrefix(entry, "ZELLIJ_PANE_ID="):
			continue
		}
		stripped = append(stripped, entry)
	}
	return stripped
}

// detachedRunner runs zellij with the session-creating environment stripped.
type detachedRunner struct{}

func (detachedRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, name, args...)
	command.Env = zellijEnv()
	return command.CombinedOutput()
}

func requireZellij(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("zellij"); err != nil {
		t.Skip("zellij is not installed; the end to end acceptance test needs it")
	}
	if os.Getenv("CC_DECK_SKIP_E2E") != "" {
		t.Skip("CC_DECK_SKIP_E2E is set")
	}
}

// startBackgroundSession creates a real detached session and removes it after
// the test, whatever the outcome.
func startBackgroundSession(t *testing.T, name string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	command := exec.CommandContext(ctx, "zellij", "attach", "-b", "-c", name)
	command.Env = zellijEnv()
	if out, err := command.CombinedOutput(); err != nil {
		t.Skipf("could not create a background Zellij session: %v: %s", err, out)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		kill := exec.CommandContext(cleanupCtx, "zellij", "delete-session", "--force", name)
		kill.Env = zellijEnv()
		_ = kill.Run()
	})
}

// TestEndToEndProbeAgreesWithARealZellijWebServer is the acceptance test for
// the whole feature: a real web server, a real session, a static endpoint
// pointed at the real local address, and the real five stage probe.
func TestEndToEndProbeAgreesWithARealZellijWebServer(t *testing.T) {
	requireZellij(t)

	zellij := NewZellij(detachedRunner{})
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := zellij.ValidateCapabilities(ctx); err != nil {
		t.Skipf("this Zellij does not support web sharing: %v", err)
	}

	sessionName := "cc-deck-e2e-probe"
	startBackgroundSession(t, sessionName)

	localURL, started, err := zellij.EnsureWebServer(ctx)
	if err != nil {
		t.Skipf("could not start the Zellij web server: %v", err)
	}
	if started {
		// Only stop what this test started. A web server the developer was
		// already running is not ours to close, which is the same rule the
		// feature itself now follows.
		t.Cleanup(func() {
			stopCtx, stopCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer stopCancel()
			_ = zellij.StopWebServer(stopCtx)
		})
	}
	require.NotEmpty(t, localURL)

	endpoint := NewStaticEndpoint(config.SharingConfig{Endpoint: localURL}, "", "", zellij)

	probeCtx, probeCancel := context.WithTimeout(ctx, 30*time.Second)
	defer probeCancel()
	result, err := endpoint.Probe(probeCtx, EndpointRef{BaseURL: localURL}, sessionName)

	require.NoError(t, err, "the real web server must pass every stage; diagnostic was %q at %s", result.Diagnostic, result.FailedAt)
	require.True(t, result.OK)
	require.Empty(t, result.FailedAt)
	require.False(t, result.CheckedAt.IsZero())
}

// The complement: a real endpoint address that no longer serves must fail, and
// must fail at a layer that names the problem rather than timing out.
func TestEndToEndProbeFailsAgainstAnAddressThatServesNothing(t *testing.T) {
	requireZellij(t)

	zellij := NewZellij(detachedRunner{})
	// Port zero is never listening, so the HTTP stage cannot connect. The
	// address resolves, so this is not a DNS failure, which is the attribution
	// being checked.
	endpoint := NewStaticEndpoint(config.SharingConfig{Endpoint: "http://127.0.0.1:1"}, "", "", zellij)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	result, err := endpoint.Probe(ctx, EndpointRef{BaseURL: "http://127.0.0.1:1"}, "cc-deck-e2e-probe")

	require.Error(t, err)
	require.False(t, result.OK)
	require.Equal(t, StageHTTP, result.FailedAt, "a closed port is an HTTP-layer failure, not a name that does not resolve")
}

// The probe must never leave a credential behind on a real Zellij, which is the
// invariant that unit fakes cannot prove.
func TestEndToEndProbeLeavesNoTokenBehind(t *testing.T) {
	requireZellij(t)

	zellij := NewZellij(detachedRunner{})
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := zellij.ValidateCapabilities(ctx); err != nil {
		t.Skipf("this Zellij does not support web sharing: %v", err)
	}

	sessionName := "cc-deck-e2e-tokens"
	startBackgroundSession(t, sessionName)

	localURL, started, err := zellij.EnsureWebServer(ctx)
	if err != nil {
		t.Skipf("could not start the Zellij web server: %v", err)
	}
	if started {
		t.Cleanup(func() {
			stopCtx, stopCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer stopCancel()
			_ = zellij.StopWebServer(stopCtx)
		})
	}

	counted := func() int {
		listCtx, listCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer listCancel()
		command := exec.CommandContext(listCtx, "zellij", "web", "--list-tokens")
		command.Env = zellijEnv()
		out, err := command.CombinedOutput()
		if err != nil {
			t.Skipf("this Zellij cannot list tokens: %v: %s", err, out)
		}
		return len(strings.Split(strings.TrimSpace(string(out)), "\n"))
	}

	before := counted()
	endpoint := NewStaticEndpoint(config.SharingConfig{Endpoint: localURL}, "", "", zellij)
	_, _ = endpoint.Probe(ctx, EndpointRef{BaseURL: localURL}, sessionName)
	require.Equal(t, before, counted(), "the probe must revoke the credential it minted")
}
