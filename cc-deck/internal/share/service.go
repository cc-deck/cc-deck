package share

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

type SharingService struct {
	store    Store
	zellij   Zellij
	endpoint Endpoint
	// verifyTimeout bounds the whole probe, not each stage. A per stage budget
	// multiplies to several times the intended ceiling, which is how a bound
	// becomes a hang.
	verifyTimeout time.Duration
	now           func() time.Time
	labels        *LabelGenerator
}

const rollbackTimeout = 5 * time.Second

// DefaultVerifyTimeout matches config.DefaultVerifyTimeout and is used when a
// caller does not supply one.
const DefaultVerifyTimeout = 15 * time.Second

func NewService(store Store, zellij Zellij, endpoint Endpoint) *SharingService {
	return NewServiceWithTimeout(store, zellij, endpoint, DefaultVerifyTimeout)
}

func NewServiceWithTimeout(store Store, zellij Zellij, endpoint Endpoint, verifyTimeout time.Duration) *SharingService {
	if verifyTimeout <= 0 {
		verifyTimeout = DefaultVerifyTimeout
	}
	return &SharingService{
		store: store, zellij: zellij, endpoint: endpoint,
		verifyTimeout: verifyTimeout, now: time.Now, labels: NewLabelGenerator(nil),
	}
}

func (s *SharingService) Start(ctx context.Context, req StartRequest) ([]Invitation, error) {
	var invitations []Invitation
	err := s.store.WithLock(ctx, func() error {
		existing, err := s.store.Load()
		if err != nil {
			return err
		}
		if existing != nil {
			if existing.State == StateActive {
				if (req.Workspace == "" || req.Workspace == existing.Workspace) &&
					(req.Session == "" || req.Session == existing.Session) {
					invitations = nil
					return nil
				}
				return fmt.Errorf("workspace %q is already shared; unshare it first", existing.Workspace)
			}
			// A non-active operation is residue from an interrupted run. Clear it
			// before issuing new invitations. Endpoint health is deliberately not
			// consulted here: an unreachable endpoint is not evidence that the
			// shared session ended, so it must never trigger teardown.
			var reconciled SharingStatus
			if err := s.reconcileLocked(ctx, existing, &reconciled, "previous sharing operation was "+string(existing.State)); err != nil {
				return fmt.Errorf("cannot start while stale sharing resources remain: %w", err)
			}
		}
		if err = s.zellij.ValidateCapabilities(ctx); err != nil {
			return err
		}
		if req.Session == "" {
			return fmt.Errorf("canonical session is required")
		}
		exists, err := s.zellij.SessionExists(ctx, req.Session)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("canonical session %q does not exist", req.Session)
		}
		session := req.Session

		ref, err := s.endpoint.Resolve(ctx)
		if err != nil {
			return err
		}

		id, err := operationID()
		if err != nil {
			return err
		}
		now := s.now().UTC()
		interactiveLabel, err := s.labels.Next(nil)
		if err != nil {
			return err
		}
		observerLabel, err := s.labels.Next(map[string]bool{interactiveLabel: true})
		if err != nil {
			return err
		}
		op := &SharingOperation{
			ID: id, Workspace: req.Workspace, Session: session,
			EndpointName: ref.Name, EndpointURL: ref.BaseURL,
			Invitations: []InvitationRecord{
				{Label: interactiveLabel, Role: RoleInteractive, State: InvitationActive, CreatedAt: now},
				{Label: observerLabel, Role: RoleObserver, State: InvitationActive, CreatedAt: now},
			},
			State: StateStarting, CreatedAt: now, UpdatedAt: now,
		}

		var undo []func(context.Context) error
		persisted := false
		rollback := func(cause error) error {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), rollbackTimeout)
			defer cancel()
			var residuals []string
			for i := len(undo) - 1; i >= 0; i-- {
				if rollbackErr := undo[i](cleanupCtx); rollbackErr != nil {
					residuals = append(residuals, rollbackErr.Error())
				}
			}
			if persisted {
				if rollbackErr := s.store.Remove(); rollbackErr != nil {
					residuals = append(residuals, "operation state could not be removed: "+rollbackErr.Error())
				}
			}
			if len(residuals) == 0 {
				return cause
			}
			op.State, op.UpdatedAt, op.Residuals = StateDegraded, s.now().UTC(), residuals
			_ = s.store.Save(op)
			return fmt.Errorf("%w; rollback residuals: %s", cause, strings.Join(residuals, "; "))
		}

		_, webStarted, err := s.zellij.EnsureWebServer(ctx)
		if err != nil {
			return rollback(err)
		}
		// Ownership is recorded at the moment cc-deck acts, never inferred later.
		// Teardown stops the web server only when this is true.
		op.WebServerOwned = webStarted
		if webStarted {
			undo = append(undo, func(cleanupCtx context.Context) error { return s.zellij.StopWebServer(cleanupCtx) })
		}
		interactiveCredential, err := s.zellij.CreateToken(ctx, interactiveLabel, false)
		if err != nil {
			return rollback(err)
		}
		undo = append(undo, func(cleanupCtx context.Context) error {
			return s.zellij.RevokeToken(cleanupCtx, interactiveCredential.Name)
		})
		op.Invitations[0].CredentialName = interactiveCredential.Name
		observerCredential, err := s.zellij.CreateToken(ctx, observerLabel, true)
		if err != nil {
			return rollback(err)
		}
		op.Invitations[1].CredentialName = observerCredential.Name
		undo = append(undo, func(cleanupCtx context.Context) error {
			return s.zellij.RevokeToken(cleanupCtx, observerCredential.Name)
		})

		// The verification gate. It blocks: no invitation is printed until the
		// endpoint has been shown to reach a live terminal, because an
		// invitation to a blank page is worse than no invitation at all.
		if probe, err := s.verify(ctx, ref, session, req.NoVerify); err != nil {
			return rollback(err)
		} else {
			op.LastProbe = probe
		}

		op.Transition(StateActive, s.now())
		interactive, err := BuildInvitation(ref.BaseURL, session, interactiveLabel, interactiveCredential.Secret, RoleInteractive)
		if err != nil {
			return rollback(err)
		}
		observer, err := BuildInvitation(ref.BaseURL, session, observerLabel, observerCredential.Secret, RoleObserver)
		if err != nil {
			return rollback(err)
		}
		invitations = []Invitation{interactive, observer}
		if err = s.store.Save(op); err != nil {
			return rollback(err)
		}
		persisted = true
		return nil
	})
	if err != nil {
		return nil, err
	}
	return invitations, nil
}

