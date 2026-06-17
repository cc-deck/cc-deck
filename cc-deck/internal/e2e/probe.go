//go:build e2e

package e2e

import (
	"fmt"
	"strings"
)

// ProbeCheck defines a single validation to run inside a built container.
type ProbeCheck struct {
	Name    string
	Command []string
	Check   func(exitCode int, stdout string) error
}

// ContainerProbeChecks returns the standard probes to run against a cc-deck image.
func ContainerProbeChecks(expectedUser, expectedHome, expectedShell string) []ProbeCheck {
	return []ProbeCheck{
		{
			Name:    "claude-code-binary",
			Command: []string{"claude", "--version"},
			Check:   expectExitZero,
		},
		{
			Name:    "zellij-binary",
			Command: []string{"zellij", "--version"},
			Check:   expectExitZero,
		},
		{
			Name:    "cc-deck-binary",
			Command: []string{"cc-deck", "--version"},
			Check:   expectExitZero,
		},
		{
			Name:    "user-identity",
			Command: []string{"whoami"},
			Check:   expectOutput(expectedUser),
		},
		{
			Name:    "home-directory",
			Command: []string{"sh", "-c", "echo $HOME"},
			Check:   expectOutput(expectedHome),
		},
		{
			Name:    "shell-config",
			Command: []string{"sh", "-c", "echo $SHELL"},
			Check:   expectContains(expectedShell),
		},
		{
			Name:    "cc-session-binary",
			Command: []string{"which", "cc-session"},
			Check:   expectExitZero,
		},
		{
			Name:    "cc-setup-binary",
			Command: []string{"which", "cc-setup"},
			Check:   expectExitZero,
		},
		{
			Name:    "write-permissions",
			Command: []string{"sh", "-c", "touch $HOME/test-write && rm $HOME/test-write"},
			Check:   expectExitZero,
		},
		{
			Name:    "plugin-installed",
			Command: []string{"sh", "-c", "ls " + expectedHome + "/.config/zellij/plugins/cc_deck.wasm"},
			Check:   expectExitZero,
		},
	}
}

func expectExitZero(exitCode int, stdout string) error {
	if exitCode != 0 {
		return fmt.Errorf("expected exit 0, got %d (output: %s)", exitCode, strings.TrimSpace(stdout))
	}
	return nil
}

func expectOutput(expected string) func(int, string) error {
	return func(exitCode int, stdout string) error {
		if exitCode != 0 {
			return fmt.Errorf("expected exit 0, got %d", exitCode)
		}
		got := strings.TrimSpace(stdout)
		if got != expected {
			return fmt.Errorf("expected %q, got %q", expected, got)
		}
		return nil
	}
}

func expectContains(substr string) func(int, string) error {
	return func(exitCode int, stdout string) error {
		if exitCode != 0 {
			return fmt.Errorf("expected exit 0, got %d", exitCode)
		}
		if !strings.Contains(stdout, substr) {
			return fmt.Errorf("expected output to contain %q, got %q", substr, strings.TrimSpace(stdout))
		}
		return nil
	}
}
