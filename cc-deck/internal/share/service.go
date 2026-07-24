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
	guard    Guard
	now      func() time.Time
}

const rollbackTimeout = 5 * time.Second

func NewService(store Store, zellij Zellij, provider Provider) *SharingService {
	return &SharingService{store: store, zellij: zellij, provider: provider, now: time.Now}
}

func NewServiceWithGuard(store Store, zellij Zellij, provider Provider, guard Guard) *SharingService {
	service := NewService(store, zellij, provider)
	service.guard = guard
	return service
}

func (s *SharingService) Start(ctx context.Context, req StartRequest) (InvitationSet, error) {
	var invitations InvitationSet
	err := s.store.WithLock(ctx, func() error {
		existing, err := s.store.Load()
		if err != nil {
			return err
		}
		if existing != nil {
			providerStatus, statusErr := s.provider.Status(ctx, existing.ProviderHandle)
			if existing.State == StateActive && statusErr == nil &&
				(providerStatus.State == "ready" || providerStatus.State == "starting") {
				if (req.Session == "" || req.Session == existing.Session) &&
					(req.Provider == "" || req.Provider == existing.Provider) {
					invitations = InvitationSet{Warnings: []string{fmt.Sprintf("Sharing is already active for session %q with provider %s; existing credentials are not redisplayed.", existing.Session, existing.Provider)}}
					return nil
				}
				return fmt.Errorf("sharing operation already exists for session %q; run cc-deck share status or stop first", existing.Session)
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
		session, err := s.zellij.ResolveSession(ctx, req.Session)
		if err != nil {
			return err
		}

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
		op := &SharingOperation{
			ID: id, Session: session, Provider: providerName,
			InteractiveTokenLabel: "cc-deck-" + id + "-interactive",
			ObserverTokenLabel:    "cc-deck-" + id + "-observer",
			State:                 StateStarting, CreatedAt: now, UpdatedAt: now,
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

		if err = s.zellij.ShareSession(ctx, session); err != nil {
			return err
		}
		undo = append(undo, func(cleanupCtx context.Context) error { return s.zellij.UnshareSession(cleanupCtx, session) })
		localURL, webStarted, err := s.zellij.EnsureWebServer(ctx)
		if err != nil {
			return rollback(err)
		}
		if webStarted {
			undo = append(undo, func(cleanupCtx context.Context) error { return s.zellij.StopWebServer(cleanupCtx) })
		}
		interactiveToken, err := s.zellij.CreateToken(ctx, op.InteractiveTokenLabel, false)
		if err != nil {
			return rollback(err)
		}
		undo = append(undo, func(cleanupCtx context.Context) error {
			return s.zellij.RevokeToken(cleanupCtx, op.InteractiveTokenLabel)
		})
		observerToken, err := s.zellij.CreateToken(ctx, op.ObserverTokenLabel, true)
		if err != nil {
			return rollback(err)
		}
		undo = append(undo, func(cleanupCtx context.Context) error { return s.zellij.RevokeToken(cleanupCtx, op.ObserverTokenLabel) })
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
		invitations, err = BuildInvitations(ready.EndpointURL, session, interactiveToken, observerToken)
		if err != nil {
			return rollback(err)
		}
		if err = s.store.Save(op); err != nil {
			return rollback(err)
		}
		persisted = true
		if s.guard != nil {
			guardHandle, guardErr := s.guard.Start(ctx, op.ID)
			if guardErr != nil {
				return rollback(fmt.Errorf("start sharing lifecycle guard: %w", guardErr))
			}
			op.Guard = guardHandle
			if err = s.store.Save(op); err != nil {
				_ = s.guard.Disarm(context.Background(), guardHandle)
				return rollback(err)
			}
		}
		return nil
	})
	if err != nil {
		return InvitationSet{}, err
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

func (s *SharingService) Stop(ctx context.Context) (SharingStatus, error) {
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
		return s.teardownLocked(ctx, op, &status)
	})
	return status, err
}

func statusFromOperation(op *SharingOperation) SharingStatus {
	if op == nil {
		return SharingStatus{State: StateInactive}
	}
	return SharingStatus{
		State: op.State, Session: op.Session, Provider: op.Provider,
		EndpointURL: op.EndpointURL, InteractiveAvailable: op.State == StateActive,
		ObserverAvailable: op.State == StateActive,
		Residuals:         append([]string(nil), op.Residuals...),
	}
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
	if s.guard != nil && op.Guard.PID > 0 {
		if err := s.guard.Disarm(ctx, op.Guard); err != nil {
			residuals = append(residuals, "lifecycle guard could not be disarmed: "+err.Error())
		}
	}

	type cleanupStep struct {
		residual string
		run      func() error
	}
	steps := []cleanupStep{
		{"public endpoint may remain active", func() error { return s.provider.Stop(ctx, op.ProviderHandle) }},
		{"observer credential may remain active", func() error { return s.zellij.RevokeToken(ctx, op.ObserverTokenLabel) }},
		{"interactive credential may remain active", func() error { return s.zellij.RevokeToken(ctx, op.InteractiveTokenLabel) }},
		{"selected session may remain shared", func() error { return s.zellij.UnshareSession(ctx, op.Session) }},
		{"remote clients may remain connected", func() error { return s.zellij.StopWebServer(ctx) }},
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
