package session

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

const (
	autoSaveName     = "auto"
	autoSaveCooldown = 5 * time.Minute
)

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
	cmd := exec.Command(binPath, "snapshot", "save", "--auto")
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	_ = cmd.Start()
	// Don't wait - let it run independently of the hook subprocess.
	if cmd.Process != nil {
		_ = cmd.Process.Release()
	}
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
