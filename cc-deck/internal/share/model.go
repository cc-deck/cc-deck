package share

import (
	"fmt"
	"time"
)

type LifecycleState string

const (
	StateInactive LifecycleState = "inactive"
	StateStarting LifecycleState = "starting"
	StateActive   LifecycleState = "active"
	StateStopping LifecycleState = "stopping"
	// StateDegraded is a teardown residual marker only: it records that cleanup
	// left something behind. It is never written because an endpoint probe
	// failed. A failing probe reports degraded to the caller while the persisted
	// operation stays active, because an unreachable endpoint is not evidence
	// that the shared session ended.
	StateDegraded LifecycleState = "degraded"
)

// ProbeStage names one layer of the endpoint verification. The order below is
// the order the probe runs them in, and the probe stops at the first failure.
type ProbeStage string

const (
	StageDNS       ProbeStage = "dns"
	StageTLS       ProbeStage = "tls"
	StageHTTP      ProbeStage = "http"
	StageAuth      ProbeStage = "auth"
	StageWebSocket ProbeStage = "websocket"
)

// ProbeStages lists every declared stage in probe order. Tests assert that each
// one has a failure case, so a stage added without coverage fails the build.
var ProbeStages = []ProbeStage{StageDNS, StageTLS, StageHTTP, StageAuth, StageWebSocket}

// ProbeResult is an observation, never a state transition. Recording one must
// not move SharingOperation.State and must not trigger teardown.
type ProbeResult struct {
	OK         bool       `yaml:"ok"`
	FailedAt   ProbeStage `yaml:"failed_at,omitempty"`
	Diagnostic string     `yaml:"diagnostic,omitempty"`
	CheckedAt  time.Time  `yaml:"checked_at"`
}

// ProbeFailedError carries a failed ProbeResult out of the service so the CLI
// can render it. The stage and diagnostic travel as data rather than as
// prose, so presentation stays in the command layer.
type ProbeFailedError struct {
	Result   ProbeResult
	Endpoint string
}

func (e *ProbeFailedError) Error() string {
	return fmt.Sprintf("endpoint verification failed at the %s stage: %s", e.Result.FailedAt, e.Result.Diagnostic)
}

// Detail renders the address alongside what the probe observed, which is the
// first line of the failure message the CLI contract fixes.
func (e *ProbeFailedError) Detail() string {
	if e.Endpoint == "" {
		return e.Result.Diagnostic
	}
	return e.Endpoint + " " + e.Result.Diagnostic
}

type SharingOperation struct {
	ID        string `yaml:"id"`
	Workspace string `yaml:"workspace"`
	Session   string `yaml:"session"`
	// EndpointName is the configured endpoint this share used, empty when the
	// address came from a flag.
	EndpointName string `yaml:"endpoint_name,omitempty"`
	// EndpointURL is the address recorded at share time. Status re-probes this
	// value rather than the current configuration, so editing configuration
	// never silently retargets a live share.
	EndpointURL string             `yaml:"endpoint_url,omitempty"`
	Invitations []InvitationRecord `yaml:"invitations,omitempty"`
	// WebServerOwned is true only when EnsureWebServer reported that it started
	// the server. Teardown stops the web server only when this is set.
	WebServerOwned   bool `yaml:"web_server_owned,omitempty"`
	WebServerStopped bool `yaml:"web_server_stopped,omitempty"`
	// LastProbe is nil when the share was never verified, which is how a
	// --no-verify share reports no verification age.
	LastProbe *ProbeResult   `yaml:"last_probe,omitempty"`
	State     LifecycleState `yaml:"state"`
	CreatedAt time.Time      `yaml:"created_at"`
	UpdatedAt time.Time      `yaml:"updated_at"`
	Residuals []string       `yaml:"residuals,omitempty"`
}

type InvitationRole string

const (
	RoleInteractive InvitationRole = "interactive"
	RoleObserver    InvitationRole = "observer"
)

type InvitationState string

const (
	InvitationActive  InvitationState = "active"
	InvitationRevoked InvitationState = "revoked"
)

type InvitationRecord struct {
	Label          string          `yaml:"label"`
	CredentialName string          `yaml:"credential_name,omitempty"`
	Role           InvitationRole  `yaml:"role"`
	State          InvitationState `yaml:"state"`
	CreatedAt      time.Time       `yaml:"created_at"`
}

// Transition guards the forward path a healthy operation takes. It governs the
// start path only: teardown, rollback, and reconciliation assign State
// directly, because they must be able to record a degraded or stopping
// operation from any state including one this map would refuse. Read this as
// the happy path, not as the complete set of transitions the type undergoes.
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

type StartRequest struct {
	Workspace, Session string
	// NoVerify skips the verification gate. The resulting share carries no
	// LastProbe, which is what makes a listing show no verification age.
	NoVerify bool
}
type InviteRequest struct {
	Workspace string
	Label     string
	Role      InvitationRole
	NoVerify  bool
}
type InvitationSet struct {
	InteractiveBrowser, InteractiveTerminal string
	ObserverBrowser, ObserverTerminal       string
	Warnings                                []string
}

type Invitation struct {
	Label    string
	Browser  string
	Terminal string
	Role     InvitationRole
	Warnings []string
}
type SharingStatus struct {
	State                                         LifecycleState
	Workspace, Session, EndpointName, EndpointURL string
	LastProbe                                     *ProbeResult
	Invitations                                   []InvitationRecord
	InteractiveAvailable, ObserverAvailable       bool
	Residuals                                     []string
}
