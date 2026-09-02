package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

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
