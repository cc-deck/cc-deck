package podman

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// captureStdout redirects os.Stdout for the duration of fn and returns
// whatever was written to it.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read captured stdout: %v", err)
	}
	return string(data)
}

func TestExecOutput_Success(t *testing.T) {
	fakePodman(t, `printf '  trimmed output  \n'`)
	out, err := ExecOutput(context.Background(), "c1", "echo hi")
	assert.NoError(t, err)
	assert.Equal(t, "trimmed output", out)
}

func TestExecOutput_Error(t *testing.T) {
	fakePodman(t, `echo "boom" >&2; exit 1`)
	out, err := ExecOutput(context.Background(), "c1", "false")
	assert.Error(t, err)
	assert.Empty(t, out)
	assert.Contains(t, err.Error(), "podman exec")
}

func TestExec_NonInteractive_Success(t *testing.T) {
	fakePodman(t, `exit 0`)
	err := Exec(context.Background(), "c1", []string{"echo", "hi"}, false)
	assert.NoError(t, err)
}

func TestExec_NonInteractive_Error(t *testing.T) {
	fakePodman(t, `exit 1`)
	err := Exec(context.Background(), "c1", []string{"false"}, false)
	assert.Error(t, err)
}

func TestExec_Interactive_PodmanNotFound(t *testing.T) {
	// Point PATH at an empty directory so podman cannot be found. This
	// exercises the LookPath failure branch without ever reaching
	// syscall.Exec (which would replace the test process).
	t.Setenv("PATH", t.TempDir())
	err := Exec(context.Background(), "c1", []string{"sh"}, true)
	assert.ErrorIs(t, err, ErrPodmanNotFound)
}

func TestExecWithCleanup_PodmanNotFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	err := ExecWithCleanup(context.Background(), "c1", []string{"sh"}, "escape")
	assert.ErrorIs(t, err, ErrPodmanNotFound)
}

func TestExecWithCleanup_PrintsCleanupEscape(t *testing.T) {
	fakePodman(t, `exit 0`)
	out := captureStdout(t, func() {
		err := ExecWithCleanup(context.Background(), "c1", []string{"echo", "hi"}, "\x1b[?1049l")
		assert.NoError(t, err)
	})
	assert.Equal(t, "\x1b[?1049l", out)
}

func TestExecWithCleanup_NoEscapeWhenEmpty(t *testing.T) {
	fakePodman(t, `exit 0`)
	out := captureStdout(t, func() {
		err := ExecWithCleanup(context.Background(), "c1", []string{"echo", "hi"}, "")
		assert.NoError(t, err)
	})
	assert.Empty(t, out)
}

func TestResolveLocalPath_ContainerPathUnchanged(t *testing.T) {
	// Paths containing ":" are container paths (e.g. "mycontainer:/path")
	// and must be returned unchanged.
	got := resolveLocalPath("mycontainer:/some/path")
	assert.Equal(t, "mycontainer:/some/path", got)
}

func TestResolveLocalPath_ExistingLocalPath(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(file, []byte("data"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	got := resolveLocalPath(file)
	// No symlinks involved, so EvalSymlinks should resolve to itself
	// (modulo OS-level path normalization).
	resolved, _ := filepath.EvalSymlinks(file)
	assert.Equal(t, resolved, got)
}

func TestResolveLocalPath_NonExistentPath(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "does-not-exist.txt")
	got := resolveLocalPath(missing)
	resolvedDir, _ := filepath.EvalSymlinks(dir)
	assert.Equal(t, filepath.Join(resolvedDir, "does-not-exist.txt"), got)
}

func TestCp_Success(t *testing.T) {
	dir := fakePodman(t, `echo "$@" > "$(dirname "$0")/args.log"`)
	srcDir := t.TempDir()
	src := filepath.Join(srcDir, "src.txt")
	if err := os.WriteFile(src, []byte("data"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	err := Cp(context.Background(), src, "mycontainer:/dst.txt")
	assert.NoError(t, err)
	log := readArgsLog(t, dir)
	assert.Contains(t, log, "cp ")
	assert.Contains(t, log, "mycontainer:/dst.txt")
}

func TestCp_Error(t *testing.T) {
	fakePodman(t, `echo "boom" >&2; exit 1`)
	err := Cp(context.Background(), "mycontainer:/src.txt", t.TempDir())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "podman cp")
	assert.Contains(t, err.Error(), "boom")
}