// verify runs the endpoint probe under a single deadline derived from the
// configured budget, following the convention ZellijCLI.run already uses:
// impose the default bound only when the caller has not set a shorter one.
//
// When skipped it returns a nil result rather than a zero value, because a
// zero timestamp would render as an age and claim a verification that never
// happened.
func (s *SharingService) verify(ctx context.Context, ref EndpointRef, session string, skip bool) (*ProbeResult, error) {
	if skip {
		return nil, nil
	}
	probeCtx := ctx
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		probeCtx, cancel = context.WithTimeout(ctx, s.verifyTimeout)
		defer cancel()
	}
	result, err := s.endpoint.Probe(probeCtx, ref, session)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func operationID() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate sharing operation ID: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

func (s *SharingService) Invite(ctx context.Context, req InviteRequest) (Invitation, error) {
	var invitation Invitation
	err := s.store.WithLock(ctx, func() error {
		op, err := s.store.Load()
		if err != nil {
			return err
		}
		if op == nil || op.State != StateActive {
			return fmt.Errorf("no active sharing operation")
		}
		if req.Workspace != "" && req.Workspace != op.Workspace {
			return fmt.Errorf("workspace %q is not shared; active share belongs to %q", req.Workspace, op.Workspace)
		}
		exists, err := s.zellij.SessionExists(ctx, op.Session)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("canonical session %q does not exist", op.Session)
		}
		if req.Role != RoleInteractive && req.Role != RoleObserver {
			return fmt.Errorf("invitation role must be interactive or observer")
		}
		existing := map[string]bool{}
		for _, record := range op.Invitations {
			existing[record.Label] = true
		}
		label := req.Label
		if label == "" {
			label, err = s.labels.Next(existing)
			if err != nil {
				return err
			}
		}
		if existing[label] {
			return fmt.Errorf("invitation label %q already exists", label)
		}
		// The same blocking gate as Start, and for the same reason. It runs
		// before any credential is minted, so a failed verification leaves
		// nothing behind to clean up.
		probe, err := s.verify(ctx, EndpointRef{Name: op.EndpointName, BaseURL: op.EndpointURL}, op.Session, req.NoVerify)
		if err != nil {
			return err
		}
		if probe != nil {
			op.LastProbe = probe
		}
		credential, err := s.zellij.CreateToken(ctx, label, req.Role == RoleObserver)
		if err != nil {
			return err
		}
		invitation, err = BuildInvitation(op.EndpointURL, op.Session, label, credential.Secret, req.Role)
		if err != nil {
			_ = s.zellij.RevokeToken(context.Background(), credential.Name)
			return err
		}
		op.Invitations = append(op.Invitations, InvitationRecord{Label: label, CredentialName: credential.Name, Role: req.Role, State: InvitationActive, CreatedAt: s.now().UTC()})
		op.UpdatedAt = s.now().UTC()
		if err := s.store.Save(op); err != nil {
			_ = s.zellij.RevokeToken(context.Background(), credential.Name)
			return err
		}
		return nil
	})
	return invitation, err
}

