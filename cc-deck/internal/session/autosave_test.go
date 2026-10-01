package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"

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

func TestCooldownElapsed_TrueWhenNoAutoSnapshotExists(t *testing.T) {
	withTempStateHome(t)

	if !CooldownElapsed() {
		t.Error("CooldownElapsed() = false, want true when no previous auto-save exists")
	}
}

func TestCooldownElapsed_FalseWhenRecentAutoSnapshotExists(t *testing.T) {
	withTempStateHome(t)

	if err := SaveSnapshot(&Snapshot{Version: 1, Name: autoSaveName, SavedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("seeding auto snapshot: %v", err)
	}

	if CooldownElapsed() {
		t.Error("CooldownElapsed() = true, want false right after an auto-save")
	}
}

func TestCooldownElapsed_TrueWhenAutoSnapshotIsOld(t *testing.T) {
	withTempStateHome(t)

	if err := SaveSnapshot(&Snapshot{Version: 1, Name: autoSaveName, SavedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("seeding auto snapshot: %v", err)
	}
	old := time.Now().Add(-autoSaveCooldown - time.Minute)
	if err := os.Chtimes(snapshotPath(autoSaveName), old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	if !CooldownElapsed() {
		t.Error("CooldownElapsed() = false, want true once the cooldown window has passed")
	}
}

func TestRotateAutoSnapshots_CreatesAllTiersWhenMissing(t *testing.T) {
	withTempStateHome(t)

	snap := &Snapshot{Version: 1, Name: autoSaveName, SavedAt: time.Now().UTC()}
	rotateAutoSnapshots(snap)

	for _, tier := range autoSaveTiers {
		if _, err := os.Stat(snapshotPath(tier.name)); err != nil {
			t.Errorf("expected tier snapshot %q to be created, stat error: %v", tier.name, err)
		}
	}
}

func TestRotateAutoSnapshots_SkipsTierNotYetExpired(t *testing.T) {
	withTempStateHome(t)

	tier := autoSaveTiers[0] // "auto-hourly", 1h threshold
	path := snapshotPath(tier.name)

	// Seed the tier snapshot as freshly saved, well within its threshold.
	if err := SaveSnapshot(&Snapshot{Version: 1, Name: tier.name, SavedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("seeding tier snapshot: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	originalModTime := info.ModTime()

	snap := &Snapshot{Version: 1, Name: autoSaveName, SavedAt: time.Now().UTC(),
		Sessions: []SessionEntry{{DisplayName: "should-not-be-written"}}}
	rotateAutoSnapshots(snap)

	info, err = os.Stat(path)
	if err != nil {
		t.Fatalf("stat after rotate: %v", err)
	}
	if !info.ModTime().Equal(originalModTime) {
		t.Error("expected tier snapshot to be left untouched before its threshold elapses")
	}
}

func TestRotateAutoSnapshots_OverwritesTierAfterThresholdElapsed(t *testing.T) {
	withTempStateHome(t)

	tier := autoSaveTiers[0] // "auto-hourly", 1h threshold
	path := snapshotPath(tier.name)

	if err := SaveSnapshot(&Snapshot{Version: 1, Name: tier.name, SavedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("seeding tier snapshot: %v", err)
	}
	old := time.Now().Add(-tier.threshold - time.Minute)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	snap := &Snapshot{Version: 1, Name: autoSaveName, SavedAt: time.Now().UTC(),
		Sessions: []SessionEntry{{DisplayName: "promoted"}}}
	rotateAutoSnapshots(snap)

	loaded, err := LoadSnapshot(tier.name)
	if err != nil {
		t.Fatalf("loading tier snapshot: %v", err)
	}
	if loaded.Name != tier.name {
		t.Errorf("Name = %q, want %q (tier name preserved on the promoted copy)", loaded.Name, tier.name)
	}
	if len(loaded.Sessions) != 1 || loaded.Sessions[0].DisplayName != "promoted" {
		t.Errorf("expected promoted sessions data in tier snapshot, got %+v", loaded.Sessions)
	}
}

func TestSnapshotPath_JoinsSessionsDirAndName(t *testing.T) {
	dir := withTempStateHome(t)

	got := snapshotPath("foo")
	want := filepath.Join(dir, "cc-deck", "sessions", "foo.json")
	if got != want {
		t.Errorf("snapshotPath(%q) = %q, want %q", "foo", got, want)
	}
}
