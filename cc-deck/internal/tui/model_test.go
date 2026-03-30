package tui

import (
	"testing"
)

func TestBuildAttachCommand_Local(t *testing.T) {
	cmd := buildAttachCommand("my-project", "local")
	if cmd == nil {
		t.Fatal("expected non-nil command for local type")
	}
	args := cmd.Args
	if len(args) != 3 || args[0] != "zellij" || args[1] != "attach" || args[2] != "cc-deck-my-project" {
		t.Errorf("unexpected args: %v", args)
	}
}

func TestBuildAttachCommand_Container(t *testing.T) {
	cmd := buildAttachCommand("my-project", "container")
	if cmd == nil {
		t.Fatal("expected non-nil command for container type")
	}
	args := cmd.Args
	if len(args) < 5 || args[0] != "podman" || args[1] != "exec" {
		t.Errorf("unexpected args: %v", args)
	}
	// Container name is args[3] (after -it)
	if args[3] != "cc-deck-my-project" {
		t.Errorf("expected container name cc-deck-my-project, got %s", args[3])
	}
}

func TestBuildAttachCommand_Unknown(t *testing.T) {
	cmd := buildAttachCommand("my-project", "k8s-deploy")
	if cmd != nil {
		t.Error("expected nil command for unsupported type")
	}
}

func TestStatusIndicator(t *testing.T) {
	cases := []struct {
		state    string
		notEmpty bool
	}{
		{"running", true},
		{"stopped", true},
		{"creating", true},
		{"error", true},
		{"unknown", true},
	}
	for _, tc := range cases {
		result := statusIndicator(tc.state)
		if tc.notEmpty && result == "" {
			t.Errorf("statusIndicator(%q) returned empty string", tc.state)
		}
	}
}

func TestTruncate(t *testing.T) {
	cases := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hell…"},
		{"hi", 2, "hi"},
		{"", 5, ""},
		{"abc", 0, ""},
	}
	for _, tc := range cases {
		got := truncate(tc.input, tc.maxLen)
		if got != tc.expected {
			t.Errorf("truncate(%q, %d) = %q, want %q", tc.input, tc.maxLen, got, tc.expected)
		}
	}
}
