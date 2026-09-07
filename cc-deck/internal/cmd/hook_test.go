package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cc-deck/cc-deck/internal/agent"
)

// hookTestEnv points the pane-map at a temp dir and captures what would have
// been piped to the plugin, so a test can assert on delivery without zellij.
type hookTestEnv struct {
	t    *testing.T
	sent [][]byte
}

func newHookTestEnv(t *testing.T) *hookTestEnv {
	t.Helper()

	dir := t.TempDir()
	origStateDir, origMapFile := hookStateDir, paneMapFile
	origSend := sendHookPayload

	hookStateDir = dir
	paneMapFile = filepath.Join(dir, "pane-map.json")

	env := &hookTestEnv{t: t}
	sendHookPayload = func(_ string, payload []byte) {
		env.sent = append(env.sent, payload)
	}

	t.Cleanup(func() {
		hookStateDir, paneMapFile = origStateDir, origMapFile
		sendHookPayload = origSend
	})

	// runHook resolves zellij from PATH before doing anything else.
	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skip("no /bin/sh to stand in for zellij")
	}
	t.Setenv("PATH", "/bin:/usr/bin")
	stub := filepath.Join(dir, "bin")
	if err := os.MkdirAll(stub, 0o755); err != nil {
		t.Fatalf("mkdir stub: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stub, "zellij"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write zellij stub: %v", err)
	}
	t.Setenv("PATH", stub+":/bin:/usr/bin")

	return env
}

func (e *hookTestEnv) inZellij(session string) {
	e.t.Helper()
	e.t.Setenv("ZELLIJ", "0")
	e.t.Setenv("ZELLIJ_SESSION_NAME", session)
}

func (e *hookTestEnv) outsideZellij() {
	e.t.Helper()
	e.t.Setenv("ZELLIJ", "")
	e.t.Setenv("ZELLIJ_SESSION_NAME", "")
}

func (e *hookTestEnv) writeMap(m map[string]paneMapEntry) {
	e.t.Helper()
	data, err := json.Marshal(m)
	if err != nil {
		e.t.Fatalf("marshal pane map: %v", err)
	}
	if err := os.WriteFile(paneMapFile, data, 0o600); err != nil {
		e.t.Fatalf("write pane map: %v", err)
	}
}

func (e *hookTestEnv) readMap() map[string]paneMapEntry {
	e.t.Helper()
	return loadPaneMap()
}

func claudeEvent(event, sessionID string) string {
	payload := map[string]string{
		"hook_event_name": event,
		"session_id":      sessionID,
		"cwd":             "/tmp",
	}
	data, _ := json.Marshal(payload)
	return string(data)
}

// The live bug: an agent resumed outside Zellij reuses its session id, hits a
// cached pane id from when it last ran inside Zellij, and its events reach the
// sidebar of whatever session happens to be running.
func TestRunHook_OutsideZellijSendsNothingDespiteCachedPane(t *testing.T) {
	env := newHookTestEnv(t)
	env.writeMap(map[string]paneMapEntry{
		"resumed-session": {PaneID: 3, ZellijSession: "cc-deck-local", UpdatedAt: time.Now().Unix()},
	})
	env.outsideZellij()

	runHook(strings.NewReader(claudeEvent("PreToolUse", "resumed-session")), "", "claude")

	if len(env.sent) != 0 {
		t.Fatalf("expected no payload from outside Zellij, sent %d: %s", len(env.sent), env.sent[0])
	}
}

func TestRunHook_InsideZellijStampsSessionAndPane(t *testing.T) {
	env := newHookTestEnv(t)
	env.inZellij("cc-deck-local")

	runHook(strings.NewReader(claudeEvent("PreToolUse", "sess-a")), "7", "claude")

	if len(env.sent) != 1 {
		t.Fatalf("expected exactly one payload, got %d", len(env.sent))
	}
	var got agent.NormalizedPayload
	if err := json.Unmarshal(env.sent[0], &got); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if got.PaneID != 7 {
		t.Errorf("pane id: got %d, want 7", got.PaneID)
	}

	entry, ok := env.readMap()["sess-a"]
	if !ok {
		t.Fatal("expected a cache entry for sess-a")
	}
	if entry.PaneID != 7 {
		t.Errorf("cached pane id: got %d, want 7", entry.PaneID)
	}
	if entry.ZellijSession != "cc-deck-local" {
		t.Errorf("cached zellij session: got %q, want %q", entry.ZellijSession, "cc-deck-local")
	}
	if entry.UpdatedAt == 0 {
		t.Error("expected the entry to carry a timestamp")
	}
}

