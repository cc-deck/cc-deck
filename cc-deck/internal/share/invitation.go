package share

import (
	"fmt"
	"net/url"
	"strings"
)

const TrustedControlWarning = "WARNING: Interactive access gives trusted collaborators control of your complete terminal session."

func BuildInvitation(endpoint, session, label, token string, role InvitationRole) (Invitation, error) {
	remote, err := sessionURL(endpoint, session)
	if err != nil {
		return Invitation{}, err
	}
	// Only the interactive role carries a warning; an observer link controls
	// nothing, so it carries none.
	var warnings []string
	if role == RoleInteractive {
		warnings = []string{TrustedControlWarning}
	}
	return Invitation{
		Label: label, Role: role,
		URL: remote, Token: token,
		Browser:  browserInvitation(remote, token),
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
