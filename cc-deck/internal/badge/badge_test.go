package badge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cc-deck/cc-deck/internal/config"
)

func TestExtractDotPath(t *testing.T) {
	tests := []struct {
		name string
		json string
		path string
		want string
	}{
		{"simple key", `{"mode":"ship"}`, ".mode", "ship"},
		{"nested key", `{"result":{"outcome":"pass"}}`, ".result.outcome", "pass"},
		{"deep nested", `{"a":{"b":{"c":"deep"}}}`, ".a.b.c", "deep"},
		{"numeric value", `{"count":42}`, ".count", "42"},
		{"boolean value", `{"ok":true}`, ".ok", "true"},
		{"missing key", `{"mode":"ship"}`, ".status", ""},
		{"missing nested", `{"a":{"b":"c"}}`, ".a.x", ""},
		{"empty path", `{"mode":"ship"}`, "", ""},
		{"dot only", `{"mode":"ship"}`, ".", ""},
		{"non-object root", `"hello"`, ".mode", ""},
		{"null value", `{"mode":null}`, ".mode", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var doc interface{}
			if err := json.Unmarshal([]byte(tt.json), &doc); err != nil {
				t.Fatalf("bad test JSON: %v", err)
			}
			got := extractDotPath(doc, tt.path)
			if got != tt.want {
				t.Errorf("extractDotPath(%s) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestEvaluateRule(t *testing.T) {
	dir := t.TempDir()

	// Write a test state file
	stateFile := filepath.Join(dir, ".spex-state")
	os.WriteFile(stateFile, []byte(`{"mode":"ship"}`), 0o644)

	tests := []struct {
		name string
		rule config.BadgeRule
		want string
	}{
		{
			"matching value",
			config.BadgeRule{File: ".spex-state", Format: "json", Extract: ".mode", Values: map[string]string{"ship": "S", "flow": "F"}},
			"S",
		},
		{
			"default on unmatched value",
			config.BadgeRule{File: ".spex-state", Format: "json", Extract: ".mode", Values: map[string]string{"flow": "F"}, Default: "D"},
			"D",
		},
		{
			"missing file",
			config.BadgeRule{File: "nonexistent.json", Format: "json", Extract: ".mode", Values: map[string]string{"ship": "S"}},
			"",
		},
		{
			"default on missing path",
			config.BadgeRule{File: ".spex-state", Format: "json", Extract: ".nonexistent", Values: map[string]string{}, Default: "D"},
			"D",
		},
		{
			"no default no match",
			config.BadgeRule{File: ".spex-state", Format: "json", Extract: ".nonexistent", Values: map[string]string{}},
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateRule(tt.rule, dir)
			if got != tt.want {
				t.Errorf("evaluateRule() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEvaluate(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "state.json"), []byte(`{"mode":"ship"}`), 0o644)
	os.WriteFile(filepath.Join(dir, "scan.json"), []byte(`{"status":"pass"}`), 0o644)

	rules := []config.BadgeRule{
		{Name: "spex", File: "state.json", Format: "json", Extract: ".mode", Values: map[string]string{"ship": "S", "flow": "F"}},
		{Name: "scan", File: "scan.json", Format: "json", Extract: ".status", Values: map[string]string{"pass": "P", "fail": "X"}},
	}

	badges := Evaluate(rules, dir)
	if len(badges) != 2 {
		t.Fatalf("expected 2 badges, got %d", len(badges))
	}
	if badges[0] != "S" {
		t.Errorf("badge[0] = %q, want S", badges[0])
	}
	if badges[1] != "P" {
		t.Errorf("badge[1] = %q, want P", badges[1])
	}
}

func TestEvaluateEmptyRules(t *testing.T) {
	badges := Evaluate(nil, "/tmp")
	if badges != nil {
		t.Errorf("expected nil, got %v", badges)
	}
}

func TestEvaluateEmptyWorkingDir(t *testing.T) {
	rules := []config.BadgeRule{{Name: "x", File: "f", Format: "json", Extract: ".k", Values: map[string]string{}}}
	badges := Evaluate(rules, "")
	if badges != nil {
		t.Errorf("expected nil, got %v", badges)
	}
}

func TestEvaluateSkipsInvalidFormat(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "f.yaml"), []byte("key: val"), 0o644)

	rules := []config.BadgeRule{{Name: "x", File: "f.yaml", Format: "yaml", Extract: ".key", Values: map[string]string{"val": "V"}}}
	badges := Evaluate(rules, dir)
	if badges != nil {
		t.Errorf("expected nil for non-json format, got %v", badges)
	}
}

func TestEvaluateInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "bad.json"), []byte("{invalid"), 0o644)

	rules := []config.BadgeRule{{Name: "x", File: "bad.json", Format: "json", Extract: ".key", Values: map[string]string{}}}
	badges := Evaluate(rules, dir)
	if badges != nil {
		t.Errorf("expected nil for invalid JSON, got %v", badges)
	}
}

func TestEvaluateEmptyFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "empty.json"), []byte(""), 0o644)

	rules := []config.BadgeRule{{Name: "x", File: "empty.json", Format: "json", Extract: ".key", Values: map[string]string{}, Default: "D"}}
	badges := Evaluate(rules, dir)
	if badges != nil {
		t.Errorf("expected nil for empty file, got %v", badges)
	}
}

func TestEvaluatePartialMatch(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.json"), []byte(`{"mode":"ship"}`), 0o644)

	rules := []config.BadgeRule{
		{Name: "exists", File: "a.json", Format: "json", Extract: ".mode", Values: map[string]string{"ship": "S"}},
		{Name: "missing", File: "nope.json", Format: "json", Extract: ".x", Values: map[string]string{}},
	}
	badges := Evaluate(rules, dir)
	if len(badges) != 1 || badges[0] != "S" {
		t.Errorf("expected [S], got %v", badges)
	}
}
