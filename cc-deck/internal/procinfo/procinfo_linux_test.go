//go:build linux

package procinfo

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeProc writes /proc/<pid> fixture files under root.
type fakeProc struct {
	pid, ppid int
	comm      string
	state     string
	start     int64
	argv      []string
	env       []string // nil means: no environ file
	cwd       string
	uid       *int // nil means: the current user
}

func (f fakeProc) write(t *testing.T, root string) {
	t.Helper()
	dir := filepath.Join(root, fmt.Sprint(f.pid))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	state := f.state
	if state == "" {
		state = "S"
	}
	// Fields 3..22: state ppid pgrp session tty_nr tpgid flags minflt
	// cminflt majflt cmajflt utime stime cutime cstime priority nice
	// num_threads itrealvalue starttime, followed by more fields.
	stat := fmt.Sprintf("%d (%s) %s %d 1 1 0 -1 4194304 0 0 0 0 0 0 0 0 20 0 1 0 %d 1000 10 18446744073709551615",
		f.pid, f.comm, state, f.ppid, f.start)
	mustWrite(t, filepath.Join(dir, "stat"), stat)
	uid := os.Getuid()
	if f.uid != nil {
		uid = *f.uid
	}
	mustWrite(t, filepath.Join(dir, "status"), fmt.Sprintf("Name:\t%s\nUid:\t%d\t%d\t%d\t%d\n", f.comm, uid, uid, uid, uid))
	mustWrite(t, filepath.Join(dir, "cmdline"), strings.Join(f.argv, "\x00")+"\x00")
	if f.env != nil {
		mustWrite(t, filepath.Join(dir, "environ"), strings.Join(f.env, "\x00")+"\x00")
	}
	if f.cwd != "" {
		if err := os.Symlink(f.cwd, filepath.Join(dir, "cwd")); err != nil {
			t.Fatal(err)
		}
	}
}

func mustWrite(t *testing.T, path, data string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func testSource(root string) linuxSource { return newLinuxSource(root, os.Getuid()) }

func TestLinuxGet_CommWithSpacesAndParens(t *testing.T) {
	root := t.TempDir()
	fakeProc{pid: 42, ppid: 7, comm: "we (ird) name", start: 123456,
		argv: []string{"/usr/bin/codex", "resume"}, env: []string{"PWD=/w", "A=b=c"}, cwd: "/w"}.write(t, root)

	p, err := testSource(root).Get(42)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if p.PID != 42 || p.PPID != 7 || p.StartTime != 123456 {
		t.Errorf("got pid=%d ppid=%d start=%d", p.PID, p.PPID, p.StartTime)
	}
	if p.Comm != "we (ird) name" || p.StartUnix != 0 {
		t.Errorf("comm/startUnix = %q/%d, want fixture comm and 0 without btime", p.Comm, p.StartUnix)
	}
	if p.Exe != "/usr/bin/codex" || len(p.Args) != 1 || p.Args[0] != "resume" {
		t.Errorf("got exe=%q args=%q", p.Exe, p.Args)
	}
	if p.Env["PWD"] != "/w" || p.Env["A"] != "b=c" {
		t.Errorf("env: %v", p.Env)
	}
	if p.Cwd != "/w" {
		t.Errorf("cwd: got %q", p.Cwd)
	}
}

func TestLinuxStartUnixAndListUserMatching(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "stat"), "cpu 1 2 3\nbtime 1700000000\n")
	fakeProc{pid: 10, ppid: 1, comm: "2.1.302", start: 250, argv: []string{"claude"}, env: []string{"PWD=/a"}}.write(t, root)
	fakeProc{pid: 11, ppid: 1, comm: "claude", start: 399, argv: []string{"claude"}, env: []string{"PWD=/a"}}.write(t, root)
	fakeProc{pid: 12, ppid: 1, comm: "codex", start: 400, argv: []string{"codex"}, env: []string{"PWD=/a"}}.write(t, root)

	p, err := testSource(root).Get(10)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if p.StartUnix != 1700000002 {
		t.Errorf("StartUnix = %d, want 1700000002", p.StartUnix)
	}
	got, err := testSource(root).ListUserMatching(func(comm string) bool { return comm == "2.1.302" || comm == "claude" })
	if err != nil {
		t.Fatalf("ListUserMatching: %v", err)
	}
	if len(got) != 2 || got[0].PID != 10 || got[1].PID != 11 {
		t.Fatalf("matching pids = %+v, want 10 and 11", got)
	}
}

