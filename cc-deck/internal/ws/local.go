package ws

import (
	"bufio"
	"context"
	"log"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	// zellijSessionPrefix is prepended to workspace names to form Zellij session names.
	zellijSessionPrefix = "cc-deck-"

	// paneMapPath is the location of the pane map file written by the hook.
	paneMapPath = "/tmp/cc-deck-pane-map.json"
)

// LocalWorkspace manages a local Zellij-based workspace that runs
// directly on the host machine.
type LocalWorkspace struct {
	name  string
	store *FileStateStore
	defs  *DefinitionStore

	pipeOnce sync.Once
	pipeCh   PipeChannel
	dataOnce sync.Once
	dataCh   DataChannel
}

// Type returns WorkspaceTypeLocal.
func (e *LocalWorkspace) Type() WorkspaceType {
	return WorkspaceTypeLocal
}

// Name returns the workspace name.
func (e *LocalWorkspace) Name() string {
	return e.name
}

// zellijSessionName returns the Zellij session name for this workspace.
func (e *LocalWorkspace) zellijSessionName() string {
	return zellijSessionPrefix + e.name
}

// Create provisions a new local workspace by validating the name,
// checking for the Zellij binary, and adding a record to the state store.
func (e *LocalWorkspace) Create(_ context.Context, _ CreateOpts) error {
	if err := ValidateWsName(e.name); err != nil {
		return err
	}

	// Check that zellij is available.
	if _, err := exec.LookPath("zellij"); err != nil {
		return ErrZellijNotFound
	}

	inst := &WorkspaceInstance{
		Name:         e.name,
		Type:         WorkspaceTypeLocal,
		SessionState: SessionStateNone,
		CreatedAt:    time.Now().UTC(),
	}

	return e.store.AddInstance(inst)
}

// Attach connects to the workspace's Zellij session. If the session
// does not exist, it is created in the background first (with the cc-deck
// layout), then attached. This avoids issues with "zellij --session"
// not creating new sessions when a Zellij server is already running.
func (e *LocalWorkspace) Attach(_ context.Context) error {
	zellijPath, err := exec.LookPath("zellij")
	if err != nil {
		return ErrZellijNotFound
	}

	sessionName := e.zellijSessionName()

	// Update last_attached timestamp and session state.
	if inst, findErr := e.store.FindInstanceByName(e.name); findErr == nil {
		now := time.Now().UTC()
		inst.LastAttached = &now
		inst.SessionState = SessionStateExists
		_ = e.store.UpdateInstance(inst)
	}

	// Inside Zellij: cannot attach from within another session.
	if os.Getenv("ZELLIJ") != "" {
		fmt.Fprintf(os.Stderr, "Already inside Zellij. Detach first (Ctrl+o d), then run:\n")
		fmt.Fprintf(os.Stderr, "  cc-deck ws attach %s\n", e.name)
		return nil
	}

	// Delete any EXITED session with the same name to prevent stale ghosts.
	// cc-deck manages its own session lifecycle; Zellij's serialization cache
	// only produces stale EXITED sessions that interfere with clean re-attach.
	if ZellijSessionState(sessionName) == "exited" {
		del := exec.Command(zellijPath, "delete-session", "--force", sessionName)
		_ = del.Run()
	}

	// If the session doesn't exist, create it in the background with the
	// cc-deck layout. We try --layout with attach -b first, then fall back
	// to attach -b without layout.
	if !zellijSessionExists(sessionName) {
		create := exec.Command(zellijPath, "--layout", "cc-deck", "attach", "-b", sessionName)
		if out, createErr := create.CombinedOutput(); createErr != nil {
			fallback := exec.Command(zellijPath, "attach", "-b", sessionName)
			if fout, fallbackErr := fallback.CombinedOutput(); fallbackErr != nil {
				return fmt.Errorf("creating session: %s\n%s\nlayout attempt: %s", fallbackErr, string(fout), string(out))
			}
		}
	}

	return syscall.Exec(zellijPath, []string{"zellij", "attach", sessionName}, os.Environ())
}

