package ssh

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// fakeSSHScript is a POSIX sh script fragment that logs all arguments it was
// invoked with and then dispatches on the final argument (the remote
// command) so tests can simulate different ssh responses.
const fakeSSHScript = `
echo "$@" > "$(dirname "$0")/args.log"
for last; do :; done
case "$last" in
  "echo ok") echo ok ;;
  "uname -s -m") echo "Linux x86_64" ;;
  "uname -s -m-bad") echo "onlyonefield" ;;
  "fail-me") echo "boom" >&2; exit 1 ;;
  *) echo "unhandled: $last" >&2; exit 1 ;;
esac
`

func TestRun_Success(t *testing.T) {
	dir := fakeBinary(t, "ssh", fakeSSHScript)
	c := NewClient("user@host", 0, "", "", "")

	out, err := c.Run(context.Background(), "echo ok")
	assert.NoError(t, err)
	assert.Equal(t, "ok", out)

	log := readArgsLog(t, dir)
	assert.Contains(t, log, "-o StrictHostKeyChecking=accept-new")
	assert.Contains(t, log, "-o BatchMode=yes")
	assert.Contains(t, log, "user@host -- echo ok")
}

func TestRun_Error(t *testing.T) {
	fakeBinary(t, "ssh", fakeSSHScript)
	c := NewClient("user@host", 0, "", "", "")

	out, err := c.Run(context.Background(), "fail-me")
	assert.Error(t, err)
	assert.Empty(t, out)
	assert.Contains(t, err.Error(), "boom")
}

func TestRun_SSHBinaryNotFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	c := NewClient("user@host", 0, "", "", "")

	_, err := c.Run(context.Background(), "echo ok")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ssh binary not found")
}

func TestCheck_Success(t *testing.T) {
	fakeBinary(t, "ssh", fakeSSHScript)
	c := NewClient("user@host", 0, "", "", "")

	err := c.Check(context.Background())
	assert.NoError(t, err)
}

func TestCheck_Failure(t *testing.T) {
	fakeBinary(t, "ssh", `echo "boom" >&2; exit 1`)
	c := NewClient("user@host", 0, "", "", "")

	err := c.Check(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SSH connectivity check failed for user@host")
}

func TestRemoteInfo_Success(t *testing.T) {
	fakeBinary(t, "ssh", fakeSSHScript)
	c := NewClient("user@host", 0, "", "", "")

	osName, arch, err := c.RemoteInfo(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "linux", osName)
	assert.Equal(t, "amd64", arch)
}

func TestRemoteInfo_RunError(t *testing.T) {
	fakeBinary(t, "ssh", `echo "boom" >&2; exit 1`)
	c := NewClient("user@host", 0, "", "", "")

	_, _, err := c.RemoteInfo(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "detecting remote OS/arch")
}

func TestRemoteInfo_UnexpectedOutput(t *testing.T) {
	fakeBinary(t, "ssh", `echo "onlyonefield"`)
	c := NewClient("user@host", 0, "", "", "")

	_, _, err := c.RemoteInfo(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected uname output")
}

func TestUpload_Success(t *testing.T) {
	dir := fakeBinary(t, "scp", `echo "$@" > "$(dirname "$0")/args.log"`)
	c := NewClient("user@host", 2222, "/path/key", "bastion", "/path/config")

	err := c.Upload(context.Background(), "/local/file", "/remote/file")
	assert.NoError(t, err)

	log := readArgsLog(t, dir)
	assert.Contains(t, log, "-r")
	assert.Contains(t, log, "-F /path/config")
	assert.Contains(t, log, "-P 2222")
	assert.Contains(t, log, "-i /path/key")
	assert.Contains(t, log, "-J bastion")
	assert.Contains(t, log, "/local/file user@host:/remote/file")
}

func TestUpload_Error(t *testing.T) {
	fakeBinary(t, "scp", `echo "boom" >&2; exit 1`)
	c := NewClient("user@host", 0, "", "", "")

	err := c.Upload(context.Background(), "/local/file", "/remote/file")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "scp upload failed")
}

func TestUpload_SCPBinaryNotFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	c := NewClient("user@host", 0, "", "", "")

	err := c.Upload(context.Background(), "/local/file", "/remote/file")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "scp binary not found")
}

