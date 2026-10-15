package procinfo

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

// ErrTruncated reports a procargs2 buffer that ended before argc arguments
// were read. The values parsed so far are still returned.
var ErrTruncated = errors.New("procinfo: truncated procargs2 buffer")

// ErrTruncatedEnv is the ErrTruncated case where all arguments were read and
// only the environment was cut off. errors.Is(err, ErrTruncated) holds too.
var ErrTruncatedEnv = fmt.Errorf("%w (environment)", ErrTruncated)

// ParseProcArgs2 parses the buffer returned by the Darwin sysctl
// kern.procargs2.
//
// Layout: a native-endian int32 argc, the executable path (NUL-terminated),
// NUL padding, argc NUL-terminated argument strings, then NUL-terminated
// KEY=VALUE environment strings until an empty string or the end of the
// buffer.
//
// It returns argv[0] as exe, argv[1:] as args, and the environment as a map.
// A buffer that ends early yields what parsed so far plus ErrTruncated.
func ParseProcArgs2(buf []byte) (exe string, args []string, env map[string]string, err error) {
	env = make(map[string]string)
	if len(buf) < 4 {
		return "", nil, env, ErrTruncated
	}
	argc := int(int32(binary.NativeEndian.Uint32(buf[:4])))
	rest := buf[4:]

	// Executable path.
	i := bytes.IndexByte(rest, 0)
	if i < 0 {
		return "", nil, env, ErrTruncated
	}
	rest = rest[i:]
	// Skip the NUL padding after the executable path. The padding length is
	// not recorded, so an empty argv[0] is indistinguishable from padding
	// and is skipped too, shifting the arguments by one (the approach ps and
	// psutil take as well). Codex always sets a non-empty argv[0].
	for len(rest) > 0 && rest[0] == 0 {
		rest = rest[1:]
	}

	var argv []string
	for n := 0; n < argc; n++ {
		if len(rest) == 0 {
			exe, args = splitArgv(argv)
			return exe, args, env, ErrTruncated
		}
		j := bytes.IndexByte(rest, 0)
		if j < 0 {
			argv = append(argv, string(rest))
			exe, args = splitArgv(argv)
			return exe, args, env, ErrTruncated
		}
		argv = append(argv, string(rest[:j]))
		rest = rest[j+1:]
	}
	exe, args = splitArgv(argv)

	for len(rest) > 0 {
		j := bytes.IndexByte(rest, 0)
		if j < 0 {
			// The kernel terminates every string; an unterminated tail means
			// the buffer was cut inside the environment.
			return exe, args, env, ErrTruncatedEnv
		}
		kv := string(rest[:j])
		rest = rest[j+1:]
		if kv == "" {
			break
		}
		if k, v, ok := strings.Cut(kv, "="); ok && k != "" {
			env[k] = v
		}
	}
	return exe, args, env, nil
}

func splitArgv(argv []string) (string, []string) {
	if len(argv) == 0 {
		return "", nil
	}
	return argv[0], argv[1:]
}

// parseEnvBlock parses a NUL-separated KEY=VALUE block (Linux
// /proc/<pid>/environ).
func parseEnvBlock(data []byte) map[string]string {
	env := make(map[string]string)
	for _, kv := range strings.Split(string(data), "\x00") {
		if k, v, ok := strings.Cut(kv, "="); ok && k != "" {
			env[k] = v
		}
	}
	return env
}

// parseArgvBlock parses a NUL-separated argument block (Linux
// /proc/<pid>/cmdline).
func parseArgvBlock(data []byte) (string, []string) {
	s := strings.TrimSuffix(string(data), "\x00")
	if s == "" {
		return "", nil
	}
	return splitArgv(strings.Split(s, "\x00"))
}
