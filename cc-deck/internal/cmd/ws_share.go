package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/cc-deck/cc-deck/internal/config"
	sharing "github.com/cc-deck/cc-deck/internal/share"
	"github.com/cc-deck/cc-deck/internal/ws"
	"github.com/spf13/cobra"
)

// shareOptions carries the per-command endpoint selection. It overrides
// nothing in the user's configuration; it applies to one command only.
type shareOptions struct {
	endpoint     string
	endpointName string
	noVerify     bool
}

func workspaceShareService(gf *GlobalFlags, opts shareOptions) (sharing.Service, error) {
	runner := osCommandRunner{}
	store := sharing.NewFileStore("")
	zellij := sharing.NewZellij(runner)
	cfg, err := loadSharingConfig(gf)
	if err != nil {
		return nil, err
	}
	endpoint := sharing.NewStaticEndpoint(cfg.Sharing, opts.endpoint, opts.endpointName, zellij)
	return sharing.NewServiceWithTimeout(store, zellij, endpoint, cfg.VerifyTimeout()), nil
}

var makeWorkspaceShareService = workspaceShareService

// ensureReady is a seam so tests can drive the share path without provisioning
// real infrastructure.
var ensureReady = ws.EnsureReady

func loadSharingConfig(gf *GlobalFlags) (*config.Config, error) {
	configFile := ""
	if gf != nil {
		configFile = gf.ConfigFile
	}
	cfg, err := config.Load(configFile)
	if err != nil {
		return nil, fmt.Errorf("load sharing configuration: %w", err)
	}
	return cfg, nil
}

type osCommandRunner struct{}

