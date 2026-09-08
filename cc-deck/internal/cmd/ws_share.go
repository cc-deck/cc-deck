package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

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

	// A live share cannot be re-pointed in place. An invitation embeds both the
	// endpoint and a login token, and the token is never stored, so a different
	// endpoint means new credentials and dead links. Start would otherwise
	// return success here without so much as resolving the address, which reads
	// as though the new endpoint had been accepted.
	if current.State == sharing.StateActive && current.Workspace == workspace.Name() {
		requested, asked, resolveErr := requestedEndpointURL(gf, opts)
		if resolveErr != nil {
			return nil, ws.ReadyResult{}, resolveErr
		}
		if asked && requested != current.EndpointURL {
			return nil, ws.ReadyResult{}, endpointConflictError(workspace.Name(), current, requested)
		}
		printActiveShare(os.Stdout, current)
	}
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

// runWsRetargetShare moves an active share to a different endpoint.
//
// The order matters. The new endpoint is verified first, so an address that
// does not work leaves the working share untouched rather than trading a
// functioning share for a broken one. Only once it has passed is the old share
// torn down and a new one issued.
//
// Invitations cannot be carried across. Each one pairs the endpoint with a
// login token, and the token is never stored, so the move necessarily mints new
// credentials and every link already distributed stops working.
func runWsRetargetShare(ctx context.Context, gf *GlobalFlags, name string, opts shareOptions, cmd *cobra.Command) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if opts.endpoint != "" && opts.endpointName != "" {
		return fmt.Errorf("--endpoint and --endpoint-name cannot be used together")
	}

	service, err := makeWorkspaceShareService(gf, opts)
	if err != nil {
		return err
	}
	current, err := service.Snapshot(ctx)
	if err != nil {
		return err
	}
	if current.State != sharing.StateActive || current.Workspace != name {
		return fmt.Errorf("workspace %q is not shared, so there is no endpoint to move\n"+
			"  Share it first: cc-deck ws start %s --share --endpoint ADDRESS", name, name)
	}

	requested, _, err := requestedEndpointURL(gf, opts)
	if err != nil {
		return err
	}
	if requested == current.EndpointURL {
		fmt.Fprintf(os.Stdout, "Workspace %q is already shared through %s; nothing to move\n", name, requested)
		return nil
	}

	if err = verifyEndpointBeforeMove(ctx, gf, opts, current.Session); err != nil {
		return err
	}

	if _, err = service.Stop(ctx, name); err != nil {
		return fmt.Errorf("could not stop the current share, so nothing was moved: %w", err)
	}
	invitations, err := service.Start(ctx, sharing.StartRequest{
		Workspace: name,
		Session:   current.Session,
		NoVerify:  opts.noVerify,
	})
	if err != nil {
		return fmt.Errorf("the share through %s was stopped, but the move to %s failed: %w\n"+
			"  The workspace is still running and is no longer shared.\n"+
			"  Retry with: cc-deck ws start %s --share --endpoint %s",
			current.EndpointURL, requested, explainShareFailure(err), name, requested)
	}

	fmt.Fprintf(os.Stdout, "Moved the share for %q to %s\n", name, requested)
	if revoked := invitationLabels(current.Invitations); revoked != "" {
		fmt.Fprintf(os.Stdout, "  revoked: %s\n", revoked)
	}
	if cmd != nil {
		printInvitations(cmd, invitations)
	}
	return nil
}

// invitationLabels joins the labels of invitations that a move leaves behind.
func invitationLabels(records []sharing.InvitationRecord) string {
	labels := make([]string, 0, len(records))
	for _, record := range records {
		labels = append(labels, record.Label)
	}
	return strings.Join(labels, ", ")
}

// verifyEndpointBeforeMove runs the same five-layer gate the share path runs,
// against the endpoint being moved to, before anything is torn down.
func verifyEndpointBeforeMove(ctx context.Context, gf *GlobalFlags, opts shareOptions, session string) error {
	cfg, err := loadSharingConfig(gf)
	if err != nil {
		return err
	}
	endpoint := sharing.NewStaticEndpoint(cfg.Sharing, opts.endpoint, opts.endpointName, sharing.NewZellij(osCommandRunner{}))
	ref, err := endpoint.Resolve(ctx)
	if err != nil {
		return err
	}
	if opts.noVerify {
		return nil
	}
	probeCtx, cancel := context.WithTimeout(ctx, cfg.VerifyTimeout())
	defer cancel()
	result, err := endpoint.Probe(probeCtx, ref, session)
	if err != nil {
		return explainShareFailure(err)
	}
	if !result.OK {
		return fmt.Errorf("endpoint verification failed at the %s stage, so nothing was moved\n  %s",
			result.FailedAt, stageExplanation(result.FailedAt))
	}
	return nil
}

// requestedEndpointURL resolves the endpoint asked for on this command line,
// and reports whether one was asked for at all.
//
// Only an explicit request is ever compared against a live share. A configured
// default that drifted after the share started is not a conflict the user
// raised here, and treating it as one would turn a harmless repeat of
// `ws start --share` into an error.
func requestedEndpointURL(gf *GlobalFlags, opts shareOptions) (string, bool, error) {
	if opts.endpoint == "" && opts.endpointName == "" {
		return "", false, nil
	}
	cfg, err := loadSharingConfig(gf)
	if err != nil {
		return "", false, err
	}
	// Resolve performs no network access, and reads no Zellij state, so the
	// resolver needs no Zellij handle.
	ref, err := sharing.NewStaticEndpoint(cfg.Sharing, opts.endpoint, opts.endpointName, nil).Resolve(context.Background())
	if err != nil {
		return "", false, err
	}
	return ref.BaseURL, true, nil
}

