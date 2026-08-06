package share

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

type SharingService struct {
	store    Store
	zellij   Zellij
	provider Provider
	now      func() time.Time
	labels   *LabelGenerator
}

const rollbackTimeout = 5 * time.Second

func NewService(store Store, zellij Zellij, provider Provider) *SharingService {
	return &SharingService{store: store, zellij: zellij, provider: provider, now: time.Now, labels: NewLabelGenerator(nil)}
}

func (s *SharingService) Start(ctx context.Context, req StartRequest) ([]Invitation, error) {
	var invitations []Invitation
	err := s.store.WithLock(ctx, func() error {
		existing, err := s.store.Load()
		if err != nil {
			return err
		}
		if existing != nil {
			providerStatus, statusErr := s.provider.Status(ctx, existing.ProviderHandle)
			if existing.State == StateActive && statusErr == nil &&
				(providerStatus.State == "ready" || providerStatus.State == "starting") {
				if (req.Workspace == "" || req.Workspace == existing.Workspace) &&
					(req.Session == "" || req.Session == existing.Session) &&
					(req.Provider == "" || req.Provider == existing.Provider) {
					invitations = nil
					return nil
				}
				return fmt.Errorf("workspace %q is already shared; unshare it first", existing.Workspace)
			}

			var reconciled SharingStatus
			if err := s.reconcileLocked(ctx, existing, &reconciled, statusErr, providerStatus); err != nil {
				return fmt.Errorf("cannot start while stale sharing resources remain: %w", err)
			}
		}
		if err = s.zellij.ValidateCapabilities(ctx); err != nil {
			return err
		}
		if err = s.provider.Validate(ctx); err != nil {
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

		id, err := operationID()
		if err != nil {
			return err
		}
		providerName := req.Provider
		if providerName == "" {
			providerName = s.provider.Name()
		}
		if providerName != s.provider.Name() {
			return fmt.Errorf("provider %q is not available", providerName)
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
			ID: id, Workspace: req.Workspace, Session: session, Provider: providerName,
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

		localURL, webStarted, err := s.zellij.EnsureWebServer(ctx)
		if err != nil {
			return rollback(err)
		}
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
		handle, err := s.provider.Start(ctx, localURL)
		if err != nil {
			return rollback(err)
		}
		op.ProviderHandle = handle
		undo = append(undo, func(cleanupCtx context.Context) error { return s.provider.Stop(cleanupCtx, handle) })
		ready, err := s.provider.Ready(ctx, handle)
		if err != nil {
			return rollback(err)
		}
		if ready.State != "ready" || ready.EndpointURL == "" {
			return rollback(fmt.Errorf("provider did not return a ready public endpoint: %s", ready.Diagnostic))
		}
		op.EndpointURL = ready.EndpointURL
		if op.ProviderHandle.Metadata == nil {
			op.ProviderHandle.Metadata = map[string]string{}
		}
		op.ProviderHandle.Metadata["endpoint"] = ready.EndpointURL
		op.Transition(StateActive, s.now())
		interactive, err := BuildInvitation(ready.EndpointURL, session, interactiveLabel, interactiveCredential.Secret, RoleInteractive)
		if err != nil {
			return rollback(err)
		}
		observer, err := BuildInvitation(ready.EndpointURL, session, observerLabel, observerCredential.Secret, RoleObserver)
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
		if sessionErr != nil || !sessionExists {
			diagnostic := "canonical session disappeared"
			if sessionErr != nil {
				diagnostic = sessionErr.Error()
			}
			return s.reconcileLocked(ctx, op, &status, fmt.Errorf("%s", diagnostic), ProviderStatus{})
		}
		providerStatus, providerErr := s.provider.Status(ctx, op.ProviderHandle)
		if op.State == StateActive && providerErr == nil && (providerStatus.State == "ready" || providerStatus.State == "starting") {
			if providerStatus.EndpointURL != "" {
				status.EndpointURL = providerStatus.EndpointURL
			}
			return nil
		}

		// A persisted operation whose endpoint is no longer healthy is stale. Reconcile
		// every resource before returning so status never reports a dead operation as
		// active. Cleanup is intentionally performed while holding this command's one
		// lifecycle lock; teardownLocked itself never reacquires it.
		return s.reconcileLocked(ctx, op, &status, providerErr, providerStatus)
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
		State: op.State, Workspace: op.Workspace, Session: op.Session, Provider: op.Provider,
		EndpointURL: op.EndpointURL, Residuals: append([]string(nil), op.Residuals...),
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

func (s *SharingService) reconcileLocked(ctx context.Context, op *SharingOperation, status *SharingStatus, providerErr error, providerStatus ProviderStatus) error {
	diagnostic := providerStatus.Diagnostic
	if providerErr != nil {
		diagnostic = providerErr.Error()
	}
	if diagnostic == "" {
		diagnostic = "provider state is " + providerStatus.State
	}
	op.State = StateDegraded
	op.Residuals = []string{"provider endpoint unhealthy: " + diagnostic}
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
	if !op.EndpointStopped {
		steps = append(steps, cleanupStep{"public endpoint may remain active", func() error {
			if err := s.provider.Stop(ctx, op.ProviderHandle); err != nil {
				return err
			}
			op.EndpointStopped, op.UpdatedAt = true, s.now().UTC()
			if err := s.store.Save(op); err != nil {
				return fmt.Errorf("endpoint stopped but cleanup progress could not be persisted: %w", err)
			}
			return nil
		}})
	}
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
	if !op.WebServerStopped {
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