func (osCommandRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

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

func readyAndMaybeShare(ctx context.Context, gf *GlobalFlags, workspace ws.Workspace, share bool, opts shareOptions) ([]sharing.Invitation, ws.ReadyResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if !share {
		ready, err := ws.EnsureReady(ctx, workspace, ws.ReadyOptions{})
		return nil, ready, err
	}
	if opts.endpoint != "" && opts.endpointName != "" {
		return nil, ws.ReadyResult{}, fmt.Errorf("--endpoint and --endpoint-name cannot be used together")
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
	service, err := makeWorkspaceShareService(gf, opts)
	if err != nil {
		return nil, ws.ReadyResult{}, err
	}
	// Stored state is enough to tell whether this workspace is already shared.
	// Start runs the verification gate itself, so probing twice would only
	// double the cost of every share.
	current, _ := service.Snapshot(ctx)
	ready, err := ensureWorkspaceReady(ctx, workspace, share, current, ensureReady)
	if err != nil {
		return nil, ready, err
	}
	invitations, err := service.Start(ctx, sharing.StartRequest{
		Workspace: workspace.Name(),
		Session:   ws.ZellijSessionName(workspace.Name()),
		NoVerify:  opts.noVerify,
	})
	// A workspace the user asked for stays. Sharing failing is not a reason to
	// destroy a session that is running and usable locally, which is what
	// killing it here used to do.
	if err != nil {
		err = explainShareFailure(err)
	}
	return invitations, ready, err
}

// explainShareFailure turns a verification failure into the message shape the
// CLI contract fixes: the failing stage named first, then what that layer
// means, then where to read more. Presentation lives here so the probe itself
// stays free of it.
func explainShareFailure(err error) error {
	var probeErr *sharing.ProbeFailedError
	if !errors.As(err, &probeErr) {
		return err
	}
	return fmt.Errorf("endpoint verification failed at the %s stage\n  %s\n  %s\n  See: cc-deck docs, sharing guide, endpoint requirements",
		probeErr.Result.FailedAt, probeErr.Detail(), stageExplanation(probeErr.Result.FailedAt))
}

// degradedSharingError turns a degraded report into a non-zero exit. The share
// itself is untouched: this says that something is wrong, not that anything was
// done about it.
func degradedSharingError(probe *sharing.ProbeResult, residuals []string) error {
	if probe != nil && !probe.OK {
		return fmt.Errorf("sharing is degraded: endpoint verification failed at the %s stage\n  %s\n  The share is intact. Repair the endpoint and run this command again",
			probe.FailedAt, stageExplanation(probe.FailedAt))
	}
	if len(residuals) > 0 {
		return fmt.Errorf("sharing is degraded: %s", strings.Join(residuals, "; "))
	}
	return fmt.Errorf("sharing is degraded")
}

// stageExplanation says what a failing layer means in terms of what the user
// can change, because naming the layer alone does not tell anyone what to fix.
func stageExplanation(stage sharing.ProbeStage) string {
	switch stage {
	case sharing.StageDNS:
		return "The address does not resolve from this machine. Check the name, and check whether a local DNS filter is answering for it."
	case sharing.StageTLS:
		return "The TLS handshake failed. Check the certificate the endpoint presents, including its name and its chain."
	case sharing.StageHTTP:
		return "The address did not serve the Zellij web client. It may not be answering at all, or the proxy may point somewhere other than the Zellij web server, or it may be rewriting paths."
	case sharing.StageAuth:
		return "Login did not return a session cookie. A proxy that drops POST bodies or strips Set-Cookie produces exactly this."
	case sharing.StageWebSocket:
		return "A proxy that forwards HTTP but drops the Upgrade and Connection headers produces a page that loads and a terminal that never fills."
	default:
		return "The endpoint could not be verified."
	}
}

func printInvitations(cmd *cobra.Command, invitations []sharing.Invitation) {
	for _, invitation := range invitations {
		for _, warning := range invitation.Warnings {
			fmt.Fprintln(cmd.OutOrStdout(), warning)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s invitation %q:\nBrowser:\n%s\nTerminal (experimental):\n%s\n", invitation.Role, invitation.Label, invitation.Browser, invitation.Terminal)
	}
}

// addEndpointFlags attaches the per-command endpoint selection shared by
// "start --share" and "invite".
func addEndpointFlags(cmd *cobra.Command, opts *shareOptions) {
	cmd.Flags().StringVar(&opts.endpoint, "endpoint", "", "Endpoint address to use for this command only")
	cmd.Flags().StringVar(&opts.endpointName, "endpoint-name", "", "Configured endpoint to select by name")
	cmd.Flags().BoolVar(&opts.noVerify, "no-verify", false, "Skip endpoint verification; the share reports no verification age")
	cmd.MarkFlagsMutuallyExclusive("endpoint", "endpoint-name")
}

func newWsSharingCommands(gf *GlobalFlags) []*cobra.Command {
	var role, label string
	var inviteOpts shareOptions
	invite := &cobra.Command{Use: "invite [name]", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		store := ws.NewStateStore("")
		name, _, err := resolveWorkspaceName(args, store)
		if err != nil {
			return err
		}
		service, err := makeWorkspaceShareService(gf, inviteOpts)
		if err != nil {
			return err
		}
		invitation, err := service.Invite(cmd.Context(), sharing.InviteRequest{
			Workspace: name, Label: label, Role: sharing.InvitationRole(role), NoVerify: inviteOpts.noVerify,
		})
		if err != nil {
			return explainShareFailure(err)
		}
		printInvitations(cmd, []sharing.Invitation{invitation})
		return nil
	}}
	invite.Flags().StringVar(&role, "role", "", "Invitation role: interactive or observer")
	invite.Flags().StringVar(&label, "name", "", "Optional invitation label")
	addEndpointFlags(invite, &inviteOpts)
	_ = invite.MarkFlagRequired("role")

	revoke := &cobra.Command{Use: "revoke [name] INVITATION_LABEL", Args: cobra.RangeArgs(1, 2), RunE: func(cmd *cobra.Command, args []string) error {
		labelArg := args[len(args)-1]
		nameArgs := args[:len(args)-1]
		name, _, err := resolveWorkspaceName(nameArgs, ws.NewStateStore(""))
		if err != nil {
			return err
		}
		service, err := makeWorkspaceShareService(gf, shareOptions{})
		if err != nil {
			return err
		}
		_, err = service.Revoke(cmd.Context(), name, labelArg)
		return err
	}}
	// Unshare never probes. Teardown must not depend on endpoint health, or an
	// endpoint that is down becomes a reason the share cannot be removed.
	unshare := &cobra.Command{Use: "unshare [name]", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		name, _, err := resolveWorkspaceName(args, ws.NewStateStore(""))
		if err != nil {
			return err
		}
		service, err := makeWorkspaceShareService(gf, shareOptions{})
		if err != nil {
			return err
		}
		_, err = service.Stop(cmd.Context(), name)
		return err
	}}
	return []*cobra.Command{invite, revoke, unshare}
}
