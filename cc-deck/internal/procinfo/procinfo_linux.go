//go:build linux

package procinfo

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// linuxSource reads /proc. The root is a field so tests can point it at a
// fixture tree.
type linuxSource struct {
	root string
	uid  int
}

func defaultSource() Source { return linuxSource{root: "/proc", uid: os.Getuid()} }

func (linuxSource) Self() int { return os.Getpid() }

// Get reads /proc/<pid>/{stat,cmdline,environ,cwd}.
func (s linuxSource) Get(pid int) (Process, error) {
	if pid <= 0 {
		return Process{}, fmt.Errorf("procinfo: invalid pid %d", pid)
	}
	st, err := s.own(pid)
	if err != nil {
		return Process{}, err
	}
	return s.details(st)
}

// Ancestor reads only /proc/<pid>/{status,stat,cmdline}, which stay
// readable for a non-dumpable process of the same user.
func (s linuxSource) Ancestor(pid int) (Process, error) {
	if pid <= 0 {
		return Process{}, fmt.Errorf("procinfo: invalid pid %d", pid)
	}
	st, err := s.own(pid)
	if err != nil {
		return Process{}, err
	}
	cmdline, err := os.ReadFile(filepath.Join(s.root, strconv.Itoa(pid), "cmdline"))
	if err != nil {
		return Process{}, fmt.Errorf("procinfo: cmdline %d: %w", pid, err)
	}
	exe, args := parseArgvBlock(cmdline)
	if exe == "" {
		exe = st.comm
	}
	return Process{PID: st.pid, PPID: st.ppid, StartTime: st.start, Exe: exe, Args: args}, nil
}

// own checks that pid belongs to the current user and returns its stat.
//
// An unreadable status (hidepid=noaccess, an LSM) is ambiguous: the kernel
// also denies it for a non-dumpable process of the same user, such as a
// hardened Codex daemon. It counts as another user's only when the
// /proc/<pid> directory names an owner that is neither this user nor root
// (a non-dumpable process of this user shows root there); otherwise the
// error is returned, so the caller drops the event instead of trusting a
// pane identity that may be inherited.
func (s linuxSource) own(pid int) (procStat, error) {
	uid, err := s.owner(pid)
	if errors.Is(err, fs.ErrPermission) {
		if fi, serr := os.Lstat(filepath.Join(s.root, strconv.Itoa(pid))); serr == nil {
			if sys, ok := fi.Sys().(*syscall.Stat_t); ok && int(sys.Uid) != s.uid && sys.Uid != 0 {
				return procStat{}, fmt.Errorf("procinfo: process %d: %w", pid, ErrForeign)
			}
		}
		return procStat{}, err
	}
	if err != nil {
		return procStat{}, err
	}
	if uid != s.uid {
		return procStat{}, fmt.Errorf("procinfo: process %d: %w", pid, ErrForeign)
	}
	return s.stat(pid)
}

