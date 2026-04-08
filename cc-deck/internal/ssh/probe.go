package ssh

import (
	"context"
	"fmt"
)

// Runner abstracts SSH command execution for testability.
type Runner interface {
	Run(ctx context.Context, cmd string) (string, error)
}

// Probe checks whether an SSH host has been provisioned by cc-deck setup.
// It verifies that zellij, cc-deck, and claude are all present on the remote host.
func Probe(ctx context.Context, runner Runner) error {
	output, err := runner.Run(ctx, "which zellij && which cc-deck && which claude")
	if err != nil {
		return fmt.Errorf("host appears unprovisioned (missing tools): %s\n"+
			"Run 'cc-deck setup' to provision the host first", output)
	}
	return nil
}
