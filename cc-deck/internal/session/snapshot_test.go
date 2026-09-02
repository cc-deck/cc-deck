package session

import (
	"testing"
	"time"

	"github.com/cc-deck/cc-deck/internal/xdg"
)

// withTempStateHome points xdg.StateHome at a temporary directory for the
// duration of the test and restores the original value afterward. This
// isolates SessionsDir() so tests never touch a real user's state directory.
func withTempStateHome(t *testing.T) string {
	t.Helper()
	orig := xdg.StateHome
	dir := t.TempDir()
	xdg.StateHome = dir
	t.Cleanup(func() {
		xdg.StateHome = orig
	})
	return dir
}

func TestSessionsDir_UsesXDGStateHome(t *testing.T) {
	dir := withTempStateHome(t)
	got := SessionsDir()
	want := dir + "/cc-deck/sessions"
	if got != want {
		t.Errorf("SessionsDir() = %q, want %q", got, want)
	}
}

func TestSaveAndLoadSnapshot_RoundTrip(t *testing.T) {
	withTempStateHome(t)

	snap := &Snapshot{
		Version: 1,
		Name:    "test-snap",
		SavedAt: time.Now().UTC().Truncate(time.Second),
		Sessions: []SessionEntry{
			{TabName: "tab1", WorkingDir: "/home/user/proj", SessionID: "abc123", DisplayName: "proj", GitBranch: "main"},
		},
	}

	if err := SaveSnapshot(snap); err != nil {
		t.Fatalf("SaveSnapshot failed: %v", err)
	}

	loaded, err := LoadSnapshot("test-snap")
	if err != nil {
		t.Fatalf("LoadSnapshot failed: %v", err)
	}

	if loaded.Name != snap.Name {
		t.Errorf("Name = %q, want %q", loaded.Name, snap.Name)
	}
	if len(loaded.Sessions) != 1 {
		t.Fatalf("len(Sessions) = %d, want 1", len(loaded.Sessions))
	}
	if loaded.Sessions[0].SessionID != "abc123" {
		t.Errorf("SessionID = %q, want %q", loaded.Sessions[0].SessionID, "abc123")
	}
	if !loaded.SavedAt.Equal(snap.SavedAt) {
		t.Errorf("SavedAt = %v, want %v", loaded.SavedAt, snap.SavedAt)
	}
}

func TestLoadSnapshot_MissingFileReturnsError(t *testing.T) {
	withTempStateHome(t)

	if _, err := LoadSnapshot("does-not-exist"); err == nil {
		t.Fatal("expected error loading missing snapshot, got nil")
	}
}

func TestListSnapshots_EmptyDirReturnsNil(t *testing.T) {
	withTempStateHome(t)

	infos, err := ListSnapshots()
	if err != nil {
		t.Fatalf("ListSnapshots failed: %v", err)
	}
	if len(infos) != 0 {
		t.Errorf("len(infos) = %d, want 0", len(infos))
	}
}

func TestListSnapshots_SortedNewestFirst(t *testing.T) {
	withTempStateHome(t)

	older := &Snapshot{Version: 1, Name: "older", SavedAt: time.Now().Add(-time.Hour).UTC()}
	newer := &Snapshot{Version: 1, Name: "newer", SavedAt: time.Now().UTC()}

	if err := SaveSnapshot(older); err != nil {
		t.Fatalf("saving older: %v", err)
	}
	if err := SaveSnapshot(newer); err != nil {
		t.Fatalf("saving newer: %v", err)
	}

	infos, err := ListSnapshots()
	if err != nil {
		t.Fatalf("ListSnapshots failed: %v", err)
	}
	if len(infos) != 2 {
		t.Fatalf("len(infos) = %d, want 2", len(infos))
	}
	if infos[0].Name != "newer" || infos[1].Name != "older" {
		t.Errorf("order = [%s, %s], want [newer, older]", infos[0].Name, infos[1].Name)
	}
}

func TestLatestSnapshot_ReturnsMostRecent(t *testing.T) {
	withTempStateHome(t)

	if err := SaveSnapshot(&Snapshot{Version: 1, Name: "old", SavedAt: time.Now().Add(-time.Hour).UTC()}); err != nil {
		t.Fatalf("saving old: %v", err)
	}
	if err := SaveSnapshot(&Snapshot{Version: 1, Name: "new", SavedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("saving new: %v", err)
	}

	latest, err := LatestSnapshot()
	if err != nil {
		t.Fatalf("LatestSnapshot failed: %v", err)
	}
	if latest.Name != "new" {
		t.Errorf("Name = %q, want %q", latest.Name, "new")
	}
}

func TestLatestSnapshot_NoneReturnsError(t *testing.T) {
	withTempStateHome(t)

	if _, err := LatestSnapshot(); err == nil {
		t.Fatal("expected error when no snapshots exist, got nil")
	}
}

func TestRemoveSnapshot_DeletesFile(t *testing.T) {
	withTempStateHome(t)

	if err := SaveSnapshot(&Snapshot{Version: 1, Name: "to-remove", SavedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("saving snapshot: %v", err)
	}

	if err := RemoveSnapshot("to-remove"); err != nil {
		t.Fatalf("RemoveSnapshot failed: %v", err)
	}

	if _, err := LoadSnapshot("to-remove"); err == nil {
		t.Fatal("expected snapshot to be gone after RemoveSnapshot")
	}
}

func TestRemoveSnapshot_MissingReturnsError(t *testing.T) {
	withTempStateHome(t)

	if err := RemoveSnapshot("nope"); err == nil {
		t.Fatal("expected error removing nonexistent snapshot, got nil")
	}
}

func TestRemoveAllSnapshots_RemovesEverything(t *testing.T) {
	withTempStateHome(t)

	for _, name := range []string{"a", "b", "c"} {
		if err := SaveSnapshot(&Snapshot{Version: 1, Name: name, SavedAt: time.Now().UTC()}); err != nil {
			t.Fatalf("saving %s: %v", name, err)
		}
	}

	count, err := RemoveAllSnapshots()
	if err != nil {
		t.Fatalf("RemoveAllSnapshots failed: %v", err)
	}
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}

	infos, err := ListSnapshots()
	if err != nil {
		t.Fatalf("ListSnapshots failed: %v", err)
	}
	if len(infos) != 0 {
		t.Errorf("len(infos) after RemoveAllSnapshots = %d, want 0", len(infos))
	}
}

func TestRemoveAllSnapshots_NoDirReturnsZero(t *testing.T) {
	withTempStateHome(t)

	count, err := RemoveAllSnapshots()
	if err != nil {
		t.Fatalf("RemoveAllSnapshots failed: %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}

func TestNextSnapshotName_IncrementsFromExisting(t *testing.T) {
	withTempStateHome(t)

	for _, name := range []string{"snapshot-1", "snapshot-3", "snapshot-2"} {
		if err := SaveSnapshot(&Snapshot{Version: 1, Name: name, SavedAt: time.Now().UTC()}); err != nil {
			t.Fatalf("saving %s: %v", name, err)
		}
	}

	got := nextSnapshotName()
	if got != "snapshot-4" {
		t.Errorf("nextSnapshotName() = %q, want %q", got, "snapshot-4")
	}
}

func TestNextSnapshotName_FirstIsOne(t *testing.T) {
	withTempStateHome(t)

	got := nextSnapshotName()
	if got != "snapshot-1" {
		t.Errorf("nextSnapshotName() = %q, want %q", got, "snapshot-1")
	}
}
