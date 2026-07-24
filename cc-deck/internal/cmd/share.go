package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/cc-deck/cc-deck/internal/config"
	sharing "github.com/cc-deck/cc-deck/internal/share"
	"github.com/spf13/cobra"
)

type shareStarter interface {
	Start(context.Context, sharing.StartRequest) (sharing.InvitationSet, error)
}

func NewShareCmd(gf *GlobalFlags) *cobra.Command {
	runner := osCommandRunner{}
	zellij := sharing.NewZellij(runner)
	provider := sharing.NewCloudflareProvider(runner)
	service := sharing.NewService(sharing.NewFileStore(""), zellij, provider)
	return newShareCmd(gf, service)
}

func newShareCmd(gf *GlobalFlags, service shareStarter) *cobra.Command {
	parent := &cobra.Command{
		Use:   "share",
		Short: "Share one complete Zellij session",
	}
	var provider string
	start := &cobra.Command{
		Use:   "start [session]",
		Short: "Start ephemeral sharing",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			session := ""
			if len(args) == 1 {
				session = args[0]
			}
			if provider == "" {
				cfg, err := config.Load(gf.ConfigFile)
				if err != nil {
					return err
				}
				provider = cfg.SharingProvider()
			}
			invitations, err := service.Start(cmd.Context(), sharing.StartRequest{Session: session, Provider: provider})
			if err != nil {
				return fmt.Errorf("start session sharing: %w", err)
			}
			out := cmd.OutOrStdout()
			for _, warning := range invitations.Warnings {
				fmt.Fprintln(out, warning)
			}
			fmt.Fprintf(out, "\nInteractive browser invitation:\n%s\n", invitations.InteractiveBrowser)
			fmt.Fprintf(out, "\nInteractive terminal invitation (experimental):\n%s\n", invitations.InteractiveTerminal)
			fmt.Fprintf(out, "\nObserver browser invitation (read-only):\n%s\n", invitations.ObserverBrowser)
			fmt.Fprintf(out, "\nObserver terminal invitation (read-only, experimental):\n%s\n", invitations.ObserverTerminal)
			return nil
		},
	}
	start.Flags().StringVar(&provider, "provider", "", "Exposure provider (default from sharing configuration)")
	parent.AddCommand(start)
	return parent
}

type osCommandRunner struct{}

func (osCommandRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

func (osCommandRunner) Start(ctx context.Context, name string, args ...string) (sharing.Process, error) {
	command := exec.CommandContext(ctx, name, args...)
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	if err := command.Start(); err != nil {
		return nil, err
	}
	return &commandProcess{command: command}, nil
}

type commandProcess struct{ command *exec.Cmd }

func (p *commandProcess) PID() int                      { return p.command.Process.Pid }
func (p *commandProcess) Wait() error                   { return p.command.Wait() }
func (p *commandProcess) Signal(signal os.Signal) error { return p.command.Process.Signal(signal) }
func (p *commandProcess) Kill() error                   { return p.command.Process.Kill() }