func TestDownload_Success(t *testing.T) {
	dir := fakeBinary(t, "scp", `echo "$@" > "$(dirname "$0")/args.log"`)
	c := NewClient("user@host", 0, "", "", "")

	err := c.Download(context.Background(), "/remote/file", "/local/file")
	assert.NoError(t, err)

	log := readArgsLog(t, dir)
	assert.Contains(t, log, "user@host:/remote/file /local/file")
}

func TestDownload_Error(t *testing.T) {
	fakeBinary(t, "scp", `echo "boom" >&2; exit 1`)
	c := NewClient("user@host", 0, "", "", "")

	err := c.Download(context.Background(), "/remote/file", "/local/file")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "scp download failed")
}

func TestDownload_SCPBinaryNotFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	c := NewClient("user@host", 0, "", "", "")

	err := c.Download(context.Background(), "/remote/file", "/local/file")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "scp binary not found")
}

func TestRsync_Push_Success(t *testing.T) {
	dir := fakeBinary(t, "rsync", `echo "$@" > "$(dirname "$0")/args.log"`)
	c := NewClient("user@host", 2222, "/path/key", "bastion", "/path/config")

	err := c.Rsync(context.Background(), "/local/dir", "/remote/dir", []string{".git", "node_modules"}, true)
	assert.NoError(t, err)

	log := readArgsLog(t, dir)
	assert.Contains(t, log, "-avz")
	assert.Contains(t, log, "--progress")
	assert.Contains(t, log, "-e ssh -F")
	assert.Contains(t, log, "--exclude .git")
	assert.Contains(t, log, "--exclude node_modules")
	assert.Contains(t, log, "/local/dir /remote/dir")
}

func TestRsync_Pull_Success(t *testing.T) {
	dir := fakeBinary(t, "rsync", `echo "$@" > "$(dirname "$0")/args.log"`)
	c := NewClient("user@host", 0, "", "", "")

	err := c.Rsync(context.Background(), "/remote/dir", "/local/dir", nil, false)
	assert.NoError(t, err)

	log := readArgsLog(t, dir)
	assert.Contains(t, log, "/remote/dir /local/dir")
}

func TestRsync_Error(t *testing.T) {
	fakeBinary(t, "rsync", `echo "boom" >&2; exit 1`)
	c := NewClient("user@host", 0, "", "", "")

	err := c.Rsync(context.Background(), "/local/dir", "/remote/dir", nil, true)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rsync failed")
}

// isolatedPATH creates a directory containing only the given fake
// executables (keyed by name) and points PATH exclusively at it, so
// binaries not installed there (e.g. rsync) are genuinely unresolvable via
// exec.LookPath.
func isolatedPATH(t *testing.T, bins map[string]string) string {
	t.Helper()

	dir := t.TempDir()
	logPath := filepath.Join(dir, "args.log")
	for name, script := range bins {
		scriptPath := filepath.Join(dir, name)
		full := "#!/bin/sh\nLOG=\"" + logPath + "\"\n" + script + "\n"
		if err := os.WriteFile(scriptPath, []byte(full), 0o755); err != nil {
			t.Fatalf("write fake %s script: %v", name, err)
		}
	}
	t.Setenv("PATH", dir)
	return logPath
}

func TestRsync_FallbackToScp_Push(t *testing.T) {
	logPath := isolatedPATH(t, map[string]string{
		"scp": `echo "$@" > "$LOG"`,
	})
	c := NewClient("user@host", 0, "", "", "")

	err := c.Rsync(context.Background(), "/local/dir", "/remote/dir", nil, true)
	assert.NoError(t, err)
	assert.FileExists(t, logPath)
}

func TestRsync_FallbackToScp_Pull(t *testing.T) {
	logPath := isolatedPATH(t, map[string]string{
		"scp": `echo "$@" > "$LOG"`,
	})
	c := NewClient("user@host", 0, "", "", "")

	err := c.Rsync(context.Background(), "/remote/dir", "/local/dir", nil, false)
	assert.NoError(t, err)
	assert.FileExists(t, logPath)
}

func TestRunInteractive_OnDetachEscape(t *testing.T) {
	fakeBinary(t, "ssh", `exit 0`)
	c := NewClient("user@host", 0, "", "", "")
	c.OnDetachEscape = "\x1b[?1049l"

	attached := false
	c.OnAttach = func() { attached = true }

	err := c.RunInteractive("zellij attach")
	assert.NoError(t, err)
	assert.True(t, attached)
}

func TestRunInteractive_SSHBinaryNotFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	c := NewClient("user@host", 0, "", "", "")

	err := c.RunInteractive("zellij attach")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ssh binary not found")
}