func TestLinuxBootTimeReadOnce(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "stat"), "btime 1700000000\n")
	src := testSource(root)
	mustWrite(t, filepath.Join(root, "stat"), "btime invalid\n")
	if got := src.startUnix(250); got != 1700000002 {
		t.Fatalf("cached startUnix = %d", got)
	}
	if got := testSource(root).startUnix(250); got != 0 {
		t.Fatalf("malformed btime startUnix = %d", got)
	}
}

func TestLinuxMatchingUnreadableOwnerFails(t *testing.T) {
	root := t.TempDir()
	fakeProc{pid: 10, ppid: 1, comm: "claude", start: 100, argv: []string{"claude"}, env: []string{}}.write(t, root)
	if err := os.Remove(filepath.Join(root, "10/status")); err != nil {
		t.Fatal(err)
	}
	if _, err := testSource(root).ListUserMatching(func(c string) bool { return c == "claude" }); err == nil {
		t.Fatal("live matching process with unreadable ownership must fail listing")
	}
}

func TestLinuxGet_MissingEnvironFails(t *testing.T) {
	root := t.TempDir()
	fakeProc{pid: 5, ppid: 1, comm: "codex", start: 9, argv: []string{"codex"}}.write(t, root)
	if _, err := testSource(root).Get(5); err == nil {
		t.Fatal("expected an error without an environ file")
	}
}

func TestLinuxGet_ExitedProcess(t *testing.T) {
	root := t.TempDir()
	if _, err := testSource(root).Get(99); err == nil {
		t.Fatal("expected an error for a missing process")
	}
	fakeProc{pid: 98, ppid: 1, comm: "codex", state: "Z", start: 1, argv: []string{"codex"}, env: []string{}}.write(t, root)
	if _, err := testSource(root).Get(98); err == nil {
		t.Fatal("expected an error for a zombie")
	}
}

