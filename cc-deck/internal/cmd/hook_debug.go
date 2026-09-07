package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Hook diagnostics are opt-in: they exist to answer whether the Zellij
// environment actually reaches a hook subprocess. The pane-map fallback in
// runHook was added in March 2026 because Claude Code was observed stripping
// ZELLIJ env vars from hook subprocesses roughly 93% of the time. That figure
// was never re-measured, and the fallback is what lets agents running outside
// Zellij reach the sidebar. Enable the flag file to re-measure it.
//
// Enable:   touch ~/.local/state/cc-deck/hook-debug
// Read:     ~/.local/state/cc-deck/hook.log
var (
	hookDebugFlagFile = filepath.Join(hookStateDir, "hook-debug")
	hookDebugLogFile  = filepath.Join(hookStateDir, "hook.log")

	hookDebugOnce sync.Once
	hookDebugOn   bool
)

// hookDebugEnabled reports whether the diagnostic flag file exists.
// Checked once per process; a hook invocation is short-lived.
func hookDebugEnabled() bool {
	hookDebugOnce.Do(func() {
		_, err := os.Stat(hookDebugFlagFile)
		hookDebugOn = err == nil
	})
	return hookDebugOn
}

// hookDebugf appends one line to the hook diagnostic log.
//
// It never writes to stdout or stderr. A hook that prints to either can
// disrupt the agent that invoked it, so every failure here is swallowed.
func hookDebugf(format string, args ...any) {
	if !hookDebugEnabled() {
		return
	}
	if err := os.MkdirAll(hookStateDir, 0700); err != nil {
		return
	}
	f, err := os.OpenFile(hookDebugLogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()
	line := fmt.Sprintf(format, args...)
	_, _ = fmt.Fprintf(f, "%s %s\n", time.Now().Format(time.RFC3339), line)
}

// logHookEnv records whether the Zellij environment survived into this hook
// process, alongside how the pane id was resolved.
func logHookEnv(event, paneIDArg, resolution string) {
	hookDebugf(
		"event=%s resolution=%s pane_id_arg=%q ZELLIJ=%q ZELLIJ_SESSION_NAME=%q ZELLIJ_PANE_ID=%q",
		event, resolution, paneIDArg,
		os.Getenv("ZELLIJ"),
		os.Getenv("ZELLIJ_SESSION_NAME"),
		os.Getenv("ZELLIJ_PANE_ID"),
	)
}
