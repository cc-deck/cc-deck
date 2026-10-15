package procinfo

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// procargs2 builds a kern.procargs2 buffer: argc, exec path, NUL padding,
// argv, environment.
func procargs2(execPath string, pad int, argv, env []string) []byte {
	buf := binary.NativeEndian.AppendUint32(nil, uint32(len(argv)))
	buf = append(buf, execPath...)
	buf = append(buf, 0)
	for i := 0; i < pad; i++ {
		buf = append(buf, 0)
	}
	for _, a := range argv {
		buf = append(buf, a...)
		buf = append(buf, 0)
	}
	for _, e := range env {
		buf = append(buf, e...)
		buf = append(buf, 0)
	}
	return buf
}

func TestParseProcArgs2_Normal(t *testing.T) {
	buf := procargs2("/opt/codex/bin/codex", 7,
		[]string{"codex", "-C", "work dir", "resume"},
		[]string{"PWD=/Users/me/a", "ZELLIJ_PANE_ID=4", "EMPTY=", "WEIRD=a=b"})
	// Trailing NULs after the environment are common in real buffers.
	buf = append(buf, 0, 0, 0)

	exe, args, env, err := ParseProcArgs2(buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exe != "codex" {
		t.Errorf("exe: got %q, want %q", exe, "codex")
	}
	want := []string{"-C", "work dir", "resume"}
	if len(args) != len(want) {
		t.Fatalf("args: got %q, want %q", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Errorf("args[%d]: got %q, want %q", i, args[i], want[i])
		}
	}
	for k, v := range map[string]string{"PWD": "/Users/me/a", "ZELLIJ_PANE_ID": "4", "EMPTY": "", "WEIRD": "a=b"} {
		if got, ok := env[k]; !ok || got != v {
			t.Errorf("env[%s]: got (%q, %v), want %q", k, got, ok, v)
		}
	}
}

func TestParseProcArgs2_EmptyEnv(t *testing.T) {
	exe, args, env, err := ParseProcArgs2(procargs2("/bin/codex", 0, []string{"/bin/codex"}, nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exe != "/bin/codex" || len(args) != 0 || len(env) != 0 {
		t.Errorf("got exe=%q args=%q env=%v", exe, args, env)
	}
}

func TestParseProcArgs2_Truncated(t *testing.T) {
	full := procargs2("/bin/codex", 3, []string{"codex", "app-server", "--listen"}, []string{"A=1"})
	cut := bytes.Index(full, []byte("--listen")) + 3
	cases := map[string][]byte{
		"empty":       nil,
		"argc only":   full[:4],
		"inside exec": full[:8],
		"inside argv": full[:cut],
		"inside env":  full[:len(full)-1],
	}
	// Claim three arguments but deliver none.
	short := binary.NativeEndian.AppendUint32(nil, 3)
	short = append(short, "/bin/codex\x00\x00"...)
	cases["argc larger than argv"] = short
	for name, buf := range cases {
		t.Run(name, func(t *testing.T) {
			_, _, _, err := ParseProcArgs2(buf)
			if !errors.Is(err, ErrTruncated) {
				t.Fatalf("got err %v, want ErrTruncated", err)
			}
			// Only a cut inside the environment leaves the arguments whole.
			if got, want := errors.Is(err, ErrTruncatedEnv), name == "inside env"; got != want {
				t.Fatalf("ErrTruncatedEnv: got %v, want %v (err %v)", got, want, err)
			}
		})
	}

	// What parsed before the cut is still returned.
	exe, args, _, err := ParseProcArgs2(full[:cut])
	if !errors.Is(err, ErrTruncated) {
		t.Fatalf("got err %v, want ErrTruncated", err)
	}
	if exe != "codex" || len(args) < 1 || args[0] != "app-server" {
		t.Errorf("partial parse: exe=%q args=%q", exe, args)
	}
}

// The real source must at least describe the test binary itself.
func TestDefault_SelfSmoke(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("no process source on " + runtime.GOOS)
	}
	src := Default()
	p, err := src.Get(src.Self())
	if err != nil {
		t.Fatalf("Get(Self()): %v", err)
	}
	if p.PID != os.Getpid() {
		t.Errorf("pid: got %d, want %d", p.PID, os.Getpid())
	}
	if p.PPID != os.Getppid() {
		t.Errorf("ppid: got %d, want %d", p.PPID, os.Getppid())
	}
	if p.StartTime == 0 {
		t.Error("expected a nonzero start time")
	}
	self, _ := os.Executable()
	if filepath.Base(p.Exe) != filepath.Base(self) {
		t.Errorf("exe: got %q, want basename of %q", p.Exe, self)
	}
	if p.Env["PATH"] != os.Getenv("PATH") {
		t.Error("expected the environment to include PATH")
	}

	if _, err := src.ListUser(); err != nil {
		t.Errorf("ListUser: %v", err)
	}

	a, err := src.Ancestor(src.Self())
	if err != nil {
		t.Fatalf("Ancestor(Self()): %v", err)
	}
	if a.PID != p.PID || a.PPID != p.PPID || a.StartTime != p.StartTime || a.Exe != p.Exe || len(a.Env) != 0 {
		t.Errorf("Ancestor: got %+v, want the identity of Get without the environment", a)
	}
}

// Pid 1 (launchd or init) belongs to root: for any other user, Get reports
// ErrForeign instead of reading it.
func TestDefault_ForeignProcess(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("no process source on " + runtime.GOOS)
	}
	if os.Getuid() == 0 {
		t.Skip("running as root: pid 1 is not foreign")
	}
	if _, err := Default().Ancestor(1); err != nil && !errors.Is(err, ErrForeign) {
		t.Fatalf("Ancestor(1): got %v, want ErrForeign", err)
	}
	_, err := Default().Get(1)
	if err == nil {
		// In a container started with --user, pid 1 can be this user's.
		t.Skip("pid 1 belongs to this user")
	}
	if !errors.Is(err, ErrForeign) {
		t.Fatalf("Get(1): got %v, want ErrForeign", err)
	}
}
