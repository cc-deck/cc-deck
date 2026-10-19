// Package procinfo reads process data (parent, start time, arguments,
// environment, working directory) of processes owned by the current user,
// without cgo and without spawning helper processes.
//
// It exists for the Codex hook attribution in internal/cmd: a Codex hook that
// runs inside the shared app-server daemon must find the interactive Codex
// client that owns the event, which requires inspecting other processes.
package procinfo

import (
	"errors"
	"path/filepath"
)

// ErrUnsupported is returned on platforms without a process source.
var ErrUnsupported = errors.New("procinfo: process inspection is not supported on this platform")

// ErrForeign is returned by Get for a process owned by another user. Such a
// process is never read in depth: its arguments and environment are not
// this user's data, and they are usually not readable anyway.
var ErrForeign = errors.New("procinfo: process belongs to another user")

// CodexComm is the executable basename that ListUser filters on.
const CodexComm = "codex"

// Process is a snapshot of one process.
type Process struct {
	PID  int
	PPID int
	// StartTime is the platform start time of the process: microseconds
	// since the epoch on Darwin, clock ticks since boot on Linux. It is
	// opaque but comparable for the same pid, which is what guards against
	// pid reuse.
	StartTime int64
	// StartUnix is the process start time in seconds since the Unix epoch.
	// Zero means the platform could not determine it.
	StartUnix int64
	// Comm is the kernel process name used by ListUserMatching.
	Comm string
	// Exe is argv[0].
	Exe string
	// Args is argv[1:].
	Args []string
	// Env is the environment block. It is only readable for processes of
	// the same user.
	Env map[string]string
	// Cwd is the working directory from /proc/<pid>/cwd on Linux; empty on
	// Darwin, which has no cgo-free API for it.
	Cwd string
}

// Source provides process data.
type Source interface {
	// Self returns the pid of the current process.
	Self() int
	// Get returns a single process of the current user. A process of
	// another user yields an error wrapping ErrForeign.
	Get(pid int) (Process, error)
	// Ancestor returns the PID, PPID, StartTime, StartUnix, Comm, Exe and Args of a process
	// of the current user, leaving Env and Cwd empty. It needs only data
	// that stays readable when the environment is not (a non-dumpable
	// process on Linux), which is all the ancestor walk of the Codex hook
	// needs. A process of another user yields an error wrapping ErrForeign.
	Ancestor(pid int) (Process, error)
	// ListUserMatching returns current-user processes whose kernel name matches.
	// The filter runs before arguments and environment are read. A matching
	// process that still runs but cannot be read fails the whole listing, so an
	// ambiguous match cannot be used for attribution.
	ListUserMatching(match func(comm string) bool) ([]Process, error)
	// ListUser is ListUserMatching with the codex filter.
	ListUser() ([]Process, error)
}

// Default returns the source for the current platform. On unsupported
// platforms every method returns ErrUnsupported.
func Default() Source {
	return defaultSource()
}

// Basename returns the executable basename of argv[0].
func (p Process) Basename() string {
	if p.Exe == "" {
		return ""
	}
	return filepath.Base(p.Exe)
}