func (s *SharingService) Revoke(ctx context.Context, workspace, label string) (SharingStatus, error) {
	var status SharingStatus
	err := s.store.WithLock(ctx, func() error {
		op, err := s.store.Load()
		if err != nil {
			return err
		}
		if op == nil {
			return fmt.Errorf("no active sharing operation")
		}
		if workspace != "" && workspace != op.Workspace {
			return fmt.Errorf("workspace %q is not shared; active share belongs to %q", workspace, op.Workspace)
		}
		for i := range op.Invitations {
			if op.Invitations[i].Label != label {
				continue
			}
			if op.Invitations[i].State == InvitationRevoked {
				status = statusFromOperation(op)
				return nil
			}
			if err := s.zellij.RevokeToken(ctx, credentialName(op.Invitations[i])); err != nil {
				return err
			}
			op.Invitations[i].State = InvitationRevoked
			op.UpdatedAt = s.now().UTC()
			if err := s.store.Save(op); err != nil {
				return err
			}
			status = statusFromOperation(op)
			return nil
		}
		return fmt.Errorf("invitation %q not found", label)
	})
	return status, err
}

func (s *SharingService) Status(ctx context.Context) (SharingStatus, error) {
	var status SharingStatus
	err := s.store.WithLock(ctx, func() error {
		op, err := s.store.Load()
		if err != nil {
			return err
		}
		if op == nil {
			status = SharingStatus{State: StateInactive}
			return nil
		}

		status = statusFromOperation(op)
		sessionExists, sessionErr := s.zellij.SessionExists(ctx, op.Session)
		if errors.Is(sessionErr, ErrZellijUnresponsive) {
			return reportUnverified(op, &status, sessionErr)
		}
		if sessionErr != nil || !sessionExists {
			// A session positively reported absent is the one legitimate teardown
			// trigger. Nothing else reaches reconcileLocked from here.
			diagnostic := "canonical session disappeared"
			if sessionErr != nil {
				diagnostic = sessionErr.Error()
			}
			return s.reconcileLocked(ctx, op, &status, diagnostic)
		}
		if op.State != StateActive {
			return s.reconcileLocked(ctx, op, &status, "sharing operation is "+string(op.State))
		}

		// The endpoint is verified for the caller's benefit, and the result is
		// recorded. Neither the outcome nor the recording moves the operation's
		// state or triggers teardown: an endpoint being unreachable is not
		// evidence that the shared session ended, and treating it as such is
		// what once destroyed a live share from an ordinary listing.
		//
		// The address probed is the one recorded when the share was created,
		// not the currently configured one, so editing configuration never
		// silently retargets a live share.
		ref := EndpointRef{Name: op.EndpointName, BaseURL: op.EndpointURL}
		probe, probeErr := s.verify(ctx, ref, op.Session, false)
		if probeErr != nil {
			var failed *ProbeFailedError
			if !errors.As(probeErr, &failed) {
				// The probe could not reach a verdict. That is not evidence of
				// anything, so nothing is recorded and nothing is touched.
				return reportUnverified(op, &status, probeErr)
			}
			s.recordProbe(op, &failed.Result)
			status = statusFromOperation(op)
			// Exactly two reported values, never a third.
			status.State = StateDegraded
			status.Residuals = append(status.Residuals,
				fmt.Sprintf("endpoint verification failed at the %s stage: %s", failed.Result.FailedAt, failed.Result.Diagnostic))
			return probeErr
		}
		s.recordProbe(op, probe)
		status = statusFromOperation(op)
		return nil
	})
	return status, err
}

