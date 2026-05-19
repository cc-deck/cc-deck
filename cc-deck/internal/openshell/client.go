package openshell

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"
)

//go:embed default-policy.yaml
var defaultPolicy []byte

// SandboxState represents the lifecycle state of an OpenShell sandbox.
type SandboxState string

const (
	SandboxStateCreating  SandboxState = "creating"
	SandboxStateRunning   SandboxState = "running"
	SandboxStateSuspended SandboxState = "suspended"
	SandboxStateError     SandboxState = "error"
	SandboxStateDeleted   SandboxState = "deleted"
)

// SandboxInfo holds information returned by GetSandbox.
type SandboxInfo struct {
	ID    string
	State SandboxState
}

// ExecResult holds the result of a command execution inside a sandbox.
type ExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// GatewayConfig holds connection parameters for the OpenShell gateway.
type GatewayConfig struct {
	Address     string
	TLS         bool
	TLSCertPath string
	TLSKeyPath  string
	TLSCAPath   string
}

// ResolveGatewayConfig determines the gateway configuration by checking
// the workspace definition first, then the environment variable, then
// the default.
func ResolveGatewayConfig(gateway *GatewayConfig) GatewayConfig {
	if gateway != nil && gateway.Address != "" {
		return *gateway
	}
	if envAddr, ok := os.LookupEnv("OPENSHELL_GATEWAY_URL"); ok && envAddr != "" {
		return GatewayConfig{Address: envAddr}
	}
	return GatewayConfig{Address: "localhost:17670"}
}

// isLocalhost returns true if the address targets a loopback interface.
func isLocalhost(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	host = strings.TrimPrefix(host, "[")
	host = strings.TrimSuffix(host, "]")
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

// cliClient wraps the openshell CLI binary for gateway communication.
type cliClient struct {
	cfg GatewayConfig
}

// NewClient creates a new OpenShell gateway client that wraps the CLI.
func NewClient(cfg GatewayConfig) (Client, error) {
	if cfg.Address == "" {
		return nil, fmt.Errorf("gateway address is required")
	}
	if !cfg.TLS && !isLocalhost(cfg.Address) {
		log.Printf("WARNING: connecting to non-localhost gateway %s without TLS", cfg.Address)
	}
	log.Printf("DEBUG: openshell: gateway configured at %s (tls=%v)", cfg.Address, cfg.TLS)
	return &cliClient{cfg: cfg}, nil
}

func (c *cliClient) Address() string {
	return c.cfg.Address
}

// CreateSandbox provisions a new sandbox on the OpenShell gateway.
func (c *cliClient) CreateSandbox(ctx context.Context, image, command, policy, provider string) (string, error) {
	start := time.Now()
	policyPath := policy
	if policyPath == "" {
		tmp, writeErr := os.CreateTemp("", "openshell-policy-*.yaml")
		if writeErr != nil {
			return "", fmt.Errorf("writing default policy: %w", writeErr)
		}
		defer os.Remove(tmp.Name())
		if _, writeErr = tmp.Write(defaultPolicy); writeErr != nil {
			tmp.Close()
			return "", fmt.Errorf("writing default policy: %w", writeErr)
		}
		tmp.Close()
		policyPath = tmp.Name()
	}
	args := []string{"sandbox", "create", "--from", image, "--policy", policyPath}
	if provider != "" {
		args = append(args, "--provider", provider)
	}
	args = append(args, "--")
	args = append(args, strings.Fields(command)...)

	// The CLI blocks with a progress spinner and never exits in non-TTY mode.
	// We read the first few lines to capture the sandbox name, then kill it.
	// The workspace layer polls for Ready via GetSandbox separately.
	sandboxName, err := c.execCLICaptureName(ctx, args...)
	log.Printf("DEBUG: openshell: CreateSandbox took %v", time.Since(start))
	if err != nil {
		return "", fmt.Errorf("creating sandbox: %w", err)
	}
	if sandboxName == "" {
		return "", fmt.Errorf("creating sandbox: could not parse sandbox name from CLI output")
	}

	return sandboxName, nil
}

// execCLICaptureName starts the openshell CLI, reads output until the sandbox
// name is found, then terminates the process. This is needed because the CLI
// blocks with a progress spinner in non-TTY mode.
func (c *cliClient) execCLICaptureName(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "openshell", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return "", err
	}

	buf := make([]byte, 4096)
	var output strings.Builder
	nameFound := false

	for !nameFound {
		n, readErr := stdout.Read(buf)
		if n > 0 {
			output.Write(buf[:n])
			if name := parseSandboxName(stripANSI(output.String())); name != "" {
				nameFound = true
			}
		}
		if readErr != nil {
			break
		}
	}

	_ = cmd.Process.Kill()
	_ = cmd.Wait()

	return parseSandboxName(stripANSI(output.String())), nil
}

// DeleteSandbox destroys an existing sandbox.
func (c *cliClient) DeleteSandbox(ctx context.Context, sandboxName string) error {
	start := time.Now()
	_, err := c.execCLI(ctx, "sandbox", "delete", sandboxName)
	log.Printf("DEBUG: openshell: DeleteSandbox(%s) took %v", sandboxName, time.Since(start))
	if err != nil {
		return fmt.Errorf("deleting sandbox %s: %w", sandboxName, err)
	}
	return nil
}

// GetSandbox retrieves the current state of a sandbox.
func (c *cliClient) GetSandbox(ctx context.Context, sandboxName string) (*SandboxInfo, error) {
	start := time.Now()
	out, err := c.execCLI(ctx, "sandbox", "get", sandboxName)
	log.Printf("DEBUG: openshell: GetSandbox(%s) took %v", sandboxName, time.Since(start))
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return &SandboxInfo{ID: sandboxName, State: SandboxStateDeleted}, nil
		}
		return nil, fmt.Errorf("getting sandbox %s: %w", sandboxName, err)
	}
	state := parseSandboxState(strings.TrimSpace(out))
	return &SandboxInfo{ID: sandboxName, State: state}, nil
}

