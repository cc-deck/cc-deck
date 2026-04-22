package ws

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
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
		Name:      e.name,
		Type:      WorkspaceTypeLocal,
		State:     WorkspaceStateRunning,
		CreatedAt: time.Now().UTC(),
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

	// Update last_attached timestamp.
	if inst, findErr := e.store.FindInstanceByName(e.name); findErr == nil {
		now := time.Now().UTC()
		inst.LastAttached = &now
		_ = e.store.UpdateInstance(inst)
	}

	// Inside Zellij: cannot attach from within another session.
	if os.Getenv("ZELLIJ") != "" {
		fmt.Fprintf(os.Stderr, "Already inside Zellij. Detach first (Ctrl+o d), then run:\n")
		fmt.Fprintf(os.Stderr, "  cc-deck ws attach %s\n", e.name)
		return nil
	}

	// If the session doesn't exist, create it in the background with the
	// cc-deck layout. "attach --create-background" alone would use the
	// default layout, so we explicitly create with --layout first.
	if !zellijSessionExists(sessionName) {
		create := exec.Command(zellijPath, "attach", "--create-background", sessionName, "--layout", "cc-deck")
		if out, err := create.CombinedOutput(); err != nil {
			// Fallback: create without layout (--layout might not be
			// supported on attach in all Zellij versions).
			fallback := exec.Command(zellijPath, "attach", "--create-background", sessionName)
			if fout, ferr := fallback.CombinedOutput(); ferr != nil {
				return fmt.Errorf("creating session: %s\n%s", ferr, string(fout))
			}
			_ = out // suppress unused warning
		}
	}

	// Attach to the (now-existing) session.
	return syscall.Exec(zellijPath, []string{"zellij", "attach", sessionName}, os.Environ())
}

// Delete removes the workspace from the state store and kills the Zellij
// session if it exists. Without force, a running session causes an error.
func (e *LocalWorkspace) Delete(_ context.Context, force bool) error {
	sessionName := e.zellijSessionName()

	if !force {
		// Check if the Zellij session is still running.
		if zellijSessionExists(sessionName) {
			return ErrRunning
		}
	}

	// Remove from state store.
	if err := e.store.RemoveInstance(e.name); err != nil {
		return err
	}

	// Best effort: kill the Zellij session.
	_ = exec.Command("zellij", "kill-session", sessionName).Run()

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
	state := WorkspaceStateUnknown
	if zellijSessionExists(sessionName) {
		state = WorkspaceStateRunning
	}

	status := &WorkspaceStatus{
		State: state,
		Since: inst.CreatedAt,
	}

	// Best effort: read pane map for session info.
	if state == WorkspaceStateRunning {
		if sessions, readErr := readPaneMapSessions(); readErr == nil {
			status.Sessions = sessions
		}
	}

	return status, nil
}

// Start creates the Zellij session in the background with the cc-deck
// layout. The session can then be attached to with Attach.
func (e *LocalWorkspace) Start(_ context.Context) error {
	zellijPath, err := exec.LookPath("zellij")
	if err != nil {
		return ErrZellijNotFound
	}

	sessionName := e.zellijSessionName()
	if zellijSessionExists(sessionName) {
		return fmt.Errorf("session %q is already running", sessionName)
	}

	cmd := exec.Command(zellijPath, "attach", sessionName, "--create-background")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("starting session: %s\n%s", err, string(out))
	}

	return nil
}

// Stop kills the Zellij session for this workspace.
func (e *LocalWorkspace) Stop(_ context.Context) error {
	sessionName := e.zellijSessionName()
	if !zellijSessionExists(sessionName) {
		return fmt.Errorf("session %q is not running", sessionName)
	}

	cmd := exec.Command("zellij", "kill-session", sessionName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("stopping session: %s\n%s", err, string(out))
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
	return &localPipeChannel{name: e.name}, nil
}

// DataChannel returns the data channel for this workspace.
func (e *LocalWorkspace) DataChannel(_ context.Context) (DataChannel, error) {
	return &localDataChannel{name: e.name}, nil
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
	sessions := listZellijSessions()
	for _, s := range sessions {
		if s == sessionName {
			return true
		}
	}
	return false
}

// listZellijSessions runs "zellij list-sessions -n" (no formatting) and
// returns names of active (non-EXITED) sessions. Returns nil if zellij
// is not available or the command fails.
func listZellijSessions() []string {
	out, err := exec.Command("zellij", "list-sessions", "-n").Output()
	if err != nil {
		return nil
	}

	var sessions []string
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.Contains(line, "(EXITED") {
			continue
		}
		// Session name is the first field.
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
	sessions := listZellijSessions()
	sessionSet := make(map[string]bool, len(sessions))
	for _, s := range sessions {
		sessionSet[s] = true
	}

	for _, inst := range instances {
		sessionName := zellijSessionPrefix + inst.Name
		newState := WorkspaceStateUnknown
		if sessionSet[sessionName] {
			newState = WorkspaceStateRunning
		}

		if inst.State != newState {
			inst.State = newState
			if err := store.UpdateInstance(inst); err != nil {
				return err
			}
		}
	}

	return nil
}
