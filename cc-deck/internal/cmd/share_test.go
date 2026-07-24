package cmd

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	sharing "github.com/cc-deck/cc-deck/internal/share"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

type fakeShareStarter struct {
	req        sharing.StartRequest
	set        sharing.InvitationSet
	err        error
	status     sharing.SharingStatus
	statusErr  error
	stopStatus sharing.SharingStatus
	stopErr    error
}

func (s *fakeShareStarter) Status(context.Context) (sharing.SharingStatus, error) {
	return s.status, s.statusErr
}
func (s *fakeShareStarter) Stop(context.Context) (sharing.SharingStatus, error) {
	return s.stopStatus, s.stopErr
}

func TestShareStatusPrintsSafeActiveFieldsWithoutCredentialLabels(t *testing.T) {
	service := &fakeShareStarter{status: sharing.SharingStatus{State: sharing.StateActive, Session: "selected", Provider: "cloudflare", EndpointURL: "https://public.example", InteractiveAvailable: true, ObserverAvailable: true}}
	command := newShareCmd(&GlobalFlags{}, service)
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetArgs([]string{"status"})
	require.NoError(t, command.Execute())
	require.Contains(t, output.String(), "State: active")
	require.Contains(t, output.String(), "Session: selected")
	require.NotContains(t, output.String(), "token")
	require.NotContains(t, output.String(), "SECRET")
}

func TestShareStatusDegradedPrintsResidualAndReturnsNonzeroError(t *testing.T) {
	service := &fakeShareStarter{status: sharing.SharingStatus{State: sharing.StateDegraded, Residuals: []string{"public endpoint may remain active"}}, statusErr: errors.New("cleanup incomplete")}
	command := newShareCmd(&GlobalFlags{}, service)
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetArgs([]string{"status"})
	require.Error(t, command.Execute())
	require.Contains(t, output.String(), "Residual exposure: public endpoint")
}

func TestShareStopIsIdempotentAndPrintsInactive(t *testing.T) {
	service := &fakeShareStarter{stopStatus: sharing.SharingStatus{State: sharing.StateInactive}}
	command := newShareCmd(&GlobalFlags{}, service)
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetArgs([]string{"stop"})
	require.NoError(t, command.Execute())
	require.Contains(t, output.String(), "Sharing is inactive")
}

func TestShareStopFailurePrintsResidualAndReturnsError(t *testing.T) {
	service := &fakeShareStarter{stopStatus: sharing.SharingStatus{State: sharing.StateDegraded, Residuals: []string{"observer credential may remain active"}}, stopErr: errors.New("cleanup incomplete")}
	command := newShareCmd(&GlobalFlags{}, service)
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetArgs([]string{"stop"})
	require.Error(t, command.Execute())
	require.Contains(t, output.String(), "observer credential")
}

func TestShareProviderSelectionAndCompletionUseRegistryNames(t *testing.T) {
	service := &fakeShareStarter{set: sharing.InvitationSet{Warnings: []string{"already active"}}}
	var selected string
	command := newShareCmdWithFactory(&GlobalFlags{}, func(_ context.Context, provider string, _ bool) (shareLifecycle, error) {
		selected = provider
		if provider == "missing" {
			return nil, errors.New("provider is not available")
		}
		return service, nil
	}, []string{"cloudflare", "fake"})
	command.SetArgs([]string{"start", "selected", "--provider", "fake"})
	require.NoError(t, command.Execute())
	require.Equal(t, "fake", selected)

	start, _, err := command.Find([]string{"start"})
	require.NoError(t, err)
	completion, ok := start.GetFlagCompletionFunc("provider")
	require.True(t, ok)
	values, directive := completion(start, nil, "")
	require.Equal(t, []string{"cloudflare", "fake"}, values)
	require.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
}

func TestShareUnknownProviderFailsBeforeStart(t *testing.T) {
	command := newShareCmdWithFactory(&GlobalFlags{}, func(_ context.Context, provider string, _ bool) (shareLifecycle, error) {
		return nil, errors.New("provider " + provider + " is not available")
	}, []string{"cloudflare"})
	command.SetArgs([]string{"start", "selected", "--provider", "missing"})
	require.ErrorContains(t, command.Execute(), "not available")
}

func (s *fakeShareStarter) Start(_ context.Context, req sharing.StartRequest) (sharing.InvitationSet, error) {
	s.req = req
	return s.set, s.err
}

func TestShareStartPrintsWarningsBeforeFourRoleLabeledInvitations(t *testing.T) {
	service := &fakeShareStarter{set: sharing.InvitationSet{
		InteractiveBrowser: "IB SECRET-I", InteractiveTerminal: "IT SECRET-I --insecure",
		ObserverBrowser: "OB SECRET-O", ObserverTerminal: "OT SECRET-O --insecure",
		Warnings: []string{sharing.TrustedControlWarning, sharing.TerminalTLSWarning},
	}}
	command := newShareCmd(&GlobalFlags{}, service)
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetArgs([]string{"start", "session with spaces", "--provider", "cloudflare"})
	require.NoError(t, command.Execute())

	got := output.String()
	require.Equal(t, sharing.StartRequest{Session: "session with spaces", Provider: "cloudflare"}, service.req)
	for _, label := range []string{"Interactive browser", "Interactive terminal", "Observer browser", "Observer terminal"} {
		require.Contains(t, got, label)
	}
	require.Less(t, strings.Index(got, sharing.TrustedControlWarning), strings.Index(got, "SECRET-I"))
	require.Less(t, strings.Index(got, sharing.TerminalTLSWarning), strings.Index(got, "--insecure"))
	require.Contains(t, got, "read-only")
}

func TestShareStartFailureSuppressesAllInvitations(t *testing.T) {
	service := &fakeShareStarter{set: sharing.InvitationSet{InteractiveBrowser: "MUST-NOT-PRINT"}, err: errors.New("provider readiness failed")}
	command := newShareCmd(&GlobalFlags{}, service)
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs([]string{"start", "selected"})
	err := command.Execute()
	require.ErrorContains(t, err, "provider readiness failed")
	require.NotContains(t, output.String(), "MUST-NOT-PRINT")
}

func TestShareStartRequiresSafeUnambiguousSelectionFromService(t *testing.T) {
	service := &fakeShareStarter{err: errors.New("select one Zellij session explicitly")}
	command := newShareCmd(&GlobalFlags{}, service)
	command.SetArgs([]string{"start"})
	err := command.Execute()
	require.ErrorContains(t, err, "select one Zellij session explicitly")
	require.Empty(t, service.req.Session)
}

func TestShareStartIdempotentResultDoesNotPrintEmptyInvitationSections(t *testing.T) {
	service := &fakeShareStarter{set: sharing.InvitationSet{Warnings: []string{"already active; credentials are not redisplayed"}}}
	command := newShareCmd(&GlobalFlags{}, service)
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetArgs([]string{"start", "selected"})
	require.NoError(t, command.Execute())
	require.Contains(t, output.String(), "already active")
	require.NotContains(t, output.String(), "invitation")
}
