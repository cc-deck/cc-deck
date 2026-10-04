package podman

import (
	"os"
	"path/filepath"
	"testing"
)

// fakePodman installs a fake "podman" executable at the front of PATH for
// the duration of the current test, so the package's exec.Command(Context)
// calls invoke the given shell script instead of a real podman binary.
// Inside script, "$(dirname "$0")" resolves to the directory containing the
// fake binary, which tests can use to write/read scratch files (e.g. to
// capture the arguments podman was invoked with).
func fakePodman(t *testing.T, script string) string {
	t.Helper()

	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "podman")
	full := "#!/bin/sh\n" + script + "\n"
	if err := os.WriteFile(scriptPath, []byte(full), 0o755); err != nil {
		t.Fatalf("write fake podman script: %v", err)
	}

	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return dir
}

// readArgsLog reads the args captured by a fake podman script that logs
// "$@" to "$(dirname "$0")/args.log".
func readArgsLog(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "args.log"))
	if err != nil {
		t.Fatalf("read args.log: %v", err)
	}
	return string(data)
}