// Delete removes the workspace from the state store and deletes the Zellij
// session if it exists. Without force, a running session causes an error.
func (e *LocalWorkspace) Delete(ctx context.Context, force bool) error {
	sessionName := e.zellijSessionName()
	state := ZellijSessionState(sessionName)

	if state == "running" && !force {
		return ErrRunning
	}

	if err := DeleteZellijSession(sessionName, force); err != nil {
		log.Printf("WARNING: failed to delete session for %s: %v", e.name, err)
	}
	if inst, findErr := e.store.FindInstanceByName(e.name); findErr == nil {
		inst.SessionState = SessionStateNone
		_ = e.store.UpdateInstance(inst)
	}

	if err := e.store.RemoveInstance(e.name); err != nil {
		return err
	}

	if e.defs != nil {
		if err := e.defs.Remove(e.name); err != nil {
			log.Printf("WARNING: removing definition: %v", err)
		}
	}

	return nil
}

// Status returns the current state of the local workspace by checking
// for a running Zellij session and reading the pane map for session details.
func (e *LocalWorkspace) Status(_ context.Context) (*WorkspaceStatus, error) {
	inst, err := e.store.FindInstanceByName(e.name)
	if err != nil {
		return nil, err
	}

	sessionName := e.zellijSessionName()
	var sessState SessionStateValue = SessionStateNone
	if zellijSessionExists(sessionName) {
		sessState = SessionStateExists
	}

	status := &WorkspaceStatus{
		SessionState: sessState,
		Since:        &inst.CreatedAt,
	}

	if sessState == SessionStateExists {
		if sessions, readErr := readPaneMapSessions(); readErr == nil {
			status.Sessions = sessions
		}
	}

	return status, nil
}

// KillSession kills the Zellij session for this workspace without
// affecting any infrastructure (local workspaces have none).
// Handles both running and exited (resurrectable) sessions.
func (e *LocalWorkspace) KillSession(_ context.Context) error {
	sessionName := e.zellijSessionName()
	state := ZellijSessionState(sessionName)
	if state == "" {
		return nil
	}

	cmd := exec.Command("zellij", "delete-session", "--force", sessionName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("killing session: %s\n%s", err, string(out))
	}

	if inst, findErr := e.store.FindInstanceByName(e.name); findErr == nil {
		inst.SessionState = SessionStateNone
		_ = e.store.UpdateInstance(inst)
	}

	return nil
}

// DeleteZellijSession removes a Zellij session completely.
// With force=true, kills a running session before deleting.
// Returns nil if the session does not exist.
func DeleteZellijSession(sessionName string, force bool) error {
	state := ZellijSessionState(sessionName)
	if state == "" {
		return nil
	}

	args := []string{"delete-session", sessionName}
	if force {
		args = []string{"delete-session", "--force", sessionName}
	}
	cmd := exec.Command("zellij", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("deleting session: %s\n%s", err, string(out))
	}

	return nil
}

// Exec is not supported for local workspaces.
func (e *LocalWorkspace) Exec(_ context.Context, _ []string) error {
	return fmt.Errorf("local workspaces: %w", ErrNotSupported)
}

// ExecOutput is not supported for local workspaces.
func (e *LocalWorkspace) ExecOutput(_ context.Context, _ []string) (string, error) {
	return "", fmt.Errorf("local workspaces: %w", ErrNotSupported)
}

// PipeChannel returns the pipe channel for this workspace.
func (e *LocalWorkspace) PipeChannel(_ context.Context) (PipeChannel, error) {
	e.pipeOnce.Do(func() {
		e.pipeCh = &localPipeChannel{name: e.name}
	})
	return e.pipeCh, nil
}

// DataChannel returns the data channel for this workspace.
func (e *LocalWorkspace) DataChannel(_ context.Context) (DataChannel, error) {
	e.dataOnce.Do(func() {
		e.dataCh = &localDataChannel{name: e.name}
	})
	return e.dataCh, nil
}

// GitChannel is not supported for local workspaces.
func (e *LocalWorkspace) GitChannel(_ context.Context) (GitChannel, error) {
	return nil, fmt.Errorf("local workspaces git channel: %w", ErrNotSupported)
}