// endpointConflictError reports the refusal and the two commands that resolve
// it. Why a live share cannot be re-pointed belongs in the sharing guide, not
// in the way of someone who already knows what they meant to type.
func endpointConflictError(name string, current sharing.SharingStatus, requested string) error {
	return fmt.Errorf(
		"workspace %q is already shared through %s, so %s was not applied\n"+
			"  To move it:      cc-deck ws update %s --endpoint %s\n"+
			"  To stop sharing: cc-deck ws unshare %s",
		name, current.EndpointURL, requested, name, requested, name)
}

// printActiveShare reports what a repeated share found, so that the command
// says something rather than nothing.
//
// Login tokens are deliberately absent from stored state, so the invitations
// themselves cannot be reprinted. Labels and roles are what remain useful.
func printActiveShare(w io.Writer, status sharing.SharingStatus) {
	fmt.Fprintf(w, "Workspace %q is already shared through %s%s\n",
		status.Workspace, status.EndpointURL, verificationSuffix(status.LastProbe))
	for _, invitation := range status.Invitations {
		fmt.Fprintf(w, "  invitation %q (%s, %s)\n", invitation.Label, invitation.Role, invitation.State)
	}
	fmt.Fprintf(w, "  Login tokens are not stored. Run \"cc-deck ws invite %s\" for a new link.\n", status.Workspace)
}

// verificationSuffix renders the stored probe result, never a fresh check.
func verificationSuffix(probe *sharing.ProbeResult) string {
	if probe == nil || probe.CheckedAt.IsZero() {
		return ""
	}
	age := formatDuration(time.Since(probe.CheckedAt)) + " ago"
	if probe.OK {
		return fmt.Sprintf(" (verified %s)", age)
	}
	return fmt.Sprintf(" (verification failed at %s, %s)", probe.FailedAt, age)
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
// stageExplanation says what a failing layer means.
//
// Each explanation is pre-wrapped with the two-space continuation indent its
// callers use, because a single unbroken sentence of nearly two hundred
// characters is unreadable in the terminal where it lands.
func stageExplanation(stage sharing.ProbeStage) string {
	switch stage {
	case sharing.StageDNS:
		return "The address does not resolve from this machine. Check the name, and check\n  whether a local DNS filter is answering for it."
	case sharing.StageTLS:
		return "The TLS handshake failed. Check the certificate the endpoint presents,\n  including its name and its chain."
	case sharing.StageHTTP:
		return "The address did not serve the Zellij web client.\n  It may be down, the proxy may point elsewhere, or it may be rewriting paths."
	case sharing.StageAuth:
		return "Login did not return a session cookie. A proxy that drops POST bodies\n  or strips Set-Cookie produces exactly this."
	case sharing.StageWebSocket:
		return "A proxy that forwards HTTP but drops the Upgrade and Connection headers\n  produces a page that loads and a terminal that never fills."
	default:
		return "The endpoint could not be verified."
	}
}

// printInvitations lays out what a person needs to hand a link to someone.
//
// Every invitation for a share addresses the same session URL, so printing it
// once and listing the tokens beneath it is both shorter and easier to read
// than repeating the address, the warnings, and an attachment command for each
// role. The full form is still one --verbose away.
func printInvitations(cmd *cobra.Command, invitations []sharing.Invitation) {
	if len(invitations) == 0 {
		return
	}
	out := cmd.OutOrStdout()
	if verboseRequested(cmd) {
		printInvitationsVerbose(out, invitations)
		return
	}

	if url, uniform := sharedInvitationURL(invitations); uniform {
		fmt.Fprintln(out, url)
		for _, invitation := range invitations {
			fmt.Fprintf(out, "  %-11s %-14s %s\n", invitation.Role, invitation.Label, invitation.Token)
		}
	} else {
		for _, invitation := range invitations {
			fmt.Fprintf(out, "%-11s %-14s %s\n", invitation.Role, invitation.Label, invitation.URL)
			fmt.Fprintf(out, "  token %s\n", invitation.Token)
		}
	}

	if hasInteractive(invitations) {
		fmt.Fprintln(out, "\nInteractive access gives control of the complete terminal session.")
	}
	fmt.Fprintln(out, "Run with --verbose for terminal attachment commands.")
}

// printInvitationsVerbose keeps the long form: every warning, the browser
// invitation, and the experimental terminal command.
func printInvitationsVerbose(out io.Writer, invitations []sharing.Invitation) {
	for _, invitation := range invitations {
		for _, warning := range invitation.Warnings {
			fmt.Fprintln(out, warning)
		}
		fmt.Fprintf(out, "%s invitation %q:\nBrowser:\n%s\nTerminal (experimental):\n%s\n",
			invitation.Role, invitation.Label, invitation.Browser, invitation.Terminal)
	}
}

// sharedInvitationURL reports the one address every invitation uses, when they
// agree. They always do for a single share, but nothing here depends on that.
func sharedInvitationURL(invitations []sharing.Invitation) (string, bool) {
	url := invitations[0].URL
	if url == "" {
		return "", false
	}
	for _, invitation := range invitations[1:] {
		if invitation.URL != url {
			return "", false
		}
	}
	return url, true
}

func hasInteractive(invitations []sharing.Invitation) bool {
	for _, invitation := range invitations {
		if invitation.Role == sharing.RoleInteractive {
			return true
		}
	}
	return false
}

// verboseRequested reads the global --verbose flag, tolerating a command that
// was built without it.
func verboseRequested(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}
	verbose, err := cmd.Flags().GetBool("verbose")
	return err == nil && verbose
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
