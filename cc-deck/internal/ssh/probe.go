package ssh

import (
	"context"
	"fmt"
	"strings"
)

// Runner abstracts SSH command execution for testability.
type Runner interface {
	Run(ctx context.Context, cmd string) (string, error)
}

// requiredTools lists the tools that must be present on a provisioned host.
var requiredTools = []string{"zellij", "cc-deck", "claude"}

// probePathPrefix is prepended to PATH for non-interactive SSH sessions
// where user-local directories like ~/.local/bin are not on the default PATH.
const probePathPrefix = "PATH=$HOME/.local/bin:$PATH"

// Probe checks whether an SSH host has been provisioned by cc-deck build.
// It verifies that zellij, cc-deck, and claude are all present on the remote host.
func Probe(ctx context.Context, runner Runner) error {
	var missing []string
	for _, tool := range requiredTools {
		_, err := runner.Run(ctx, fmt.Sprintf("%s which %s", probePathPrefix, tool))
		if err != nil {
			missing = append(missing, tool)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("host appears unprovisioned (missing tools: %s)\n"+
			"Run 'cc-deck build' to provision the host first",
			strings.Join(missing, ", "))
	}
	return nil
}
