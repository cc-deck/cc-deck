//go:build darwin

package procinfo

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// darwinZombie is SZOMB from <sys/proc.h>. A zombie has no arguments or
// environment left to read and no longer runs, so it is not a client.
const darwinZombie = 5

type darwinSource struct{}

func defaultSource() Source { return darwinSource{} }

func (darwinSource) Self() int { return os.Getpid() }

// Get reads the process through kern.proc.pid and kern.procargs2.
func (darwinSource) Get(pid int) (Process, error) {
	kp, err := kinfo(pid)
	if err != nil {
		return Process{}, err
	}
	if int(kp.Eproc.Ucred.Uid) != os.Getuid() {
		return Process{}, fmt.Errorf("procinfo: process %d: %w", pid, ErrForeign)
	}
	return fromKinfo(kp)
}

// Ancestor is Get without the environment: an environment that was cut off
// does not matter for the arguments.
func (darwinSource) Ancestor(pid int) (Process, error) {
	kp, err := kinfo(pid)
	if err != nil {
		return Process{}, err
	}
	if int(kp.Eproc.Ucred.Uid) != os.Getuid() {
		return Process{}, fmt.Errorf("procinfo: process %d: %w", pid, ErrForeign)
	}
	buf, err := unix.SysctlRaw("kern.procargs2", pid)
	if err != nil {
		return Process{}, fmt.Errorf("procinfo: kern.procargs2 %d: %w", pid, err)
	}
	exe, args, _, err := ParseProcArgs2(buf)
	if err != nil && !errors.Is(err, ErrTruncatedEnv) {
		return Process{}, fmt.Errorf("procinfo: kern.procargs2 %d: %w", pid, err)
	}
	st := kp.Proc.P_starttime
	p := Process{
		PID:       pid,
		PPID:      int(kp.Eproc.Ppid),
		StartTime: int64(st.Sec)*1_000_000 + int64(st.Usec),
		Exe:       exe,
		Args:      args,
	}
	if p.Exe == "" {
		p.Exe = comm(kp)
	}
	return p, nil
}

func kinfo(pid int) (*unix.KinfoProc, error) {
	if pid <= 0 {
		return nil, fmt.Errorf("procinfo: invalid pid %d", pid)
	}
	kp, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil {
		return nil, fmt.Errorf("procinfo: kern.proc.pid %d: %w", pid, err)
	}
	// An exited pid yields a zeroed record rather than an error.
	if int(kp.Proc.P_pid) != pid || kp.Proc.P_stat == darwinZombie {
		return nil, fmt.Errorf("procinfo: process %d not found", pid)
	}
	return kp, nil
}

// ListUser lists the current uid's processes through kern.proc.uid and reads
// arguments and environment only for those whose command name is "codex".
//
// A codex process that still runs but cannot be read makes the whole listing
// fail: silently leaving it out could turn an ambiguous match into a wrong
// one. A process that exited between the listing and the read is skipped.
func (darwinSource) ListUser() ([]Process, error) {
	kps, err := unix.SysctlKinfoProcSlice("kern.proc.uid", os.Getuid())
	if err != nil {
		return nil, fmt.Errorf("procinfo: kern.proc.uid: %w", err)
	}
	var out []Process
	for i := range kps {
		kp := &kps[i]
		if comm(kp) != CodexComm || kp.Proc.P_stat == darwinZombie {
			continue
		}
		p, err := fromKinfo(kp)
		if err != nil {
			if cur, kerr := kinfo(int(kp.Proc.P_pid)); kerr != nil ||
				cur.Proc.P_starttime != kp.Proc.P_starttime {
				continue // exited meanwhile
			}
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func comm(kp *unix.KinfoProc) string {
	c := kp.Proc.P_comm[:]
	if i := bytes.IndexByte(c, 0); i >= 0 {
		c = c[:i]
	}
	return string(c)
}

func fromKinfo(kp *unix.KinfoProc) (Process, error) {
	pid := int(kp.Proc.P_pid)
	st := kp.Proc.P_starttime
	p := Process{
		PID:       pid,
		PPID:      int(kp.Eproc.Ppid),
		StartTime: int64(st.Sec)*1_000_000 + int64(st.Usec),
	}
	buf, err := unix.SysctlRaw("kern.procargs2", pid)
	if err != nil {
		return Process{}, fmt.Errorf("procinfo: kern.procargs2 %d: %w", pid, err)
	}
	exe, args, env, err := ParseProcArgs2(buf)
	if err != nil {
		// A partial environment could lack CODEX_HOME or PWD and so
		// describe a different client than the real one.
		return Process{}, fmt.Errorf("procinfo: kern.procargs2 %d: %w", pid, err)
	}
	p.Exe, p.Args, p.Env = exe, args, env
	if p.Exe == "" {
		p.Exe = comm(kp)
	}
	return p, nil
}
