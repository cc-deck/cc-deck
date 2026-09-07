package share

import (
	"context"
)

// CommandRunner runs a bounded external command and returns its output.
// There is deliberately no Start: cc-deck owns no long-lived process for
// sharing, so there is no process handle to hold and nothing to supervise.
type CommandRunner interface {
	Run(context.Context, string, ...string) ([]byte, error)
}

type TokenCredential struct{ Name, Secret string }

// EndpointRef is a resolved endpoint address. Name is the configured endpoint
// name, and is empty when the address came from a flag rather than a named
// configuration entry.
type EndpointRef struct{ Name, BaseURL string }

// Endpoint is a reverse proxy the user already runs in front of the Zellij web
// server. cc-deck never starts, stops, signals, restarts, or supervises it.
//
// Resolve reads configuration only and performs no network access, so callers
// that need an address never pay for a probe. Probe verifies reachability in
// five ordered stages and stops at the first failure; it is an observation and
// never mutates sharing state.
type Endpoint interface {
	Name() string
	Resolve(ctx context.Context) (EndpointRef, error)
	Probe(ctx context.Context, ref EndpointRef, session string) (ProbeResult, error)
}

type Zellij interface {
	ValidateCapabilities(context.Context) error
	SessionExists(context.Context, string) (bool, error)
	CreateToken(context.Context, string, bool) (TokenCredential, error)
	RevokeToken(context.Context, string) error
	EnsureWebServer(context.Context) (string, bool, error)
	StopWebServer(context.Context) error
}
type Store interface {
	WithLock(context.Context, func() error) error
	Load() (*SharingOperation, error)
	Save(*SharingOperation) error
	Remove() error
}
type Service interface {
	Start(context.Context, StartRequest) ([]Invitation, error)
	Invite(context.Context, InviteRequest) (Invitation, error)
	Revoke(context.Context, string, string) (SharingStatus, error)
	// Status verifies the endpoint before reporting. It probes.
	Status(context.Context) (SharingStatus, error)
	// Snapshot reports the stored state and nothing else. It performs no
	// network access and contacts no endpoint, whatever the state says, which
	// is what lets a listing stay as cheap as an unshared one.
	Snapshot(context.Context) (SharingStatus, error)
	Stop(context.Context, string) (SharingStatus, error)
}