// ExecSandbox runs a command inside a sandbox and captures the output.
func (c *cliClient) ExecSandbox(ctx context.Context, sandboxName string, cmd []string) (*ExecResult, error) {
	start := time.Now()
	args := []string{"sandbox", "exec", "-n", sandboxName, "--"}
	args = append(args, cmd...)
	out, err := c.execCLI(ctx, args...)
	log.Printf("DEBUG: openshell: ExecSandbox(%s, %v) took %v", sandboxName, cmd, time.Since(start))
	result := &ExecResult{Stdout: out}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			result.Stderr = string(exitErr.Stderr)
			return result, nil
		}
		return nil, fmt.Errorf("exec in sandbox %s: %w", sandboxName, err)
	}
	return result, nil
}

// ExecSandboxStream runs a command inside a sandbox, streaming output
// to stdout/stderr.
func (c *cliClient) ExecSandboxStream(ctx context.Context, sandboxName string, cmd []string) error {
	start := time.Now()
	args := []string{"sandbox", "exec", "-n", sandboxName, "--"}
	args = append(args, cmd...)
	osCmd := exec.CommandContext(ctx, "openshell", args...)
	osCmd.Stdout = os.Stdout
	osCmd.Stderr = os.Stderr
	err := osCmd.Run()
	log.Printf("DEBUG: openshell: ExecSandboxStream(%s, %v) took %v", sandboxName, cmd, time.Since(start))
	return err
}

// AttachExec connects interactively to a sandbox via `openshell sandbox connect`.
// The cmd parameter is currently unused because connect runs the sandbox's
// default shell. For Zellij-based workspaces, the sandbox image should have
// Zellij as the entrypoint or default command.
func (c *cliClient) AttachExec(ctx context.Context, sandboxName string, _ []string) error {
	start := time.Now()
	args := []string{"sandbox", "connect", sandboxName}
	osCmd := exec.CommandContext(ctx, "openshell", args...)
	osCmd.Stdin = os.Stdin
	osCmd.Stdout = os.Stdout
	osCmd.Stderr = os.Stderr
	err := osCmd.Run()
	log.Printf("DEBUG: openshell: AttachExec(%s) took %v", sandboxName, time.Since(start))
	return err
}

// Upload transfers files from the local filesystem into a sandbox.
func (c *cliClient) Upload(ctx context.Context, sandboxName, localPath, remotePath string) error {
	start := time.Now()
	_, err := c.execCLI(ctx, "sandbox", "upload", sandboxName, localPath, remotePath)
	log.Printf("DEBUG: openshell: Upload(%s, %s -> %s) took %v", sandboxName, localPath, remotePath, time.Since(start))
	if err != nil {
		return fmt.Errorf("uploading to sandbox %s: %w", sandboxName, err)
	}
	return nil
}

// Download transfers files from a sandbox to the local filesystem.
func (c *cliClient) Download(ctx context.Context, sandboxName, remotePath, localPath string) error {
	start := time.Now()
	_, err := c.execCLI(ctx, "sandbox", "download", sandboxName, remotePath, localPath)
	log.Printf("DEBUG: openshell: Download(%s, %s -> %s) took %v", sandboxName, remotePath, localPath, time.Since(start))
	if err != nil {
		return fmt.Errorf("downloading from sandbox %s: %w", sandboxName, err)
	}
	return nil
}

func (c *cliClient) execCLI(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "openshell", args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("%s: %s", err, string(exitErr.Stderr))
		}
		return "", err
	}
	return string(out), nil
}

// parseSandboxName extracts the sandbox name from the create command output.
// The CLI outputs "Created sandbox: <name>" followed by progress lines.
func parseSandboxName(output string) string {
	stripped := stripANSI(output)
	for _, line := range strings.Split(stripped, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Created sandbox:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "Created sandbox:"))
		}
	}
	first := strings.TrimSpace(strings.Split(stripped, "\n")[0])
	if first != "" && !strings.ContainsAny(first, " \t[") {
		return first
	}
	return ""
}

// parseSandboxPhase extracts the Phase value from sandbox get output.
// The CLI outputs "Phase: <phase>" with optional ANSI color codes.
func parseSandboxPhase(output string) string {
	stripped := stripANSI(output)
	for _, line := range strings.Split(stripped, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Phase:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "Phase:"))
		}
	}
	return output
}

func parseSandboxState(output string) SandboxState {
	phase := strings.ToLower(parseSandboxPhase(output))
	switch {
	case strings.Contains(phase, "ready"), strings.Contains(phase, "running"):
		return SandboxStateRunning
	case strings.Contains(phase, "provisioning"), strings.Contains(phase, "creating"):
		return SandboxStateCreating
	case strings.Contains(phase, "suspended"):
		return SandboxStateSuspended
	case strings.Contains(phase, "error"):
		return SandboxStateError
	case strings.Contains(phase, "delet"), strings.Contains(phase, "not found"):
		return SandboxStateDeleted
	default:
		log.Printf("WARNING: unrecognized sandbox phase: %q", phase)
		return SandboxStateError
	}
}

// stripANSI removes ANSI escape sequences from a string.
func stripANSI(s string) string {
	var result strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && !((s[j] >= 'A' && s[j] <= 'Z') || (s[j] >= 'a' && s[j] <= 'z')) {
				j++
			}
			if j < len(s) {
				j++
			}
			i = j
			continue
		}
		result.WriteByte(s[i])
		i++
	}
	return result.String()
}
