package share

import (
	"fmt"
	"net/url"
	"strings"
)

const (
	TrustedControlWarning = "WARNING: Interactive access gives trusted collaborators control of your complete terminal session."
	TerminalTLSWarning    = "WARNING: Experimental terminal attachment uses --insecure, disabling server identity verification and allowing interception."
)

func BuildInvitations(endpoint, session, interactiveToken, observerToken string) (InvitationSet, error) {
	remote, err := sessionURL(endpoint, session)
	if err != nil {
		return InvitationSet{}, err
	}
	return InvitationSet{
		InteractiveBrowser:  browserInvitation(remote, interactiveToken),
		InteractiveTerminal: terminalInvitation(remote, interactiveToken),
		ObserverBrowser:     browserInvitation(remote, observerToken),
		ObserverTerminal:    terminalInvitation(remote, observerToken),
		Warnings:            []string{TrustedControlWarning},
	}, nil
}

func BuildInvitation(endpoint, session, label, token string, role InvitationRole) (Invitation, error) {
	remote, err := sessionURL(endpoint, session)
	if err != nil {
		return Invitation{}, err
	}
	// Only the control warning is carried. The terminal attachment command is
	// no longer surfaced by the CLI, so a warning about its use of --insecure
	// would caution against something the reader was never shown.
	var warnings []string
	if role == RoleInteractive {
		warnings = []string{TrustedControlWarning}
	}
	return Invitation{
		Label: label, Role: role,
		URL: remote, Token: token,
		Browser: browserInvitation(remote, token), Terminal: terminalInvitation(remote, token),
		Warnings: warnings,
	}, nil
}

func sessionURL(endpoint, session string) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("invalid sharing endpoint %q", endpoint)
	}
	u.RawQuery, u.Fragment = "", ""
	u.Path = strings.TrimRight(u.Path, "/") + "/" + session
	return u.String(), nil
}

func browserInvitation(remote, token string) string {
	return remote + "\nLogin token: " + token
}

func terminalInvitation(remote, token string) string {
	return "zellij attach " + shellQuote(remote) + " --token " + shellQuote(token) + " --insecure"
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
