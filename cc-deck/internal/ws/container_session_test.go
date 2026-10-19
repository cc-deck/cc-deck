package ws

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeleteContainerZellijSession(t *testing.T) {
	for _, tc := range []struct {
		name       string
		deleteExit string
		listOutput string
		listExit   string
		wantError  bool
	}{
		{"deleted", "0", "", "99", false},
		{"already gone", "2", "No active zellij sessions found.", "1", false},
		{"other session survives", "2", "cc-deck-other", "0", false},
		{"only exited session remains", "2", "cc-deck-test (EXITED - 1s ago)", "0", false},
		{"target still active", "2", "cc-deck-test", "0", true},
		{"cannot verify removal", "2", "permission denied", "1", true},
		{"missing binary", "127", "zellij: not found", "127", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			binDir := t.TempDir()
			script := `#!/bin/sh
test "$1" = exec && test "$2" = test-container && test "$3" = zellij || exit 99
case "$4" in
  delete-session)
    test "$5" = --force && test "$6" = cc-deck-test || exit 99
    echo 'delete diagnostic' >&2
    exit "$TEST_DELETE_EXIT"
    ;;
  list-sessions)
    test "$5" = -n || exit 99
    printf '%s\n' "$TEST_LIST_OUTPUT"
    exit "$TEST_LIST_EXIT"
    ;;
  *) exit 99 ;;
esac
`
			require.NoError(t, os.WriteFile(filepath.Join(binDir, "podman"), []byte(script), 0o755))
			t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("TEST_DELETE_EXIT", tc.deleteExit)
			t.Setenv("TEST_LIST_OUTPUT", tc.listOutput)
			t.Setenv("TEST_LIST_EXIT", tc.listExit)
			err := deleteContainerZellijSession(context.Background(), "test-container", "cc-deck-test")
			if tc.wantError {
				require.ErrorContains(t, err, "delete diagnostic")
			} else {
				require.NoError(t, err)
			}
		})
	}
}