// owner returns the effective uid of the process from the Uid line of
// /proc/<pid>/status. The owner of the /proc/<pid> directory is not used:
// for a non-dumpable process the kernel reports root as the owner of every
// /proc/<pid> inode, which would make the user's own process look foreign.
func (s linuxSource) owner(pid int) (int, error) {
	data, err := os.ReadFile(filepath.Join(s.root, strconv.Itoa(pid), "status"))
	if err != nil {
		return 0, fmt.Errorf("procinfo: status %d: %w", pid, err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		rest, ok := strings.CutPrefix(line, "Uid:")
		if !ok {
			continue
		}
		f := strings.Fields(rest)
		if len(f) < 2 {
			break
		}
		uid, err := strconv.Atoi(f[1])
		if err != nil {
			break
		}
		return uid, nil
	}
	return 0, fmt.Errorf("procinfo: status %d: no Uid line", pid)
}

// ListUser scans the numeric entries of the proc root, keeps those whose
// stat comm is "codex" and whose effective uid is the current one, and only
// then reads their arguments, environment and working directory.
//
// A codex process that still runs but cannot be read makes the listing fail,
// so an ambiguous match never turns into a wrong one. A process that exited
// meanwhile is skipped.
func (s linuxSource) ListUser() ([]Process, error) {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, fmt.Errorf("procinfo: reading %s: %w", s.root, err)
	}
	var out []Process
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid <= 0 || !e.IsDir() {
			continue
		}
		st, err := s.stat(pid)
		if err != nil || st.comm != CodexComm {
			continue // exited, or not codex
		}
		if uid, err := s.owner(pid); err != nil || uid != s.uid {
			continue // exited, or another user's
		}
		p, err := s.details(st)
		if err != nil {
			if cur, serr := s.stat(pid); serr != nil || cur.start != st.start {
				continue // exited meanwhile
			}
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

type procStat struct {
	pid   int
	ppid  int
	comm  string
	start int64
}

var errNotRunning = errors.New("procinfo: process not running")

func (s linuxSource) stat(pid int) (procStat, error) {
	data, err := os.ReadFile(filepath.Join(s.root, strconv.Itoa(pid), "stat"))
	if err != nil {
		return procStat{}, fmt.Errorf("procinfo: stat %d: %w", pid, err)
	}
	st, err := parseStat(string(data))
	if err != nil {
		return procStat{}, fmt.Errorf("procinfo: stat %d: %w", pid, err)
	}
	return st, nil
}

// parseStat parses /proc/<pid>/stat. The comm field is wrapped in
// parentheses and may itself contain spaces and parentheses, so the fields
// after it are located from the last closing parenthesis.
func parseStat(s string) (procStat, error) {
	open := strings.IndexByte(s, '(')
	closing := strings.LastIndexByte(s, ')')
	if open < 0 || closing < open {
		return procStat{}, errors.New("malformed stat")
	}
	pid, err := strconv.Atoi(strings.TrimSpace(s[:open]))
	if err != nil {
		return procStat{}, fmt.Errorf("malformed pid: %w", err)
	}
	fields := strings.Fields(s[closing+1:])
	// fields[0] is field 3 (state), fields[1] field 4 (ppid), fields[19]
	// field 22 (starttime).
	if len(fields) < 20 {
		return procStat{}, errors.New("short stat")
	}
	if fields[0] == "Z" || fields[0] == "X" {
		return procStat{}, errNotRunning
	}
	ppid, err := strconv.Atoi(fields[1])
	if err != nil {
		return procStat{}, fmt.Errorf("malformed ppid: %w", err)
	}
	start, err := strconv.ParseInt(fields[19], 10, 64)
	if err != nil {
		return procStat{}, fmt.Errorf("malformed starttime: %w", err)
	}
	return procStat{pid: pid, ppid: ppid, comm: s[open+1 : closing], start: start}, nil
}

func (s linuxSource) details(st procStat) (Process, error) {
	dir := filepath.Join(s.root, strconv.Itoa(st.pid))
	cmdline, err := os.ReadFile(filepath.Join(dir, "cmdline"))
	if err != nil {
		return Process{}, fmt.Errorf("procinfo: cmdline %d: %w", st.pid, err)
	}
	environ, err := os.ReadFile(filepath.Join(dir, "environ"))
	if err != nil {
		return Process{}, fmt.Errorf("procinfo: environ %d: %w", st.pid, err)
	}
	exe, args := parseArgvBlock(cmdline)
	if exe == "" {
		exe = st.comm
	}
	cwd, _ := os.Readlink(filepath.Join(dir, "cwd"))
	return Process{
		PID:       st.pid,
		PPID:      st.ppid,
		StartTime: st.start,
		Exe:       exe,
		Args:      args,
		Env:       parseEnvBlock(environ),
		Cwd:       cwd,
	}, nil
}
