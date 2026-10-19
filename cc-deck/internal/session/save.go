package session

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// pluginSession mirrors the Rust Session struct serialized by the plugin.
type pluginSession struct {
	PaneID          uint32  `json:"pane_id"`
	SessionID       string  `json:"session_id"`
	DisplayName     string  `json:"display_name"`
	TabIndex        *int    `json:"tab_index"`
	TabName         *string `json:"tab_name"`
	WorkingDir      *string `json:"working_dir"`
	GitBranch       *string `json:"git_branch"`
	AgentName       *string `json:"agent_name"`
	Profile         *string `json:"profile"`
	ManuallyRenamed bool    `json:"manually_renamed"`
	Paused          bool    `json:"paused"`
	Background      bool    `json:"background"`
}

// QueryPluginState queries the cc-deck plugin for current session state
// via zellij pipe and returns a Snapshot. It targets the ambient Zellij
// session (the one this process runs in).
func QueryPluginState(name string) (*Snapshot, error) {
	return QueryPluginStateCtx(context.Background(), name, "")
}

// QueryPluginStateCtx is like QueryPluginState but with a context for timeout
// and an explicit target Zellij session. An empty targetSession queries the
// ambient session; a non-empty one queries that named session, which is how a
// daemon-hosted hook (no ambient Zellij) snapshots the session it delivered to.
func QueryPluginStateCtx(ctx context.Context, name, targetSession string) (*Snapshot, error) {
	raw, err := queryPluginCtx(ctx, targetSession)
	if err != nil {
		return nil, err
	}

	// Parse the plugin's dump-state response. The controller wraps sessions
	// in an envelope with attended_pane_id; try that first, fall back to
	// the flat map format from sidebar instances.
	sessions, err := parseDumpState(raw)
	if err != nil {
		return nil, fmt.Errorf("parsing plugin state: %w", err)
	}

	entries := buildEntries(sessions)

	if name == "" {
		name = nextSnapshotName()
	}

	return &Snapshot{
		Version:  1,
		Name:     name,
		SavedAt:  time.Now().UTC(),
		Sessions: entries,
	}, nil
}

// buildEntries converts the plugin's session map into SessionEntry values
// sorted by tab index, resolving working-dir symlinks. It is the pure mapping
// step of QueryPluginStateCtx, split out so it can be tested without a plugin.
func buildEntries(sessions map[string]pluginSession) []SessionEntry {
	var entries []SessionEntry
	for _, s := range sessions {
		entry := SessionEntry{
			SessionID:   s.SessionID,
			DisplayName: s.DisplayName,
			Paused:      s.Paused,
			Background:  s.Background,
		}
		if s.TabName != nil {
			entry.TabName = *s.TabName
		}
		if s.WorkingDir != nil {
			if resolved, err := filepath.EvalSymlinks(*s.WorkingDir); err == nil {
				entry.WorkingDir = resolved
			} else {
				entry.WorkingDir = *s.WorkingDir
			}
		}
		if s.GitBranch != nil {
			entry.GitBranch = *s.GitBranch
		}
		if s.AgentName != nil {
			entry.Agent = *s.AgentName
		}
		if s.Profile != nil {
			entry.Profile = *s.Profile
		}
		if s.TabIndex != nil {
			entry.tabIndex = *s.TabIndex
		}
		entries = append(entries, entry)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].tabIndex < entries[j].tabIndex
	})
	return entries
}

func parseDumpState(raw []byte) (map[string]pluginSession, error) {
	// Try envelope format: {"sessions": {...}, "attended_pane_id": ...}
	// Use Decoder to handle concatenated JSON from Zellij broadcast pipes
	// where the controller responds multiple times.
	var envelope struct {
		Sessions map[string]pluginSession `json:"sessions"`
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	if err := dec.Decode(&envelope); err == nil && envelope.Sessions != nil {
		return envelope.Sessions, nil
	}
	// Fall back to flat map format
	var sessions map[string]pluginSession
	dec2 := json.NewDecoder(strings.NewReader(string(raw)))
	if err := dec2.Decode(&sessions); err != nil {
		return nil, err
	}
	return sessions, nil
}

// queryPluginCtx runs zellij pipe to get session state JSON from the controller plugin.
// Uses broadcast delivery so the leader controller responds regardless of
// which instance Zellij routes targeted pipes to (duplicate instance bug).
// An empty targetSession pipes to the ambient Zellij session; a non-empty one
// pipes to that named session via "zellij --session <name> pipe".
func queryPluginCtx(ctx context.Context, targetSession string) ([]byte, error) {
	var args []string
	if targetSession != "" {
		args = append(args, "--session", targetSession)
	}
	args = append(args, "pipe", "--name", "cc-deck:dump-state", "--", "")
	cmd := exec.CommandContext(ctx, "zellij", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("querying plugin state: %w", err)
	}
	return out, nil
}
