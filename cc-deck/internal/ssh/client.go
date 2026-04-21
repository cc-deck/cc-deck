package ssh

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// Client wraps the system ssh binary for remote operations.
type Client struct {
	Host            string
	Port            int
	IdentityFile    string
	JumpHost        string
	SSHConfig       string
	AgentForwarding  bool
	OnAttach         func()
	OnDetach         func()
	OnDetachEscape   string
}

// NewClient creates a new SSH client with the given connection parameters.
func NewClient(host string, port int, identityFile, jumpHost, sshConfig string) *Client {
	return &Client{
		Host:         host,
		Port:         port,
		IdentityFile: identityFile,
		JumpHost:     jumpHost,
		SSHConfig:    sshConfig,
	}
}

// buildArgs constructs the SSH command-line arguments from client configuration.
// Extra arguments are appended after the standard options.
func (c *Client) buildArgs(extraArgs ...string) []string {
	var args []string

	if c.SSHConfig != "" {
		args = append(args, "-F", c.SSHConfig)
	}
	if c.Port != 0 {
		args = append(args, "-p", fmt.Sprintf("%d", c.Port))
	}
	if c.IdentityFile != "" {
		args = append(args, "-i", c.IdentityFile)
	}
	if c.JumpHost != "" {
		args = append(args, "-J", c.JumpHost)
	}

	if c.AgentForwarding {
		args = append(args, "-A")
	}

	// Disable strict host key checking for non-interactive use and
	// suppress known_hosts warnings.
	args = append(args, "-o", "StrictHostKeyChecking=accept-new")
	args = append(args, "-o", "BatchMode=yes")

	args = append(args, extraArgs...)

	return args
}

// Run executes a command on the remote host and returns the combined output.
// The command runs non-interactively with a timeout controlled by the context.
func (c *Client) Run(ctx context.Context, cmd string) (string, error) {
	sshBin, err := exec.LookPath("ssh")
	if err != nil {
		return "", fmt.Errorf("ssh binary not found: %w", err)
	}

	args := c.buildArgs(c.Host, "--", cmd)
	command := exec.CommandContext(ctx, sshBin, args...)

	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		return "", fmt.Errorf("ssh command failed: %s: %w", strings.TrimSpace(stderr.String()), err)
	}

	return strings.TrimSpace(stdout.String()), nil
}

// RunInteractive runs an interactive SSH session with the given command.
// If OnDetachEscape is set, uses exec.Command (not syscall.Exec) so the
// escape sequence can be emitted locally after SSH disconnects.
func (c *Client) RunInteractive(cmd string) error {
	sshBin, err := exec.LookPath("ssh")
	if err != nil {
		return fmt.Errorf("ssh binary not found: %w", err)
	}

	args := c.buildInteractiveArgs(cmd)

	if c.OnAttach != nil {
		c.OnAttach()
	}

	if c.OnDetachEscape != "" {
		sshCmd := exec.Command(sshBin, args[1:]...)
		sshCmd.Stdin = os.Stdin
		sshCmd.Stdout = os.Stdout
		sshCmd.Stderr = os.Stderr
		_ = sshCmd.Run()
		fmt.Fprint(os.Stdout, c.OnDetachEscape)
		return nil
	}

	return syscall.Exec(sshBin, args, os.Environ())
}

// buildInteractiveArgs constructs arguments for an interactive SSH session.
// Unlike buildArgs, this adds -t for PTY allocation and omits BatchMode.
func (c *Client) buildInteractiveArgs(cmd string) []string {
	args := []string{"ssh"}

	if c.SSHConfig != "" {
		args = append(args, "-F", c.SSHConfig)
	}
	if c.Port != 0 {
		args = append(args, "-p", fmt.Sprintf("%d", c.Port))
	}
	if c.IdentityFile != "" {
		args = append(args, "-i", c.IdentityFile)
	}
	if c.JumpHost != "" {
		args = append(args, "-J", c.JumpHost)
	}

	if c.AgentForwarding {
		args = append(args, "-A")
	}

	args = append(args, "-t", c.Host, "--", cmd)
	return args
}

// Check tests SSH connectivity to the remote host by running a simple command.
func (c *Client) Check(ctx context.Context) error {
	_, err := c.Run(ctx, "echo ok")
	if err != nil {
		return fmt.Errorf("SSH connectivity check failed for %s: %w", c.Host, err)
	}
	return nil
}

