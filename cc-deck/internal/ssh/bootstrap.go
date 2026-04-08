package ssh

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
)

// PreflightCheck defines a single pre-flight verification step.
type PreflightCheck interface {
	Name() string
	Run(ctx context.Context) error
	HasRemedy() bool
	Remedy(ctx context.Context) error
	ManualInstructions() string
}

// ConnectivityCheck verifies SSH connectivity to the remote host.
type ConnectivityCheck struct {
	client *Client
}

func (c *ConnectivityCheck) Name() string { return "SSH connectivity" }
func (c *ConnectivityCheck) Run(ctx context.Context) error {
	return c.client.Check(ctx)
}
func (c *ConnectivityCheck) HasRemedy() bool             { return false }
func (c *ConnectivityCheck) Remedy(_ context.Context) error { return nil }
func (c *ConnectivityCheck) ManualInstructions() string {
	return "Verify SSH access: ssh " + c.client.Host
}

// OSDetectionCheck detects the remote operating system and architecture.
type OSDetectionCheck struct {
	client *Client
	OS     string
	Arch   string
}

func (c *OSDetectionCheck) Name() string { return "OS/architecture detection" }
func (c *OSDetectionCheck) Run(ctx context.Context) error {
	os, arch, err := c.client.RemoteInfo(ctx)
	if err != nil {
		return err
	}
	c.OS = os
	c.Arch = arch
	return nil
}
func (c *OSDetectionCheck) HasRemedy() bool             { return false }
func (c *OSDetectionCheck) Remedy(_ context.Context) error { return nil }
func (c *OSDetectionCheck) ManualInstructions() string {
	return "Run 'uname -s -m' on the remote host"
}

// ZellijCheck verifies that Zellij is installed on the remote host.
type ZellijCheck struct {
	client *Client
	os     string
	arch   string
}

func (c *ZellijCheck) Name() string { return "Zellij" }
func (c *ZellijCheck) Run(ctx context.Context) error {
	_, err := c.client.Run(ctx, "which zellij")
	return err
}
func (c *ZellijCheck) HasRemedy() bool { return c.os == "linux" }
func (c *ZellijCheck) Remedy(ctx context.Context) error {
	installCmd := fmt.Sprintf(
		"curl -sSL https://github.com/zellij-org/zellij/releases/latest/download/zellij-%s-%s.tar.gz | tar xz -C /usr/local/bin 2>/dev/null || "+
			"curl -sSL https://github.com/zellij-org/zellij/releases/latest/download/zellij-%s-%s.tar.gz | sudo tar xz -C /usr/local/bin",
		c.arch, c.os, c.arch, c.os)
	_, err := c.client.Run(ctx, installCmd)
	return err
}
func (c *ZellijCheck) ManualInstructions() string {
	return "Install Zellij: https://zellij.dev/documentation/installation"
}

// ClaudeCodeCheck verifies that Claude Code is installed on the remote host.
type ClaudeCodeCheck struct {
	client *Client
}

func (c *ClaudeCodeCheck) Name() string { return "Claude Code" }
func (c *ClaudeCodeCheck) Run(ctx context.Context) error {
	_, err := c.client.Run(ctx, "which claude")
	return err
}
func (c *ClaudeCodeCheck) HasRemedy() bool { return true }
func (c *ClaudeCodeCheck) Remedy(ctx context.Context) error {
	_, err := c.client.Run(ctx, "npm install -g @anthropic-ai/claude-code 2>/dev/null || npx @anthropic-ai/claude-code --version")
	return err
}
func (c *ClaudeCodeCheck) ManualInstructions() string {
	return "Install Claude Code: npm install -g @anthropic-ai/claude-code"
}

// CcDeckCheck verifies that the cc-deck CLI is installed on the remote host.
type CcDeckCheck struct {
	client *Client
}

func (c *CcDeckCheck) Name() string { return "cc-deck CLI" }
func (c *CcDeckCheck) Run(ctx context.Context) error {
	_, err := c.client.Run(ctx, "which cc-deck")
	return err
}
func (c *CcDeckCheck) HasRemedy() bool { return true }
func (c *CcDeckCheck) Remedy(ctx context.Context) error {
	_, err := c.client.Run(ctx, "curl -sSL https://raw.githubusercontent.com/cc-deck/cc-deck/main/install.sh | bash")
	return err
}
func (c *CcDeckCheck) ManualInstructions() string {
	return "Install cc-deck: see https://github.com/cc-deck/cc-deck#installation"
}

