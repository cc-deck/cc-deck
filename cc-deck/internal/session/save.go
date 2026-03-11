package session

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
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
	ManuallyRenamed bool    `json:"manually_renamed"`
	Paused          bool    `json:"paused"`
}

// QueryPluginState queries the cc-deck plugin for current session state
// via zellij pipe and returns a Snapshot.
func QueryPluginState(name string) (*Snapshot, error) {
	return QueryPluginStateCtx(context.Background(), name)
}

// QueryPluginStateCtx is like QueryPluginState but with a context for timeout.
func QueryPluginStateCtx(ctx context.Context, name string) (*Snapshot, error) {
	raw, err := queryPluginCtx(ctx)
	if err != nil {
		return nil, err
	}

	// Parse the plugin's BTreeMap<u32, Session> JSON
	var sessions map[string]pluginSession
	if err := json.Unmarshal(raw, &sessions); err != nil {
		return nil, fmt.Errorf("parsing plugin state: %w", err)
	}

	// Convert to SessionEntry list sorted by tab index
	var entries []SessionEntry
	for _, s := range sessions {
		entry := SessionEntry{
			SessionID:   s.SessionID,
			DisplayName: s.DisplayName,
			Paused:      s.Paused,
		}
		if s.TabName != nil {
			entry.TabName = *s.TabName
		}
		if s.WorkingDir != nil {
			entry.WorkingDir = *s.WorkingDir
		}
		if s.GitBranch != nil {
			entry.GitBranch = *s.GitBranch
		}
		if s.TabIndex != nil {
			entry.tabIndex = *s.TabIndex
		}
		entries = append(entries, entry)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].tabIndex < entries[j].tabIndex
	})

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

// queryPluginCtx runs zellij pipe to get session state JSON from the plugin.
func queryPluginCtx(ctx context.Context) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "zellij", "pipe", "--name", "cc-deck:dump-state", "--", "")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("querying plugin state: %w", err)
	}
	return out, nil
}
