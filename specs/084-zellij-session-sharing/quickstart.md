# Quickstart: Validate Zellij Session Sharing

Run `make test`, `make lint`, and `make verify` from the repository root.

For manual acceptance, use supported Zellij, cloudflared, one running local session, and four client contexts. Start sharing; join twice interactively and twice as observers; verify full-session interaction versus rejected observer input; verify other sessions are unavailable; verify status hides tokens; stop and confirm disconnection and rejected old invitations. Repeat with reserved characters in the session name and forced provider/controller termination to validate reconciliation.

Terminal attachment is experimental in V1 and includes an explicit certificate-validation bypass. Perform it only after acknowledging the documented interception risk.