// PluginCheck verifies that the cc-deck Zellij plugin is installed on the remote host.
type PluginCheck struct {
	client *Client
}

func (c *PluginCheck) Name() string { return "cc-deck plugin" }
func (c *PluginCheck) Run(ctx context.Context) error {
	// Check if the plugin WASM file exists in common locations.
	_, err := c.client.Run(ctx, "test -f ~/.config/zellij/plugins/cc_deck.wasm || test -f /usr/share/zellij/plugins/cc_deck.wasm")
	return err
}
func (c *PluginCheck) HasRemedy() bool { return true }
func (c *PluginCheck) Remedy(ctx context.Context) error {
	_, err := c.client.Run(ctx, "cc-deck plugin install 2>/dev/null || true")
	return err
}
func (c *PluginCheck) ManualInstructions() string {
	return "Install plugin: cc-deck plugin install"
}

// CredentialCheck verifies that the credential mode can be satisfied.
type CredentialCheck struct {
	client   *Client
	authMode string
}

func (c *CredentialCheck) Name() string { return "Credential verification" }
func (c *CredentialCheck) Run(_ context.Context) error {
	if c.authMode == "none" {
		return nil
	}
	// Credential verification is handled at attach time; this check
	// just confirms the auth mode is valid.
	switch c.authMode {
	case "", "auto", "api", "vertex", "bedrock":
		return nil
	default:
		return fmt.Errorf("unknown auth mode: %s", c.authMode)
	}
}
func (c *CredentialCheck) HasRemedy() bool             { return false }
func (c *CredentialCheck) Remedy(_ context.Context) error { return nil }
func (c *CredentialCheck) ManualInstructions() string {
	return "Set auth mode to one of: auto, api, vertex, bedrock, none"
}

// RunPreflightChecks executes all pre-flight checks in sequence with
// interactive prompts for remediation.
func RunPreflightChecks(ctx context.Context, client *Client, authMode string, stdin io.Reader, stdout io.Writer) error {
	osCheck := &OSDetectionCheck{client: client}

	checks := []PreflightCheck{
		&ConnectivityCheck{client: client},
		osCheck,
		&ZellijCheck{client: client, os: "", arch: ""},
		&ClaudeCodeCheck{client: client},
		&CcDeckCheck{client: client},
		&PluginCheck{client: client},
		&CredentialCheck{client: client, authMode: authMode},
	}

	scanner := bufio.NewScanner(stdin)

	for i, check := range checks {
		fmt.Fprintf(stdout, "  [%d/%d] %s... ", i+1, len(checks), check.Name())

		if err := check.Run(ctx); err != nil {
			fmt.Fprintf(stdout, "MISSING\n")

			// Update OS/arch for ZellijCheck from the earlier OSDetectionCheck.
			if zc, ok := check.(*ZellijCheck); ok {
				zc.os = osCheck.OS
				zc.arch = osCheck.Arch
			}

			if check.HasRemedy() {
				fmt.Fprintf(stdout, "    Install automatically? [y/n/m(anual)] ")
				if scanner.Scan() {
					answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
					switch answer {
					case "y", "yes":
						fmt.Fprintf(stdout, "    Installing... ")
						if remedyErr := check.Remedy(ctx); remedyErr != nil {
							fmt.Fprintf(stdout, "FAILED: %v\n", remedyErr)
							fmt.Fprintf(stdout, "    Manual: %s\n", check.ManualInstructions())
						} else {
							fmt.Fprintf(stdout, "OK\n")
						}
					case "m", "manual":
						fmt.Fprintf(stdout, "    %s\n", check.ManualInstructions())
					default:
						fmt.Fprintf(stdout, "    Skipped.\n")
					}
				}
			} else {
				fmt.Fprintf(stdout, "    %s\n", check.ManualInstructions())
				return fmt.Errorf("pre-flight check %q failed: %w", check.Name(), err)
			}
		} else {
			fmt.Fprintf(stdout, "OK\n")
		}
	}

	return nil
}