// RemoteInfo detects the remote operating system and architecture.
func (c *Client) RemoteInfo(ctx context.Context) (os string, arch string, err error) {
	out, err := c.Run(ctx, "uname -s -m")
	if err != nil {
		return "", "", fmt.Errorf("detecting remote OS/arch: %w", err)
	}

	parts := strings.Fields(out)
	if len(parts) < 2 {
		return "", "", fmt.Errorf("unexpected uname output: %q", out)
	}

	return strings.ToLower(parts[0]), normalizeArch(parts[1]), nil
}

// Upload copies a local file or directory to the remote host using scp.
func (c *Client) Upload(ctx context.Context, localPath, remotePath string) error {
	scpBin, err := exec.LookPath("scp")
	if err != nil {
		return fmt.Errorf("scp binary not found: %w", err)
	}

	var args []string
	args = append(args, "-r") // recursive
	if c.SSHConfig != "" {
		args = append(args, "-F", c.SSHConfig)
	}
	if c.Port != 0 {
		args = append(args, "-P", fmt.Sprintf("%d", c.Port))
	}
	if c.IdentityFile != "" {
		args = append(args, "-i", c.IdentityFile)
	}
	if c.JumpHost != "" {
		args = append(args, "-J", c.JumpHost)
	}
	args = append(args, "-o", "StrictHostKeyChecking=accept-new")
	args = append(args, localPath, c.Host+":"+remotePath)

	cmd := exec.CommandContext(ctx, scpBin, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("scp upload failed: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// Download copies a remote file or directory to the local host using scp.
func (c *Client) Download(ctx context.Context, remotePath, localPath string) error {
	scpBin, err := exec.LookPath("scp")
	if err != nil {
		return fmt.Errorf("scp binary not found: %w", err)
	}

	var args []string
	args = append(args, "-r")
	if c.SSHConfig != "" {
		args = append(args, "-F", c.SSHConfig)
	}
	if c.Port != 0 {
		args = append(args, "-P", fmt.Sprintf("%d", c.Port))
	}
	if c.IdentityFile != "" {
		args = append(args, "-i", c.IdentityFile)
	}
	if c.JumpHost != "" {
		args = append(args, "-J", c.JumpHost)
	}
	args = append(args, "-o", "StrictHostKeyChecking=accept-new")
	args = append(args, c.Host+":"+remotePath, localPath)

	cmd := exec.CommandContext(ctx, scpBin, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("scp download failed: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// Rsync synchronizes files between local and remote using rsync over SSH.
// If push is true, src is local and dst is remote. If push is false, src is
// remote and dst is local. Falls back to scp if rsync is not available.
func (c *Client) Rsync(ctx context.Context, src, dst string, excludes []string, push bool) error {
	rsyncBin, err := exec.LookPath("rsync")
	if err != nil {
		// Fallback to scp when rsync is unavailable locally.
		fmt.Fprintf(os.Stderr, "WARNING: rsync not found on the local machine, falling back to scp (incremental sync unavailable)\n")
		if push {
			return c.Upload(ctx, src, dst)
		}
		return c.Download(ctx, src, dst)
	}

	// Build SSH command for rsync's -e flag. Quote paths to prevent
	// shell injection from filenames with special characters.
	sshParts := []string{"ssh"}
	if c.SSHConfig != "" {
		sshParts = append(sshParts, "-F", fmt.Sprintf("%q", c.SSHConfig))
	}
	if c.Port != 0 {
		sshParts = append(sshParts, "-p", fmt.Sprintf("%d", c.Port))
	}
	if c.IdentityFile != "" {
		sshParts = append(sshParts, "-i", fmt.Sprintf("%q", c.IdentityFile))
	}
	if c.JumpHost != "" {
		sshParts = append(sshParts, "-J", c.JumpHost)
	}
	sshCmd := strings.Join(sshParts, " ")

	args := []string{"-avz", "--progress", "-e", sshCmd}
	for _, ex := range excludes {
		args = append(args, "--exclude", ex)
	}
	args = append(args, src, dst)

	cmd := exec.CommandContext(ctx, rsyncBin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rsync failed: %w", err)
	}
	return nil
}

// normalizeArch maps uname machine output to Go-style architecture names.
func normalizeArch(machine string) string {
	switch machine {
	case "x86_64":
		return "amd64"
	case "aarch64":
		return "arm64"
	default:
		return strings.ToLower(machine)
	}
}
