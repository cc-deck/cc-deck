// Package profile implements per-harness profile translators, wrapper script
// rendering, per-profile config directory preparation, color derivation, and
// the Sync/Provision entry points that generate wrappers for local, SSH and
// OpenShell workspaces.
//
// Each supported agent harness (Claude Code, Codex, OpenCode) has a Translator
// that knows how to turn a config.Profile into a POSIX sh wrapper, how to lay
// out a shared-but-auth-isolated config directory, and which OpenShell provider
// type the profile's backend maps to.
package profile
