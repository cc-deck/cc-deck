package share

import (
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileStoreAtomicPermissionsAndNoTokens(t *testing.T) {
	p := filepath.Join(t.TempDir(), "nested", "share.yaml")
	s := NewFileStore(p)
	op := &SharingOperation{
		ID: "x", Workspace: "demo", Session: "s", State: StateActive, CreatedAt: time.Now(),
		Invitations: []InvitationRecord{
			{Label: "brave-otter", Role: RoleInteractive, State: InvitationActive, CreatedAt: time.Now()},
			{Label: "calm-fox", Role: RoleObserver, State: InvitationActive, CreatedAt: time.Now()},
		},
	}
	require.NoError(t, s.Save(op))
	fi, e := os.Stat(p)
	require.NoError(t, e)
	require.Equal(t, os.FileMode(0600), fi.Mode().Perm())
	di, e := os.Stat(filepath.Dir(p))
	require.NoError(t, e)
	require.Equal(t, os.FileMode(0700), di.Mode().Perm())
	b, e := os.ReadFile(p)
	require.NoError(t, e)
	require.NotContains(t, string(b), "raw-secret")
	require.NotContains(t, string(b), "token")
	got, e := s.Load()
	require.NoError(t, e)
	require.Equal(t, "x", got.ID)
	require.NoError(t, s.Remove())
	got, e = s.Load()
	require.NoError(t, e)
	require.Nil(t, got)
}
func TestFileStoreRejectsCorruption(t *testing.T) {
	p := filepath.Join(t.TempDir(), "share.yaml")
	require.NoError(t, os.WriteFile(p, []byte(":"), 0600))
	_, e := NewFileStore(p).Load()
	require.Error(t, e)
}
