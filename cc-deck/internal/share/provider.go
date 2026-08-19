package share

import (
	"context"
	"fmt"
	"os"
	"sort"
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
type TokenCredential struct{ Name, Secret string }
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
	Status(context.Context) (SharingStatus, error)
	Stop(context.Context, string) (SharingStatus, error)
}

type ProviderRegistry struct {
	providers map[string]Provider
}

func NewProviderRegistry(providers ...Provider) (*ProviderRegistry, error) {
	r := &ProviderRegistry{providers: make(map[string]Provider, len(providers))}
	for _, provider := range providers {
		if provider == nil || provider.Name() == "" {
			return nil, fmt.Errorf("provider name must not be empty")
		}
		if _, exists := r.providers[provider.Name()]; exists {
			return nil, fmt.Errorf("provider %q is registered more than once", provider.Name())
		}
		r.providers[provider.Name()] = provider
	}
	return r, nil
}

func (r *ProviderRegistry) Get(name string) (Provider, error) {
	if r == nil {
		return nil, fmt.Errorf("provider registry is not configured")
	}
	provider, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("provider %q is not available (available: %v)", name, r.Names())
	}
	return provider, nil
}

func (r *ProviderRegistry) Names() []string {
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
