package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/cc-deck/cc-deck/internal/config"
	sharing "github.com/cc-deck/cc-deck/internal/share"
	"github.com/cc-deck/cc-deck/internal/ws"
	"github.com/spf13/cobra"
)

func workspaceShareService(_ *GlobalFlags) (sharing.Service, error) {
	runner := osCommandRunner{}
	store := sharing.NewFileStore("")
	provider := sharing.NewCloudflareProvider(runner)
	zellij := sharing.NewZellij(runner)
	return sharing.NewService(store, zellij, provider), nil
}

var makeWorkspaceShareService = workspaceShareService

func configuredProvider(gf *GlobalFlags) (string, error) {
	configFile := ""
	if gf != nil {
		configFile = gf.ConfigFile
	}
	cfg, err := config.Load(configFile)
	if err != nil {
		return "", fmt.Errorf("load sharing configuration: %w", err)
	}
	if provider := cfg.SharingProvider(); provider != "" {
		return provider, nil
	}
	return "cloudflare", nil
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
	configChanged, configErr := sharing.EnsureZellijWebSharing("")
	if configErr != nil {
		return nil, ws.ReadyResult{}, fmt.Errorf("configure Zellij web sharing: %w", configErr)
	}
	if configChanged {
		fmt.Fprintln(os.Stderr, "Enabled web_sharing in Zellij config. A Zellij restart is required for this to take effect.")
		fmt.Fprintln(os.Stderr, "Run: cc-deck ws stop "+workspace.Name()+" && killall zellij && zellij --layout cc-deck")
		return nil, ws.ReadyResult{}, fmt.Errorf("web_sharing was just enabled in Zellij config; restart Zellij to activate it")
	}
	providerName, err := configuredProvider(gf)
	if err != nil {
		return nil, ws.ReadyResult{}, err
	}
	if providerName != "cloudflare" {
		return nil, ws.ReadyResult{}, fmt.Errorf("provider %q is not available", providerName)
	}
	service, err := makeWorkspaceShareService(gf)
	if err != nil {
		return nil, ws.ReadyResult{}, err
	}
	current, _ := service.Status(ctx)
	ready, err := ensureWorkspaceReady(ctx, workspace, share, current, ws.EnsureReady)
	if err != nil {
		return nil, ready, err
	}
	invitations, err := service.Start(ctx, sharing.StartRequest{Workspace: workspace.Name(), Session: ws.ZellijSessionName(workspace.Name()), Provider: providerName})
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
		service, err := makeWorkspaceShareService(gf)
		if err != nil {
			return err
		}
		invitation, err := service.Invite(cmd.Context(), sharing.InviteRequest{Workspace: name, Label: label, Role: sharing.InvitationRole(role)})
		if err != nil {
			return err
		}
		printInvitations(cmd, []sharing.Invitation{invitation})
		return nil
	}}
	invite.Flags().StringVar(&role, "role", "", "Invitation role: interactive or observer")
	invite.Flags().StringVar(&label, "name", "", "Optional invitation label")
	_ = invite.MarkFlagRequired("role")

	revoke := &cobra.Command{Use: "revoke [name] INVITATION_LABEL", Args: cobra.RangeArgs(1, 2), RunE: func(cmd *cobra.Command, args []string) error {
		labelArg := args[len(args)-1]
		nameArgs := args[:len(args)-1]
		name, _, err := resolveWorkspaceName(nameArgs, ws.NewStateStore(""))
		if err != nil {
			return err
		}
		service, err := makeWorkspaceShareService(gf)
		if err != nil {
			return err
		}
		_, err = service.Revoke(cmd.Context(), name, labelArg)
		return err
	}}
	unshare := &cobra.Command{Use: "unshare [name]", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		name, _, err := resolveWorkspaceName(args, ws.NewStateStore(""))
		if err != nil {
			return err
		}
		service, err := makeWorkspaceShareService(gf)
		if err != nil {
			return err
		}
		_, err = service.Stop(cmd.Context(), name)
		return err
	}}
	return []*cobra.Command{invite, revoke, unshare}
}
