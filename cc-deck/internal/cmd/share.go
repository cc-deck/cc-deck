package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/cc-deck/cc-deck/internal/config"
	sharing "github.com/cc-deck/cc-deck/internal/share"
	"github.com/spf13/cobra"
)

type shareLifecycle interface {
	Start(context.Context, sharing.StartRequest) (sharing.InvitationSet, error)
	Status(context.Context) (sharing.SharingStatus, error)
	Stop(context.Context) (sharing.SharingStatus, error)
}

func NewShareCmd(gf *GlobalFlags) *cobra.Command {
	runner := osCommandRunner{}
	zellij := sharing.NewZellij(runner)
	store := sharing.NewFileStore("")
	registry, err := sharing.NewProviderRegistry(sharing.NewCloudflareProvider(runner))
	if err != nil {
		panic(err)
	}
	executable, err := os.Executable()
	if err != nil {
		executable = "cc-deck"
	}
	factory := func(_ context.Context, providerName string, guarded bool) (shareLifecycle, error) {
		if providerName == "" {
			providerName = "cloudflare"
			if loadErr := store.WithLock(context.Background(), func() error {
				op, err := store.Load()
				if err != nil {
					return err
				}
				if op != nil {
					providerName = op.Provider
				}
				return nil
			}); loadErr != nil {
				return nil, loadErr
			}
		}
		provider, getErr := registry.Get(providerName)
		if getErr != nil {
			return nil, getErr
		}
		if guarded {
			return sharing.NewServiceWithGuard(store, zellij, provider, sharing.NewDetachedGuard(runner, executable)), nil
		}
		return sharing.NewService(store, zellij, provider), nil
	}
	parent := newShareCmdWithFactory(gf, factory, registry.Names())
	addGuardCommand(parent, store, registry, factory)
	return parent
}

func newShareCmd(gf *GlobalFlags, service shareLifecycle) *cobra.Command {
	return newShareCmdWithFactory(gf, func(context.Context, string, bool) (shareLifecycle, error) { return service, nil }, []string{"cloudflare"})
}

type shareServiceFactory func(context.Context, string, bool) (shareLifecycle, error)

func newShareCmdWithFactory(gf *GlobalFlags, factory shareServiceFactory, providers []string) *cobra.Command {
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
			service, err := factory(cmd.Context(), provider, true)
			if err != nil {
				return err
			}
			invitations, err := service.Start(cmd.Context(), sharing.StartRequest{Session: session, Provider: provider})
			if err != nil {
				return fmt.Errorf("start session sharing: %w", err)
			}
			out := cmd.OutOrStdout()
			for _, warning := range invitations.Warnings {
				fmt.Fprintln(out, warning)
			}
			if invitations.InteractiveBrowser == "" && invitations.InteractiveTerminal == "" &&
				invitations.ObserverBrowser == "" && invitations.ObserverTerminal == "" {
				return nil
			}
			fmt.Fprintf(out, "\nInteractive browser invitation:\n%s\n", invitations.InteractiveBrowser)
			fmt.Fprintf(out, "\nInteractive terminal invitation (experimental):\n%s\n", invitations.InteractiveTerminal)
			fmt.Fprintf(out, "\nObserver browser invitation (read-only):\n%s\n", invitations.ObserverBrowser)
			fmt.Fprintf(out, "\nObserver terminal invitation (read-only, experimental):\n%s\n", invitations.ObserverTerminal)
			return nil
		},
	}
	start.Flags().StringVar(&provider, "provider", "", "Exposure provider (default from sharing configuration)")
	_ = start.RegisterFlagCompletionFunc("provider", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return append([]string(nil), providers...), cobra.ShellCompDirectiveNoFileComp
	})

	statusCmd := &cobra.Command{
		Use: "status", Short: "Show session sharing status", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, err := factory(cmd.Context(), "", true)
			if err != nil {
				return err
			}
			status, statusErr := service.Status(cmd.Context())
			printSharingStatus(cmd, status)
			if statusErr == nil && status.State == sharing.StateDegraded {
				statusErr = fmt.Errorf("sharing remains degraded")
			}
			return statusErr
		},
	}
	stopCmd := &cobra.Command{
		Use: "stop", Short: "Stop session sharing", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, err := factory(cmd.Context(), "", true)
			if err != nil {
				return err
			}
			status, stopErr := service.Stop(cmd.Context())
			printSharingStatus(cmd, status)
			if stopErr == nil && status.State == sharing.StateDegraded {
				stopErr = fmt.Errorf("sharing cleanup remains degraded")
			}
			return stopErr
		},
	}
	parent.AddCommand(start, statusCmd, stopCmd)
	return parent
}

func defaultProvider(gf *GlobalFlags) string {
	cfg, err := config.Load(gf.ConfigFile)
	if err == nil && cfg.SharingProvider() != "" {
		return cfg.SharingProvider()
	}
	return "cloudflare"
}

func printSharingStatus(cmd *cobra.Command, status sharing.SharingStatus) {
	out := cmd.OutOrStdout()
	if status.State == sharing.StateInactive || status.State == "" {
		fmt.Fprintln(out, "Sharing is inactive.")
		return
	}
	fmt.Fprintf(out, "State: %s\nSession: %s\nProvider: %s\nEndpoint: %s\n", status.State, status.Session, status.Provider, status.EndpointURL)
	fmt.Fprintf(out, "Interactive role available: %t\nObserver role available: %t\n", status.InteractiveAvailable, status.ObserverAvailable)
	for _, residual := range status.Residuals {
		fmt.Fprintf(out, "Residual exposure: %s\n", residual)
	}
}

func addGuardCommand(parent *cobra.Command, store sharing.Store, registry *sharing.ProviderRegistry, factory shareServiceFactory) {
	var operationID, readyFile string
	guardCmd := &cobra.Command{
		Use: "guard", Hidden: true, Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var providerName string
			if err := store.WithLock(cmd.Context(), func() error {
				op, loadErr := store.Load()
				if loadErr != nil {
					return loadErr
				}
				if op == nil || op.ID != operationID {
					return fmt.Errorf("sharing operation %q is no longer current", operationID)
				}
				providerName = op.Provider
				return nil
			}); err != nil {
				return err
			}
			provider, err := registry.Get(providerName)
			if err != nil {
				return err
			}
			service, err := factory(cmd.Context(), providerName, false)
			if err != nil {
				return err
			}
			guardCtx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer cancel()
			return sharing.RunGuard(guardCtx, store, provider, service, operationID, readyFile)
		},
	}
	guardCmd.Flags().StringVar(&operationID, "operation", "", "sharing operation identity")
	guardCmd.Flags().StringVar(&readyFile, "ready-file", "", "guard readiness path")
	_ = guardCmd.MarkFlagRequired("operation")
	_ = guardCmd.MarkFlagRequired("ready-file")
	parent.AddCommand(guardCmd)
}

type osCommandRunner struct{}

func (osCommandRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

func (osCommandRunner) Start(ctx context.Context, name string, args ...string) (sharing.Process, error) {
	command := exec.CommandContext(ctx, name, args...)
	if len(args) >= 2 && args[0] == "share" && args[1] == "guard" {
		command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		command.Stdin, command.Stdout, command.Stderr = nil, nil, nil
	} else {
		command.Stdout, command.Stderr = os.Stdout, os.Stderr
	}
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