// recordProbe persists an observation. It deliberately does not call
// Transition and does not touch State: writing what was seen must never be a
// state change, or a report becomes an action.
func (s *SharingService) recordProbe(op *SharingOperation, probe *ProbeResult) {
	if probe == nil {
		return
	}
	op.LastProbe = probe
	op.UpdatedAt = s.now().UTC()
	_ = s.store.Save(op)
}

// Snapshot reports the stored sharing state without probing anything. A
// listing uses this: it renders the last recorded result and its age, and
// performs no network access at any cost.
func (s *SharingService) Snapshot(ctx context.Context) (SharingStatus, error) {
	var status SharingStatus
	err := s.store.WithLock(ctx, func() error {
		op, err := s.store.Load()
		if err != nil {
			return err
		}
		status = statusFromOperation(op)
		return nil
	})
	return status, err
}

func (s *SharingService) Stop(ctx context.Context, workspace string) (SharingStatus, error) {
	var status SharingStatus
	err := s.store.WithLock(ctx, func() error {
		op, err := s.store.Load()
		if err != nil {
			return err
		}
		if op == nil {
			status = SharingStatus{State: StateInactive}
			return nil
		}
		if workspace != "" && workspace != op.Workspace {
			return fmt.Errorf("workspace %q is not shared; active share belongs to %q", workspace, op.Workspace)
		}
		return s.teardownLocked(ctx, op, &status)
	})
	return status, err
}

func statusFromOperation(op *SharingOperation) SharingStatus {
	if op == nil {
		return SharingStatus{State: StateInactive}
	}
	status := SharingStatus{
		State: op.State, Workspace: op.Workspace, Session: op.Session,
		EndpointName: op.EndpointName, EndpointURL: op.EndpointURL,
		LastProbe:   op.LastProbe,
		Residuals:   append([]string(nil), op.Residuals...),
		Invitations: append([]InvitationRecord(nil), op.Invitations...),
	}
	if op.State == StateActive {
		for _, invitation := range op.Invitations {
			if invitation.State != InvitationActive {
				continue
			}
			status.InteractiveAvailable = status.InteractiveAvailable || invitation.Role == RoleInteractive
			status.ObserverAvailable = status.ObserverAvailable || invitation.Role == RoleObserver
		}
	}
	return status
}

// reportUnverified surfaces an operation whose health could not be determined.
// It deliberately leaves every resource in place and does not persist the
// degraded state: a wedged or slow Zellij server is not evidence that sharing
// ended, and tearing down here would revoke live credentials and close a
// working endpoint. Persisting StateDegraded would be just as harmful, because
// a degraded operation is reconciled (torn down) on the next healthy call.
func reportUnverified(op *SharingOperation, status *SharingStatus, cause error) error {
	*status = statusFromOperation(op)
	status.State = StateDegraded
	status.Residuals = append(status.Residuals, "sharing state could not be verified: "+cause.Error())
	return fmt.Errorf("sharing state could not be verified, leaving resources untouched: %w", cause)
}

