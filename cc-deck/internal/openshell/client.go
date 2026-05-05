package openshell

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"
)

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

// SSHSession holds information returned by CreateSshSession.
type SSHSession struct {
	SessionID string
	Host      string
	Port      int
	User      string
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
	return GatewayConfig{Address: "localhost:8080"}
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

// Client wraps gRPC communication with the OpenShell gateway.
// Until proto codegen is integrated, this uses the openshell CLI
// as a temporary transport layer.
type Client struct {
	cfg GatewayConfig
}

// NewClient creates a new OpenShell gateway client.
func NewClient(cfg GatewayConfig) (*Client, error) {
	if cfg.Address == "" {
		return nil, fmt.Errorf("gateway address is required")
	}
	if !cfg.TLS && !isLocalhost(cfg.Address) {
		log.Printf("WARNING: connecting to non-localhost gateway %s without TLS", cfg.Address)
	}
	log.Printf("DEBUG: openshell: connecting to gateway at %s (tls=%v)", cfg.Address, cfg.TLS)
	return &Client{cfg: cfg}, nil
}

// Address returns the gateway address.
func (c *Client) Address() string {
	return c.cfg.Address
}

// CreateSandbox provisions a new sandbox on the OpenShell gateway.
func (c *Client) CreateSandbox(ctx context.Context, image, command, policy, provider string) (string, error) {
	start := time.Now()
	args := []string{"sandbox", "create",
		"--gateway", c.cfg.Address,
		"--image", image,
		"--command", command,
	}
	if policy != "" {
		args = append(args, "--policy", policy)
	}
	if provider != "" {
		args = append(args, "--provider", provider)
	}
	out, err := c.execCLI(ctx, args...)
	log.Printf("DEBUG: openshell: CreateSandbox took %v", time.Since(start))
	if err != nil {
		return "", fmt.Errorf("creating sandbox: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// DeleteSandbox destroys an existing sandbox.
func (c *Client) DeleteSandbox(ctx context.Context, sandboxID string) error {
	start := time.Now()
	_, err := c.execCLI(ctx, "sandbox", "delete",
		"--gateway", c.cfg.Address,
		sandboxID)
	log.Printf("DEBUG: openshell: DeleteSandbox(%s) took %v", sandboxID, time.Since(start))
	if err != nil {
		return fmt.Errorf("deleting sandbox %s: %w", sandboxID, err)
	}
	return nil
}

// GetSandbox retrieves the current state of a sandbox.
func (c *Client) GetSandbox(ctx context.Context, sandboxID string) (*SandboxInfo, error) {
	start := time.Now()
	out, err := c.execCLI(ctx, "sandbox", "get",
		"--gateway", c.cfg.Address,
		"--output", "json",
		sandboxID)
	log.Printf("DEBUG: openshell: GetSandbox(%s) took %v", sandboxID, time.Since(start))
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return &SandboxInfo{ID: sandboxID, State: SandboxStateDeleted}, nil
		}
		return nil, fmt.Errorf("getting sandbox %s: %w", sandboxID, err)
	}
	state := parseSandboxState(strings.TrimSpace(out))
	return &SandboxInfo{ID: sandboxID, State: state}, nil
}

// ExecSandbox runs a command inside a sandbox.
func (c *Client) ExecSandbox(ctx context.Context, sandboxID string, cmd []string) (*ExecResult, error) {
	start := time.Now()
	args := []string{"sandbox", "exec",
		"--gateway", c.cfg.Address,
		sandboxID, "--"}
	args = append(args, cmd...)
	out, err := c.execCLI(ctx, args...)
	log.Printf("DEBUG: openshell: ExecSandbox(%s, %v) took %v", sandboxID, cmd, time.Since(start))
	result := &ExecResult{Stdout: out}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			result.Stderr = string(exitErr.Stderr)
			return result, nil
		}
		return nil, fmt.Errorf("exec in sandbox %s: %w", sandboxID, err)
	}
	return result, nil
}

// ExecSandboxStream runs a command inside a sandbox, streaming output
// to the provided writers.
func (c *Client) ExecSandboxStream(ctx context.Context, sandboxID string, cmd []string) error {
	start := time.Now()
	args := []string{"sandbox", "exec",
		"--gateway", c.cfg.Address,
		sandboxID, "--"}
	args = append(args, cmd...)
	osCmd := exec.CommandContext(ctx, "openshell", args...)
	osCmd.Stdout = os.Stdout
	osCmd.Stderr = os.Stderr
	err := osCmd.Run()
	log.Printf("DEBUG: openshell: ExecSandboxStream(%s, %v) took %v", sandboxID, cmd, time.Since(start))
	return err
}

// CreateSshSession establishes an SSH tunnel into a sandbox.
func (c *Client) CreateSshSession(ctx context.Context, sandboxID string) (*SSHSession, error) {
	start := time.Now()
	out, err := c.execCLI(ctx, "sandbox", "ssh",
		"--gateway", c.cfg.Address,
		"--output", "json",
		sandboxID)
	log.Printf("DEBUG: openshell: CreateSshSession(%s) took %v", sandboxID, time.Since(start))
	if err != nil {
		return nil, fmt.Errorf("creating SSH session for sandbox %s: %w", sandboxID, err)
	}
	session := parseSSHSession(strings.TrimSpace(out))
	return session, nil
}

func (c *Client) execCLI(ctx context.Context, args ...string) (string, error) {
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

func parseSandboxState(output string) SandboxState {
	lower := strings.ToLower(output)
	switch {
	case strings.Contains(lower, "running"):
		return SandboxStateRunning
	case strings.Contains(lower, "creating"):
		return SandboxStateCreating
	case strings.Contains(lower, "suspended"):
		return SandboxStateSuspended
	case strings.Contains(lower, "error"):
		return SandboxStateError
	case strings.Contains(lower, "deleted"), strings.Contains(lower, "not found"):
		return SandboxStateDeleted
	default:
		log.Printf("WARNING: unrecognized sandbox state: %q", output)
		return SandboxStateError
	}
}

func parseSSHSession(output string) *SSHSession {
	session := &SSHSession{}
	for _, line := range strings.Split(output, "\n") {
		parts := strings.SplitN(strings.TrimSpace(line), ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		switch strings.ToLower(key) {
		case "session_id", "sessionid", "id":
			session.SessionID = val
		case "host":
			session.Host = val
		case "port":
			fmt.Sscanf(val, "%d", &session.Port)
		case "user":
			session.User = val
		}
	}
	if session.Host == "" {
		session.Host = "localhost"
	}
	if session.Port == 0 {
		session.Port = 22
	}
	if session.User == "" {
		session.User = "sandbox"
	}
	return session
}
