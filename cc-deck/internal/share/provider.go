package share

import (
	"context"
	"os"
)

type Process interface {
	PID() int
	Wait() error
	Signal(os.Signal) error
	Kill() error
}
type CommandRunner interface {
	Run(context.Context, string, ...string) ([]byte, error)
	Start(context.Context, string, ...string) (Process, error)
}
type ProviderHandle struct {
	PID      int               `yaml:"pid"`
	Metadata map[string]string `yaml:"metadata,omitempty"`
}
type ProviderStatus struct{ State, EndpointURL, Diagnostic string }
type Provider interface {
	Name() string
	Validate(context.Context) error
	Start(context.Context, string) (ProviderHandle, error)
	Ready(context.Context, ProviderHandle) (ProviderStatus, error)
	Status(context.Context, ProviderHandle) (ProviderStatus, error)
	Stop(context.Context, ProviderHandle) error
}
type Zellij interface {
	ValidateCapabilities(context.Context) error
	ResolveSession(context.Context, string) (string, error)
	ShareSession(context.Context, string) (owned bool, err error)
	UnshareSession(context.Context, string) error
	CreateToken(context.Context, string, bool) (string, error)
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
	Start(context.Context, StartRequest) (InvitationSet, error)
	Status(context.Context) (SharingStatus, error)
	Stop(context.Context) (SharingStatus, error)
}
type Guard interface {
	Start(context.Context, string) (GuardHandle, error)
	Disarm(context.Context, GuardHandle) error
}
