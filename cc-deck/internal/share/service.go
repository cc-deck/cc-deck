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
}

const rollbackTimeout = 5 * time.Second

func NewService(store Store, zellij Zellij, provider Provider) *SharingService {
	return &SharingService{store: store, zellij: zellij, provider: provider, now: time.Now}
}

func (s *SharingService) Start(ctx context.Context, req StartRequest) (InvitationSet, error) {
	var invitations InvitationSet
	err := s.store.WithLock(ctx, func() error {
		existing, err := s.store.Load()
		if err != nil {
			return err
		}
		if existing != nil {
			if existing.State == StateActive && (req.Session == "" || req.Session == existing.Session) &&
				(req.Provider == "" || req.Provider == existing.Provider) {
				invitations = InvitationSet{Warnings: []string{fmt.Sprintf("Sharing is already active for session %q with provider %s; existing credentials are not redisplayed.", existing.Session, existing.Provider)}}
				return nil
			}
			return fmt.Errorf("sharing operation already exists for session %q; run cc-deck share status or stop first", existing.Session)
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
		rollback := func(cause error) error {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), rollbackTimeout)
			defer cancel()
			var residuals []string
			for i := len(undo) - 1; i >= 0; i-- {
				if rollbackErr := undo[i](cleanupCtx); rollbackErr != nil {
					residuals = append(residuals, rollbackErr.Error())
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

func (s *SharingService) Status(context.Context) (SharingStatus, error) {
	return SharingStatus{}, fmt.Errorf("share status is not implemented")
}

func (s *SharingService) Stop(context.Context) (SharingStatus, error) {
	return SharingStatus{}, fmt.Errorf("share stop is not implemented")
}
