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
	"github.com/cc-deck/cc-deck/internal/ws"
	"github.com/spf13/cobra"
)

func workspaceShareService(gf *GlobalFlags, guarded bool) (sharing.Service, error) {
	runner := osCommandRunner{}
	store := sharing.NewFileStore("")
	providerName := defaultProvider(gf)
	provider := sharing.NewCloudflareProvider(runner)
	if providerName != provider.Name() {
		return nil, fmt.Errorf("provider %q is not available", providerName)
	}
	zellij := sharing.NewZellij(runner)
	if !guarded {
		return sharing.NewService(store, zellij, provider), nil
	}
	executable, err := os.Executable()
	if err != nil {
		executable = "cc-deck"
	}
	return sharing.NewServiceWithGuard(store, zellij, provider, sharing.NewDetachedGuard(runner, executable)), nil
}

func defaultProvider(gf *GlobalFlags) string {
	if gf == nil {
		return "cloudflare"
	}
	cfg, err := config.Load(gf.ConfigFile)
	if err == nil && cfg.SharingProvider() != "" {
		return cfg.SharingProvider()
	}
	return "cloudflare"
}

type osCommandRunner struct{}

func (osCommandRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}
func (osCommandRunner) Start(ctx context.Context, name string, args ...string) (sharing.Process, error) {
	command := exec.CommandContext(ctx, name, args...)
	if len(args) >= 2 && args[0] == "ws" && args[1] == "share-guard" {
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

func ensureWorkspaceReady(ctx context.Context, workspace ws.Workspace, share bool, current sharing.SharingStatus, run func(context.Context, ws.Workspace, ws.ReadyOptions) (ws.ReadyResult, error)) (ws.ReadyResult, error) {
	status, err := workspace.Status(ctx)
	if err != nil {
		return ws.ReadyResult{}, err
	}
	if share && status.SessionState == ws.SessionStateExists {
		if current.State == sharing.StateActive && current.Workspace == workspace.Name() {
			return ws.ReadyResult{SessionName: ws.ZellijSessionName(workspace.Name())}, nil
		}
		return ws.ReadyResult{}, fmt.Errorf("workspace %q already has a private session; restart it for sharing:\n  cc-deck ws kill-session %s\n  cc-deck ws start %s --share", workspace.Name(), workspace.Name(), workspace.Name())
	}
	return run(ctx, workspace, ws.ReadyOptions{Share: share})
}

func readyAndMaybeShare(ctx context.Context, gf *GlobalFlags, workspace ws.Workspace, share bool) ([]sharing.Invitation, ws.ReadyResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if !share {
		ready, err := ws.EnsureReady(ctx, workspace, ws.ReadyOptions{})
		return nil, ready, err
	}
	service, err := workspaceShareService(gf, true)
	if err != nil {
		return nil, ws.ReadyResult{}, err
	}
	current, _ := service.Status(ctx)
	ready, err := ensureWorkspaceReady(ctx, workspace, share, current, ws.EnsureReady)
	if err != nil {
		return nil, ready, err
	}
	invitations, err := service.Start(ctx, sharing.StartRequest{Workspace: workspace.Name(), Session: ws.ZellijSessionName(workspace.Name()), Provider: defaultProvider(gf)})
	if err != nil && ready.SessionCreated {
		_ = workspace.KillSession(context.Background())
	}
	return invitations, ready, err
}

func printInvitations(cmd *cobra.Command, invitations []sharing.Invitation) {
	for _, invitation := range invitations {
		for _, warning := range invitation.Warnings {
			fmt.Fprintln(cmd.OutOrStdout(), warning)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s invitation %q:\nBrowser:\n%s\nTerminal (experimental):\n%s\n", invitation.Role, invitation.Label, invitation.Browser, invitation.Terminal)
	}
}

func newWsSharingCommands(gf *GlobalFlags) []*cobra.Command {
	var role, label string
	invite := &cobra.Command{Use: "invite [name]", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		store := ws.NewStateStore("")
		name, _, err := resolveWorkspaceName(args, store)
		if err != nil {
			return err
		}
		service, err := workspaceShareService(gf, true)
		if err != nil {
			return err
		}
		invitation, err := service.Invite(cmd.Context(), sharing.InviteRequest{Label: label, Role: sharing.InvitationRole(role)})
		if err != nil {
			return err
		}
		printInvitations(cmd, []sharing.Invitation{invitation})
		_ = name
		return nil
	}}
	invite.Flags().StringVar(&role, "role", "", "Invitation role: interactive or observer")
	invite.Flags().StringVar(&label, "name", "", "Optional invitation label")
	_ = invite.MarkFlagRequired("role")

	revoke := &cobra.Command{Use: "revoke [name] INVITATION_LABEL", Args: cobra.RangeArgs(1, 2), RunE: func(cmd *cobra.Command, args []string) error {
		labelArg := args[len(args)-1]
		service, err := workspaceShareService(gf, true)
		if err != nil {
			return err
		}
		_, err = service.Revoke(cmd.Context(), labelArg)
		return err
	}}
	unshare := &cobra.Command{Use: "unshare [name]", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, _ []string) error {
		service, err := workspaceShareService(gf, true)
		if err != nil {
			return err
		}
		_, err = service.Stop(cmd.Context())
		return err
	}}
	return []*cobra.Command{invite, revoke, unshare, newWsShareGuardCmd(gf)}
}

func newWsShareGuardCmd(gf *GlobalFlags) *cobra.Command {
	var operationID, readyFile string
	cmd := &cobra.Command{Use: "share-guard", Hidden: true, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		store := sharing.NewFileStore("")
		provider := sharing.NewCloudflareProvider(osCommandRunner{})
		service, err := workspaceShareService(gf, false)
		if err != nil {
			return err
		}
		guardCtx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		return sharing.RunGuard(guardCtx, store, provider, service, operationID, readyFile)
	}}
	cmd.Flags().StringVar(&operationID, "operation", "", "sharing operation identity")
	cmd.Flags().StringVar(&readyFile, "ready-file", "", "guard readiness path")
	_ = cmd.MarkFlagRequired("operation")
	_ = cmd.MarkFlagRequired("ready-file")
	return cmd
}