// Pane ids restart near zero in every Zellij run, so a mapping recorded by a
// different session addresses an unrelated pane.
func TestRunHook_CacheFromDifferentZellijSessionIsNotReused(t *testing.T) {
	env := newHookTestEnv(t)
	env.writeMap(map[string]paneMapEntry{
		"sess-b": {PaneID: 3, ZellijSession: "an-older-session", UpdatedAt: time.Now().Unix()},
	})
	env.inZellij("cc-deck-local")

	runHook(strings.NewReader(claudeEvent("PreToolUse", "sess-b")), "", "claude")

	if len(env.sent) != 0 {
		t.Fatalf("expected no payload for a foreign cache entry, sent %d", len(env.sent))
	}
}

func TestRunHook_CacheFromSameZellijSessionIsReused(t *testing.T) {
	env := newHookTestEnv(t)
	env.writeMap(map[string]paneMapEntry{
		"sess-c": {PaneID: 5, ZellijSession: "cc-deck-local", UpdatedAt: time.Now().Unix()},
	})
	env.inZellij("cc-deck-local")

	runHook(strings.NewReader(claudeEvent("PreToolUse", "sess-c")), "", "claude")

	if len(env.sent) != 1 {
		t.Fatalf("expected the cached pane to be used, sent %d", len(env.sent))
	}
	var got agent.NormalizedPayload
	if err := json.Unmarshal(env.sent[0], &got); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if got.PaneID != 5 {
		t.Errorf("pane id: got %d, want 5", got.PaneID)
	}
}

func TestLoadPaneMap_DropsExpiredEntries(t *testing.T) {
	env := newHookTestEnv(t)
	env.writeMap(map[string]paneMapEntry{
		"fresh": {PaneID: 1, ZellijSession: "s", UpdatedAt: time.Now().Unix()},
		"stale": {PaneID: 2, ZellijSession: "s", UpdatedAt: time.Now().Add(-13 * time.Hour).Unix()},
	})

	m := loadPaneMap()

	if _, ok := m["fresh"]; !ok {
		t.Error("fresh entry should survive")
	}
	if _, ok := m["stale"]; ok {
		t.Error("entry older than the TTL should be dropped")
	}
}

// The older flat schema carries no Zellij session, so it can never be trusted.
// Discarding it wholesale is the intended migration.
func TestLoadPaneMap_DiscardsLegacyFlatSchema(t *testing.T) {
	env := newHookTestEnv(t)
	if err := os.WriteFile(paneMapFile, []byte(`{"old-session":27}`), 0o600); err != nil {
		t.Fatalf("write legacy map: %v", err)
	}
	_ = env

	m := loadPaneMap()

	if len(m) != 0 {
		t.Fatalf("expected the legacy file to be discarded, got %d entries", len(m))
	}
}

func TestRunHook_SessionEndRemovesCacheEntry(t *testing.T) {
	env := newHookTestEnv(t)
	env.writeMap(map[string]paneMapEntry{
		"sess-d": {PaneID: 4, ZellijSession: "cc-deck-local", UpdatedAt: time.Now().Unix()},
	})
	env.inZellij("cc-deck-local")

	runHook(strings.NewReader(claudeEvent("SessionEnd", "sess-d")), "4", "claude")

	if _, ok := env.readMap()["sess-d"]; ok {
		t.Error("SessionEnd should remove the cache entry")
	}
}

func TestCurrentZellijSession(t *testing.T) {
	t.Run("inside", func(t *testing.T) {
		t.Setenv("ZELLIJ", "0")
		t.Setenv("ZELLIJ_SESSION_NAME", "cc-deck-local")
		name, ok := currentZellijSession()
		if !ok || name != "cc-deck-local" {
			t.Errorf("got (%q, %v), want (\"cc-deck-local\", true)", name, ok)
		}
	})
	t.Run("outside", func(t *testing.T) {
		t.Setenv("ZELLIJ", "")
		t.Setenv("ZELLIJ_SESSION_NAME", "")
		if _, ok := currentZellijSession(); ok {
			t.Error("expected not to be inside Zellij")
		}
	})
}
