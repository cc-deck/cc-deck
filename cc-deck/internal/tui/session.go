package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cc-deck/cc-deck/internal/xdg"
)

// sessionRow is the display model for one session in the detail view.
type sessionRow struct {
	name      string
	activity  string
	branch    string
	lastEvent time.Time
	paneID    uint32
	tabIndex  *int
	paused    bool
}

// pluginSession is the Go representation of the Rust Session struct
// serialized to JSON by the Zellij plugin.
type pluginSession struct {
	PaneID          uint32          `json:"pane_id"`
	SessionID       string          `json:"session_id"`
	DisplayName     string          `json:"display_name"`
	Activity        json.RawMessage `json:"activity"`
	TabIndex        *int            `json:"tab_index"`
	TabName         *string         `json:"tab_name"`
	WorkingDir      *string         `json:"working_dir"`
	GitBranch       *string         `json:"git_branch"`
	LastEventTS     uint64          `json:"last_event_ts"`
	ManuallyRenamed bool            `json:"manually_renamed"`
	Paused          bool            `json:"paused"`
	MetaTS          uint64          `json:"meta_ts"`
	DoneAttended    bool            `json:"done_attended"`
}

// parseActivity decodes the Rust serde enum format for Activity.
// Simple variants are JSON strings: "Working", "Idle", etc.
// Waiting variant is a JSON object: {"Waiting":"Permission"}.
func parseActivity(raw json.RawMessage) string {
	// Try simple string first.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}

	// Try object form: {"Waiting":"Permission"} or {"Waiting":"Notification"}.
	var obj map[string]string
	if err := json.Unmarshal(raw, &obj); err == nil {
		if reason, ok := obj["Waiting"]; ok {
			return reason
		}
	}

	return "Unknown"
}

// pluginCachePath returns the host filesystem path to the Zellij plugin's
// WASI cache directory.
func pluginCachePath() string {
	return filepath.Join(xdg.ConfigHome, "zellij", "plugins", "cc_deck.wasm", "cache")
}

// readPluginSessions reads and parses the sessions.json file from the
// Zellij plugin's WASI cache on the host filesystem.
func readPluginSessions() ([]sessionRow, error) {
	path := filepath.Join(pluginCachePath(), "sessions.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading sessions.json: %w", err)
	}

	var sessions map[string]pluginSession
	if err := json.Unmarshal(data, &sessions); err != nil {
		return nil, fmt.Errorf("parsing sessions.json: %w", err)
	}

	var rows []sessionRow
	for _, s := range sessions {
		branch := ""
		if s.GitBranch != nil {
			branch = *s.GitBranch
		}
		rows = append(rows, sessionRow{
			name:      s.DisplayName,
			activity:  parseActivity(s.Activity),
			branch:    branch,
			lastEvent: time.Unix(int64(s.LastEventTS), 0),
			paneID:    s.PaneID,
			tabIndex:  s.TabIndex,
			paused:    s.Paused,
		})
	}

	return rows, nil
}
