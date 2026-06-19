package imageprobe

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// GenerateProbeScript builds a POSIX shell script that detects OS info,
// package manager, tool presence+version, user info, and shell availability.
// Output is JSON-per-line per contracts/probe-script.md.
func GenerateProbeScript(tools []ProbeToolEntry) string {
	var sb strings.Builder

	sb.WriteString("#!/bin/sh\n")

	// JSON value sanitizer: strip quotes and backslashes from values
	sb.WriteString(`
_jsafe() { printf '%s' "$1" | tr -d '"\\\n\r\t'; }
`)

	// OS detection from /etc/os-release
	sb.WriteString(`
_os_id="" _os_id_like="" _os_name="" _os_version=""
if [ -f /etc/os-release ]; then
  while IFS='=' read -r key val; do
    val=$(echo "$val" | sed 's/^"//;s/"$//')
    case "$key" in
      ID) _os_id="$val" ;;
      ID_LIKE) _os_id_like="$val" ;;
      NAME) _os_name="$val" ;;
      VERSION_ID) _os_version="$val" ;;
    esac
  done < /etc/os-release
fi
printf '{"type":"os","id":"%s","id_like":"%s","name":"%s","version":"%s"}\n' \
  "$(_jsafe "$_os_id")" "$(_jsafe "$_os_id_like")" "$(_jsafe "$_os_name")" "$(_jsafe "$_os_version")"
`)

	// Package manager detection
	sb.WriteString(`
_pm="" _pm_path=""
for pm in dnf apt-get apk yum; do
  p=$(command -v "$pm" 2>/dev/null) && { _pm="$pm"; _pm_path="$p"; break; }
done
printf '{"type":"pkgmgr","name":"%s","path":"%s"}\n' "$(_jsafe "$_pm")" "$(_jsafe "$_pm_path")"
`)

	// Tool detection
	for _, tool := range tools {
		name := shellEscape(tool.Name)
		if name == "" {
			continue
		}
		sb.WriteString(fmt.Sprintf(`
p=$(command -v %s 2>/dev/null)
if [ -n "$p" ]; then
  v=$(%s --version 2>&1 | head -1)
  [ -z "$v" ] && v=$(%s version 2>&1 | head -1)
  [ -z "$v" ] && v=$(%s -v 2>&1 | head -1)
  printf '{"type":"tool","name":"%s","path":"%%s","version":"%%s","present":true}\n' "$p" "$(_jsafe "$v")"
else
  printf '{"type":"tool","name":"%s","path":"","version":"","present":false}\n'
fi
`, name, name, name, name, name, name))
	}

	// User info
	sb.WriteString(`
_u=$(id -un 2>/dev/null || whoami 2>/dev/null || echo "unknown")
_uid=$(id -u 2>/dev/null || echo "-1")
_home="$HOME"
_shell="$SHELL"
printf '{"type":"user","name":"%s","uid":%s,"home":"%s","shell":"%s"}\n' \
  "$(_jsafe "$_u")" "$_uid" "$(_jsafe "$_home")" "$(_jsafe "$_shell")"
`)

	// Shell availability
	sb.WriteString(`
_shells=""
for s in bash zsh sh; do
  command -v "$s" >/dev/null 2>&1 && _shells="${_shells}\"${s}\","
done
_shells=$(echo "$_shells" | sed 's/,$//')
printf '{"type":"shells","available":[%s]}\n' "$_shells"
`)

	return sb.String()
}

func shellEscape(s string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			return r
		}
		return -1
	}, s)
}

// RunProbe executes the probe script inside the given container image
// and returns parsed results. Uses context for timeout enforcement.
func RunProbe(ctx context.Context, imageRef string, tools []ProbeToolEntry) (*ProbeResult, error) {
	script := GenerateProbeScript(tools)

	cmd := exec.CommandContext(ctx, "podman", "run", "--rm", "--entrypoint", "/bin/sh", imageRef, "-c", script)

	start := time.Now()
	out, err := cmd.Output()
	duration := time.Since(start)

	if err != nil {
		return nil, fmt.Errorf("probe container %s failed: %w", imageRef, err)
	}

	result, parseErr := ParseProbeOutput(string(out))
	if parseErr != nil {
		return nil, fmt.Errorf("parsing probe output: %w", parseErr)
	}

	result.ImageRef = imageRef
	result.Timestamp = time.Now()
	result.DurationMS = duration.Milliseconds()

	return result, nil
}

// ResolveDigest returns the image digest by running podman inspect.
// Pulls the image first if not available locally.
func ResolveDigest(ctx context.Context, imageRef string) (string, error) {
	// Try inspect first
	inspectCmd := exec.CommandContext(ctx, "podman", "inspect", "--format", "{{.Digest}}", imageRef)
	out, err := inspectCmd.Output()
	if err == nil {
		digest := strings.TrimSpace(string(out))
		if digest != "" && digest != "<no value>" {
			return digest, nil
		}
	}

	// Pull and retry
	pullCmd := exec.CommandContext(ctx, "podman", "pull", "-q", imageRef)
	if pullErr := pullCmd.Run(); pullErr != nil {
		return "", fmt.Errorf("pulling image %s: %w", imageRef, pullErr)
	}

	inspectCmd2 := exec.CommandContext(ctx, "podman", "inspect", "--format", "{{.Digest}}", imageRef)
	out2, err2 := inspectCmd2.Output()
	if err2 != nil {
		return "", fmt.Errorf("inspecting image %s after pull: %w", imageRef, err2)
	}

	digest := strings.TrimSpace(string(out2))
	if digest == "" || digest == "<no value>" {
		return "", fmt.Errorf("no digest found for image %s", imageRef)
	}

	return digest, nil
}