// reconcileLocked tears down an operation that is known to be over. It is
// reached only when the canonical session is confirmed absent, or when a
// non-active operation is found as residue from an interrupted run. An
// endpoint that fails verification never reaches here: the endpoint being
// unreachable is not evidence that the shared session ended, and tearing down
// on it once destroyed live shares from an ordinary listing.
func (s *SharingService) reconcileLocked(ctx context.Context, op *SharingOperation, status *SharingStatus, diagnostic string) error {
	if diagnostic == "" {
		diagnostic = "sharing operation is no longer live"
	}
	op.State = StateDegraded
	op.Residuals = []string{diagnostic}
	op.UpdatedAt = s.now().UTC()
	_ = s.store.Save(op)
	return s.teardownLocked(ctx, op, status)
}

func (s *SharingService) teardownLocked(ctx context.Context, op *SharingOperation, status *SharingStatus) error {
	op.State = StateStopping
	op.UpdatedAt = s.now().UTC()
	op.Residuals = nil
	// Persist stopping before mutation so a killed command is reconciled later.
	var residuals []string
	if err := s.store.Save(op); err != nil {
		// Persistence failure is itself a residual, but must never prevent the safety
		// actions below. A state-store problem cannot justify leaving access active.
		residuals = append(residuals, "stopping state could not be persisted: "+err.Error())
	}
	type cleanupStep struct {
		residual string
		run      func() error
	}
	var steps []cleanupStep
	// There is no endpoint stop step. cc-deck never started the endpoint, so it
	// has nothing to stop and no right to stop it.
	for i := len(op.Invitations) - 1; i >= 0; i-- {
		invitationIndex := i
		invitation := op.Invitations[invitationIndex]
		if invitation.State != InvitationActive {
			continue
		}
		steps = append(steps, cleanupStep{
			fmt.Sprintf("%s credential %q may remain active", invitation.Role, invitation.Label),
			func() error {
				if err := s.zellij.RevokeToken(ctx, credentialName(invitation)); err != nil {
					return err
				}
				op.Invitations[invitationIndex].State = InvitationRevoked
				op.UpdatedAt = s.now().UTC()
				if err := s.store.Save(op); err != nil {
					return fmt.Errorf("credential revoked but cleanup progress could not be persisted: %w", err)
				}
				return nil
			},
		})
	}
	// Only a web server cc-deck started is cc-deck's to stop. A user who was
	// already running "zellij web" keeps it.
	if op.WebServerOwned && !op.WebServerStopped {
		steps = append(steps, cleanupStep{"remote clients may remain connected", func() error {
			if err := s.zellij.StopWebServer(ctx); err != nil {
				return err
			}
			op.WebServerStopped, op.UpdatedAt = true, s.now().UTC()
			if err := s.store.Save(op); err != nil {
				return fmt.Errorf("web server stopped but cleanup progress could not be persisted: %w", err)
			}
			return nil
		}})
	}
	for _, step := range steps {
		if err := step.run(); err != nil {
			residuals = append(residuals, step.residual+": "+err.Error())
		}
	}
	if len(residuals) == 0 {
		if err := s.store.Remove(); err == nil {
			*status = SharingStatus{State: StateInactive}
			return nil
		} else {
			residuals = append(residuals, "operation state could not be removed: "+err.Error())
		}
	}

	op.State, op.UpdatedAt, op.Residuals = StateDegraded, s.now().UTC(), residuals
	saveErr := s.store.Save(op)
	*status = statusFromOperation(op)
	if saveErr != nil {
		return fmt.Errorf("sharing cleanup incomplete: %s; persist degraded state: %v", strings.Join(residuals, "; "), saveErr)
	}
	return fmt.Errorf("sharing cleanup incomplete: %s", strings.Join(residuals, "; "))
}

func credentialName(invitation InvitationRecord) string {
	if invitation.CredentialName != "" {
		return invitation.CredentialName
	}
	return invitation.Label
}
