package share

import "time"

type LifecycleState string

const (
	StateInactive LifecycleState = "inactive"
	StateStarting LifecycleState = "starting"
	StateActive   LifecycleState = "active"
	StateStopping LifecycleState = "stopping"
	StateDegraded LifecycleState = "degraded"
)

type SharingOperation struct {
	ID                    string         `yaml:"id"`
	Session               string         `yaml:"session"`
	Provider              string         `yaml:"provider"`
	EndpointURL           string         `yaml:"endpoint_url,omitempty"`
	InteractiveTokenLabel string         `yaml:"interactive_token_label"`
	ObserverTokenLabel    string         `yaml:"observer_token_label"`
	ProviderHandle        ProviderHandle `yaml:"provider_handle"`
	Guard                 GuardHandle    `yaml:"guard,omitempty"`
	State                 LifecycleState `yaml:"state"`
	CreatedAt             time.Time      `yaml:"created_at"`
	UpdatedAt             time.Time      `yaml:"updated_at"`
	Residuals             []string       `yaml:"residuals,omitempty"`
}

func (o *SharingOperation) Transition(next LifecycleState, now time.Time) bool {
	allowed := map[LifecycleState]map[LifecycleState]bool{
		StateStarting: {StateActive: true, StateDegraded: true},
		StateActive:   {StateStopping: true, StateDegraded: true},
		StateStopping: {StateDegraded: true},
		StateDegraded: {StateStopping: true},
	}
	if o == nil || !allowed[o.State][next] {
		return false
	}
	o.State, o.UpdatedAt = next, now.UTC()
	return true
}

type StartRequest struct{ Session, Provider string }
type InvitationSet struct {
	InteractiveBrowser, InteractiveTerminal string
	ObserverBrowser, ObserverTerminal       string
	Warnings                                []string
}
type SharingStatus struct {
	State                                   LifecycleState
	Session, Provider, EndpointURL          string
	InteractiveAvailable, ObserverAvailable bool
	Residuals                               []string
}
type GuardHandle struct {
	PID         int
	OperationID string
	Ready       bool
}