func TestLinuxListUser_FiltersByComm(t *testing.T) {
	root := t.TempDir()
	fakeProc{pid: 10, ppid: 1, comm: "codex", start: 100, argv: []string{"codex"}, env: []string{"PWD=/a"}}.write(t, root)
	fakeProc{pid: 11, ppid: 1, comm: "bash", start: 101, argv: []string{"bash"}, env: []string{"PWD=/a"}}.write(t, root)
	fakeProc{pid: 12, ppid: 1, comm: "codex", state: "Z", start: 102, argv: []string{"codex"}, env: []string{}}.write(t, root)
	// Non-numeric entries such as /proc/self or /proc/sys are ignored.
	if err := os.MkdirAll(filepath.Join(root, "sys"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := testSource(root).ListUser()
	if err != nil {
		t.Fatalf("ListUser: %v", err)
	}
	if len(got) != 1 || got[0].PID != 10 {
		t.Fatalf("got %+v, want only pid 10", got)
	}
}

// A running codex process whose environment cannot be read must fail the
// listing instead of silently disappearing from the ambiguity count.
func TestLinuxListUser_UnreadableRunningClientFails(t *testing.T) {
	root := t.TempDir()
	fakeProc{pid: 10, ppid: 1, comm: "codex", start: 100, argv: []string{"codex"}, env: []string{"PWD=/a"}}.write(t, root)
	fakeProc{pid: 20, ppid: 1, comm: "codex", start: 200, argv: []string{"codex"}}.write(t, root)

	if _, err := testSource(root).ListUser(); err == nil {
		t.Fatal("expected ListUser to fail for an unreadable running codex process")
	}
}

func TestLinuxListUser_OtherUserIgnored(t *testing.T) {
	root := t.TempDir()
	fakeProc{pid: 10, ppid: 1, comm: "codex", start: 100, argv: []string{"codex"}, env: []string{"PWD=/a"}}.write(t, root)
	src := linuxSource{root: root, uid: os.Getuid() + 1}
	got, err := src.ListUser()
	if err != nil {
		t.Fatalf("ListUser: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no processes of another uid, got %d", len(got))
	}
}

func TestParseStat_Malformed(t *testing.T) {
	for _, s := range []string{"", "12 codex S 1", "12 (codex) S 1 2 3"} {
		if _, err := parseStat(s); err == nil {
			t.Errorf("parseStat(%q): expected an error", s)
		}
	}
}

func TestLinuxGet_OtherUserIsForeign(t *testing.T) {
	root := t.TempDir()
	fakeProc{pid: 10, ppid: 1, comm: "sshd", start: 100, argv: []string{"sshd"}, env: []string{"PWD=/"}}.write(t, root)
	src := linuxSource{root: root, uid: os.Getuid() + 1}
	if _, err := src.Get(10); !errors.Is(err, ErrForeign) {
		t.Fatalf("Get: got %v, want ErrForeign", err)
	}
}

// The effective uid in status decides ownership, not the owner of the
// /proc/<pid> directory (root for non-dumpable processes).
func TestLinuxOwner_FromStatusNotDirectory(t *testing.T) {
	root := t.TempDir()
	other := os.Getuid() + 1
	fakeProc{pid: 10, ppid: 1, comm: "codex", start: 100, argv: []string{"codex"}, env: []string{"PWD=/a"}, uid: &other}.write(t, root)
	if _, err := testSource(root).Get(10); !errors.Is(err, ErrForeign) {
		t.Fatalf("Get: got %v, want ErrForeign", err)
	}
	got, err := testSource(root).ListUser()
	if err != nil || len(got) != 0 {
		t.Fatalf("ListUser: got %d processes, err %v; want none", len(got), err)
	}
}

// Ancestor needs only status, stat and cmdline: a process whose environment
// cannot be read (non-dumpable) still yields its arguments.
func TestLinuxAncestor_WithoutEnviron(t *testing.T) {
	root := t.TempDir()
	fakeProc{pid: 10, ppid: 1, comm: "codex", start: 100,
		argv: []string{"codex", "app-server", "--managed-daemon"}}.write(t, root)
	if _, err := testSource(root).Get(10); err == nil {
		t.Fatal("Get: expected an error without environ")
	}
	p, err := testSource(root).Ancestor(10)
	if err != nil {
		t.Fatalf("Ancestor: %v", err)
	}
	if p.PPID != 1 || p.StartTime != 100 || p.Exe != "codex" || len(p.Args) != 2 {
		t.Errorf("Ancestor: %+v", p)
	}
}

// An unreadable status is not taken as "another user's": under
// hidepid=noaccess the kernel denies it for a non-dumpable process of this
// user too (whose /proc/<pid> directory the fixture, owned by this user,
// stands in for), and treating that as foreign would trust an inherited
// pane identity.
func TestLinuxAncestor_UnreadableStatusIsNotForeign(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root reads files regardless of their mode")
	}
	root := t.TempDir()
	fakeProc{pid: 10, ppid: 1, comm: "codex", start: 100,
		argv: []string{"codex", "app-server", "--managed-daemon"}}.write(t, root)
	if err := os.Chmod(filepath.Join(root, "10", "status"), 0); err != nil {
		t.Fatal(err)
	}
	_, err := testSource(root).Ancestor(10)
	if err == nil || errors.Is(err, ErrForeign) {
		t.Fatalf("Ancestor: got %v, want a non-foreign error", err)
	}
}
