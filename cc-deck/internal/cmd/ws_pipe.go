package cmd

import (
	"fmt"

	"github.com/cc-deck/cc-deck/internal/ws"
	"github.com/spf13/cobra"
)

func newWsPipeCmd(_ *GlobalFlags) *cobra.Command {
	var pipeName string
	var payload string

	cmd := &cobra.Command{
		Use:   "pipe <workspace>",
		Short: "Send a payload to a named pipe in a workspace",
		Long: `Send arbitrary text to a named Zellij pipe in any workspace type.
The text is delivered via PipeChannel to the plugin handling the named pipe.`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if pipeName == "" {
				return fmt.Errorf("--name is required")
			}
			return runWsPipe(args[0], pipeName, payload)
		},
	}

	cmd.Flags().StringVar(&pipeName, "name", "", "pipe name (e.g., cc-deck:voice)")
	cmd.Flags().StringVar(&payload, "payload", "", "text payload to send")
	_ = cmd.MarkFlagRequired("name")

	return cmd
}

func runWsPipe(name, pipeName, payload string) error {
	store := ws.NewStateStore("")
	defs := ws.NewDefinitionStore("")

	e, err := resolveWorkspace(name, store, defs)
	if err != nil {
		return err
	}

	ch, err := e.PipeChannel(cmd_context())
	if err != nil {
		return fmt.Errorf("getting pipe channel: %w", err)
	}

	if err := ch.Send(cmd_context(), pipeName, payload); err != nil {
		return fmt.Errorf("sending to pipe %q: %w", pipeName, err)
	}

	fmt.Printf("Sent to %s pipe %q\n", name, pipeName)
	return nil
}
