package session

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cc-deck/cc-deck/internal/xdg"
)

// isolateStateHome points xdg.StateHome at a temp dir for the duration of the
// test so CooldownElapsed() sees no previous auto snapshot and the real
// ~/.local/state/cc-deck is left untouched.
func isolateStateHome(t *testing.T) {
	t.Helper()

	orig := xdg.StateHome
	xdg.StateHome = filepath.Join(t.TempDir(), "state")
	t.Cleanup(func() { xdg.StateHome = orig })
}

// captureSpawns replaces the detached-spawn hook with a recorder and returns a
// pointer to the recorded argument vectors.
func captureSpawns(t *testing.T) *[][]string {
	t.Helper()

	var spawned [][]string
	orig := startDetached
	startDetached = func(binPath string, args ...string) {
		spawned = append(spawned, append([]string{binPath}, args...))
	}
	t.Cleanup(func() { startDetached = orig })

	return &spawned
}

// TestAutoSave_DoesNotReExecTestBinary is the regression guard for the fork
// bomb that exhausted host memory on 2026-09-07.
//
// AutoSave re-executes os.Executable() with "snapshot save --auto". Under
// "go test" that executable is the package test binary, which ignores those
// positional arguments and re-runs the entire suite instead. Every test that
// reached AutoSave therefore spawned another full suite run, and because the
// children are detached with Process.Release they were never reaped. One
// "go test ./internal/cmd" grew to 1333 live processes holding 18.6 GB.
func TestAutoSave_DoesNotReExecTestBinary(t *testing.T) {
	isolateStateHome(t)
	spawned := captureSpawns(t)

	require.True(t, CooldownElapsed(),
		"precondition: with an isolated state home there is no auto snapshot, so the cooldown must be elapsed")

	AutoSave()

	assert.Empty(t, *spawned,
		"AutoSave must not re-exec a test binary: each spawn re-runs the suite and forks another generation")
}

func TestIsTestBinaryPath(t *testing.T) {
	tests := []struct {
		name    string
		binPath string
		want    bool
	}{
		{"go test binary", "/tmp/go-build123/b001/cmd.test", true},
		{"go test binary on windows", `C:\tmp\b001\cmd.test.exe`, true},
		{"compiled test in cwd", "./session.test", true},
		{"real cc-deck binary", "/opt/homebrew/bin/cc-deck", false},
		{"real binary in build dir", "/Users/x/dev/cc-deck/cc-deck", false},
		{"binary whose name merely contains test", "/usr/local/bin/testrunner", false},
		{"empty path", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isTestBinaryPath(tt.binPath))
		})
	}
}

// TestIsTestBinary_DetectsTestRun documents that the runtime check alone is
// enough while the suite is executing, independent of the binary's name.
func TestIsTestBinary_DetectsTestRun(t *testing.T) {
	assert.True(t, isTestBinary("/opt/homebrew/bin/cc-deck"),
		"testing.Testing() must veto the re-exec even when the path looks like a real binary")
}