// Push copies local files to the target path via DataChannel.
func (e *LocalWorkspace) Push(ctx context.Context, opts SyncOpts) error {
	ch, err := e.DataChannel(ctx)
	if err != nil {
		return err
	}
	return ch.Push(ctx, opts)
}

// Pull copies files from the source path to the target via DataChannel.
func (e *LocalWorkspace) Pull(ctx context.Context, opts SyncOpts) error {
	ch, err := e.DataChannel(ctx)
	if err != nil {
		return err
	}
	return ch.Pull(ctx, opts)
}

// Harvest is not supported for local workspaces.
func (e *LocalWorkspace) Harvest(_ context.Context, _ HarvestOpts) error {
	return fmt.Errorf("local workspaces: %w", ErrNotSupported)
}

// zellijSessionExists checks whether a Zellij session with the given name
// is present in the output of "zellij list-sessions".
func zellijSessionExists(sessionName string) bool {
	sessions := listZellijSessions(false)
	for _, s := range sessions {
		if s == sessionName {
			return true
		}
	}
	return false
}

// ZellijSessionName returns the Zellij session name for a workspace name.
func ZellijSessionName(name string) string {
	return zellijSessionPrefix + name
}

// ZellijSessionState returns "running", "exited", or "" for a session.
func ZellijSessionState(sessionName string) string {
	out, err := exec.Command("zellij", "list-sessions", "-n").Output()
	if err != nil {
		return ""
	}
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] != sessionName {
			continue
		}
		if strings.Contains(line, "(EXITED") {
			return "exited"
		}
		return "running"
	}
	return ""
}

// listZellijSessions runs "zellij list-sessions -n" (no formatting) and
// returns session names. When includeExited is false, EXITED sessions are
// skipped. Returns nil if zellij is not available or the command fails.
func listZellijSessions(includeExited bool) []string {
	out, err := exec.Command("zellij", "list-sessions", "-n").Output()
	if err != nil {
		return nil
	}

	var sessions []string
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if !includeExited && strings.Contains(line, "(EXITED") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) > 0 {
			sessions = append(sessions, fields[0])
		}
	}
	return sessions
}

// paneMapEntry represents a single entry in the pane map JSON file.
type paneMapEntry struct {
	PaneID        int    `json:"pane_id"`
	HookEventName string `json:"hook_event_name"`
	CWD           string `json:"cwd"`
	ToolName      string `json:"tool_name"`
}

// readPaneMapSessions reads the pane map JSON file and converts it to
// SessionInfo entries. Returns an error if the file is missing or cannot
// be parsed.
func readPaneMapSessions() ([]SessionInfo, error) {
	data, err := os.ReadFile(paneMapPath)
	if err != nil {
		return nil, err
	}

	var paneMap map[string]paneMapEntry
	if err := json.Unmarshal(data, &paneMap); err != nil {
		return nil, err
	}

	var sessions []SessionInfo
	for id, entry := range paneMap {
		sessions = append(sessions, SessionInfo{
			Name:     id,
			Activity: entry.HookEventName,
			Branch:   "", // Not available from pane map.
		})
	}

	return sessions, nil
}

// ReconcileLocalWorkspaces updates the state of all local workspaces by checking
// which Zellij sessions are actually running. Sessions found in the Zellij
// session list are marked as Running; those not found are marked as Unknown.
func ReconcileLocalWorkspaces(store *FileStateStore) error {
	localType := WorkspaceTypeLocal
	instances, err := store.ListInstances(&ListFilter{Type: &localType})
	if err != nil {
		return err
	}

	if len(instances) == 0 {
		return nil
	}

	// Get all running Zellij sessions once.
	sessions := listZellijSessions(false)
	sessionSet := make(map[string]bool, len(sessions))
	for _, s := range sessions {
		sessionSet[s] = true
	}

	for _, inst := range instances {
		sessionName := zellijSessionPrefix + inst.Name
		var newSessionState SessionStateValue = SessionStateNone
		if sessionSet[sessionName] {
			newSessionState = SessionStateExists
		}

		if inst.SessionState != newSessionState {
			inst.SessionState = newSessionState
			if err := store.UpdateInstance(inst); err != nil {
				return err
			}
		}
	}

	return nil
}
