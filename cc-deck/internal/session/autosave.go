package session

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

const (
	autoSaveName     = "auto"
	autoSaveCooldown = 5 * time.Minute
)

// autoSaveTier defines a retention tier for auto-snapshots.
// Each tier keeps one snapshot that is updated when its age threshold is exceeded.
type autoSaveTier struct {
	name      string
	threshold time.Duration
}

// autoSaveTiers defines the retention tiers from shortest to longest.
// Each tier keeps a single snapshot, promoted from "auto" when its threshold elapses.
var autoSaveTiers = []autoSaveTier{
	{"auto-hourly", 1 * time.Hour},
	{"auto-12h", 12 * time.Hour},
	{"auto-daily", 24 * time.Hour},
	{"auto-weekly", 7 * 24 * time.Hour},
}

// AutoSave checks the cooldown and, if elapsed, spawns a detached background
// process to perform the actual save. This avoids blocking the caller (hook
// subprocess) on the blocking zellij pipe query.
func AutoSave() {
	if !CooldownElapsed() {
		return
	}

	// Spawn cc-deck snapshot save --auto as a detached background process.
	// The binary path is resolved from $PATH or the current executable.
	binPath, err := os.Executable()
	if err != nil {
		return
	}

	// Never re-execute a test binary. A Go test binary silently ignores the
	// positional arguments below and re-runs its whole suite instead, so every
	// test that reaches AutoSave would spawn another full suite run, which
	// spawns another, and so on. The children are detached and never reaped.
	if isTestBinary(binPath) {
		return
	}

	startDetached(binPath, "snapshot", "save", "--auto")
}

// startDetached runs binPath with args as a background process that outlives
// the caller. It is a variable so tests can observe the call without spawning
// a real process.
var startDetached = func(binPath string, args ...string) {
	cmd := exec.Command(binPath, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	_ = cmd.Start()
	// Don't wait - let it run independently of the hook subprocess.
	if cmd.Process != nil {
		_ = cmd.Process.Release()
	}
}

// isTestBinary reports whether re-executing binPath would run a Go test
// binary rather than the cc-deck CLI.
func isTestBinary(binPath string) bool {
	return testing.Testing() || isTestBinaryPath(binPath)
}

// isTestBinaryPath reports whether binPath names a compiled Go test binary.
// This covers the case of a test binary built with "go test -c" and run
// directly, where testing.Testing() is false until the suite starts.
func isTestBinaryPath(binPath string) bool {
	base := filepath.Base(binPath)
	return strings.HasSuffix(base, ".test") || strings.HasSuffix(base, ".test.exe")
}

// RunAutoSave performs the actual auto-save: queries plugin state and
// overwrites the single "auto" snapshot. Called by "cc-deck snapshot save --auto".
// Uses a file lock to ensure only one auto-save runs at a time.
func RunAutoSave() error {
	// Acquire exclusive lock to prevent concurrent auto-saves.
	lockPath := filepath.Join(SessionsDir(), ".autosave.lock")
	if err := os.MkdirAll(SessionsDir(), 0o755); err != nil {
		return err
	}
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer lockFile.Close()
	defer os.Remove(lockPath)

	// Non-blocking exclusive lock: if another auto-save is running, exit.
	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return nil // another auto-save is in progress
	}
	defer syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)

	// Re-check cooldown under lock (another process may have saved).
	if !CooldownElapsed() {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	snap, err := QueryPluginStateCtx(ctx, autoSaveName)
	if err != nil {
		return err
	}

	if err := SaveSnapshot(snap); err != nil {
		return err
	}

	rotateAutoSnapshots(snap)

	return nil
}

// CooldownElapsed checks if enough time has passed since the last auto-save.
func CooldownElapsed() bool {
	info, err := os.Stat(snapshotPath(autoSaveName))
	if err != nil {
		return true // no previous auto-save = proceed
	}
	return time.Since(info.ModTime()) >= autoSaveCooldown
}

// rotateAutoSnapshots promotes the current snapshot data to tiered retention slots.
// Each tier keeps a single snapshot that is overwritten when its age threshold elapses.
func rotateAutoSnapshots(snap *Snapshot) {
	for _, tier := range autoSaveTiers {
		info, err := os.Stat(snapshotPath(tier.name))
		if err != nil || time.Since(info.ModTime()) >= tier.threshold {
			tierSnap := *snap
			tierSnap.Name = tier.name
			_ = SaveSnapshot(&tierSnap)
		}
	}
}
