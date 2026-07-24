package cmd

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	sharing "github.com/cc-deck/cc-deck/internal/share"
	"github.com/stretchr/testify/require"
)

type fakeShareStarter struct {
	req sharing.StartRequest
	set sharing.InvitationSet
	err error
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
