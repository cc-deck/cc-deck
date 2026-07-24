package share

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const guardReadyTimeout = 3 * time.Second

// DetachedGuard launches the hidden guard command and waits for the child to
// confirm that it validated the persisted operation identity.
type DetachedGuard struct {
	runner     CommandRunner
	executable string
	timeout    time.Duration
	poll       time.Duration
	mu         sync.Mutex
	processes  map[int]Process
}

func NewDetachedGuard(runner CommandRunner, executable string) *DetachedGuard {
	return &DetachedGuard{runner: runner, executable: executable, timeout: guardReadyTimeout, poll: 20 * time.Millisecond, processes: map[int]Process{}}
}

func (g *DetachedGuard) Start(ctx context.Context, operationID string) (GuardHandle, error) {
	if operationID == "" {
		return GuardHandle{}, fmt.Errorf("guard operation ID must not be empty")
	}
	readyPath := filepath.Join(os.TempDir(), "cc-deck-guard-"+operationID+".ready")
	_ = os.Remove(readyPath)
	process, err := g.runner.Start(ctx, g.executable, "share", "guard", "--operation", operationID, "--ready-file", readyPath)
	if err != nil {
		return GuardHandle{}, fmt.Errorf("launch detached sharing guard: %w", err)
	}
	g.mu.Lock()
	g.processes[process.PID()] = process
	g.mu.Unlock()
	fingerprint, err := g.processFingerprint(ctx, process.PID())
	if err != nil {
		_ = process.Kill()
		return GuardHandle{}, fmt.Errorf("capture sharing guard process identity: %w", err)
	}
	if !strings.Contains(fingerprint, filepath.Base(g.executable)) || !strings.Contains(fingerprint, "--operation "+operationID) {
		_ = process.Kill()
		return GuardHandle{}, fmt.Errorf("sharing guard process identity does not contain the expected executable and operation marker")
	}

	deadline := time.NewTimer(g.timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(g.poll)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			_ = process.Kill()
			return GuardHandle{}, ctx.Err()
		case <-deadline.C:
			_ = process.Kill()
			return GuardHandle{}, fmt.Errorf("sharing guard readiness timeout")
		case <-ticker.C:
			contents, readErr := os.ReadFile(readyPath)
			if readErr == nil && string(contents) == operationID {
				_ = os.Remove(readyPath)
				return GuardHandle{PID: process.PID(), OperationID: operationID, Ready: true, ProcessFingerprint: fingerprint}, nil
			}
		}
	}
}

// Disarm only signals the guard. It deliberately does not wait: normal stop may
// hold the lifecycle lock while the guard is about to request that same lock.
func (g *DetachedGuard) Disarm(_ context.Context, handle GuardHandle) error {
	current, err := g.processFingerprint(context.Background(), handle.PID)
	if err != nil {
		if errors.Is(err, errGuardProcessGone) {
			return nil
		}
		return fmt.Errorf("validate sharing guard PID %d: %w", handle.PID, err)
	}
	if handle.ProcessFingerprint == "" || current != handle.ProcessFingerprint ||
		!strings.Contains(current, filepath.Base(g.executable)) || !strings.Contains(current, "--operation "+handle.OperationID) {
		return fmt.Errorf("refusing to signal PID %d: sharing guard process identity mismatch", handle.PID)
	}
	g.mu.Lock()
	process := g.processes[handle.PID]
	delete(g.processes, handle.PID)
	g.mu.Unlock()
	if process == nil {
		found, err := os.FindProcess(handle.PID)
		if err != nil {
			return err
		}
		process = &osProcessAdapter{Process: found}
	}
	if err := process.Signal(os.Interrupt); err != nil && !isProcessGone(err) {
		return fmt.Errorf("signal sharing guard PID %s: %w", strconv.Itoa(handle.PID), err)
	}
	return nil
}

var errGuardProcessGone = errors.New("sharing guard process is gone")

func (g *DetachedGuard) processFingerprint(ctx context.Context, pid int) (string, error) {
	out, err := g.runner.Run(ctx, "ps", "-p", strconv.Itoa(pid), "-o", "lstart=,comm=,command=")
	fingerprint := strings.TrimSpace(string(out))
	if fingerprint == "" {
		if err != nil || len(out) == 0 {
			return "", errGuardProcessGone
		}
	}
	if err != nil {
		return "", err
	}
	return fingerprint, nil
}

// RunGuard is the hidden child process lifecycle. Identity validation occurs
// under one short lock. Provider watching and Service.Stop happen after release,
// so the stop path acquires the lifecycle lock exactly once.
func RunGuard(ctx context.Context, store Store, provider Provider, service Service, operationID, readyPath string) error {
	var handle ProviderHandle
	if err := store.WithLock(ctx, func() error {
		op, err := store.Load()
		if err != nil {
			return err
		}
		if op == nil || op.ID != operationID || op.State != StateActive {
			return fmt.Errorf("sharing operation %q is no longer active", operationID)
		}
		handle = op.ProviderHandle
		return nil
	}); err != nil {
		return err
	}
	if err := os.WriteFile(readyPath, []byte(operationID), 0600); err != nil {
		return fmt.Errorf("signal sharing guard readiness: %w", err)
	}

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			_, err := service.Stop(context.Background())
			return err
		case <-ticker.C:
			status, err := provider.Status(context.Background(), handle)
			if err != nil || (status.State != "ready" && status.State != "starting") {
				_, stopErr := service.Stop(context.Background())
				return stopErr
			}
			sharingStatus, statusErr := service.Status(context.Background())
			if statusErr != nil || sharingStatus.State != StateActive {
				return statusErr
			}
		}
	}
}
