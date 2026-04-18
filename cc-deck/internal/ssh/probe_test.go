package ssh

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// mockRunner implements Runner for testing.
type mockRunner struct {
	// results maps command substrings to output/error pairs.
	// When Run is called, the first matching key is used.
	results map[string]mockResult
	// fallback is used when no key matches.
	fallback mockResult
}

type mockResult struct {
	output string
	err    error
}

func (m *mockRunner) Run(_ context.Context, cmd string) (string, error) {
	for key, result := range m.results {
		if strings.Contains(cmd, key) {
			return result.output, result.err
		}
	}
	return m.fallback.output, m.fallback.err
}

func TestProbe_Success(t *testing.T) {
	runner := &mockRunner{
		fallback: mockResult{output: "/usr/local/bin/tool", err: nil},
	}

	err := Probe(context.Background(), runner)
	if err != nil {
		t.Errorf("Probe() returned error for provisioned host: %v", err)
	}
}

func TestProbe_AllMissing(t *testing.T) {
	runner := &mockRunner{
		fallback: mockResult{output: "", err: fmt.Errorf("exit status 1")},
	}

	err := Probe(context.Background(), runner)
	if err == nil {
		t.Fatal("Probe() should return error for unprovisioned host")
	}

	msg := err.Error()
	if !strings.Contains(msg, "host appears unprovisioned") {
		t.Errorf("error should mention 'host appears unprovisioned', got: %s", msg)
	}
	if !strings.Contains(msg, "cc-deck build") {
		t.Errorf("error should mention 'cc-deck build', got: %s", msg)
	}
	for _, tool := range requiredTools {
		if !strings.Contains(msg, tool) {
			t.Errorf("error should list missing tool %q, got: %s", tool, msg)
		}
	}
}

func TestProbe_PartialMissing(t *testing.T) {
	runner := &mockRunner{
		results: map[string]mockResult{
			"which zellij":  {output: "/usr/local/bin/zellij", err: nil},
			"which cc-deck": {output: "/usr/local/bin/cc-deck", err: nil},
			"which claude":  {output: "", err: fmt.Errorf("exit status 1")},
		},
	}

	err := Probe(context.Background(), runner)
	if err == nil {
		t.Fatal("Probe() should return error when some tools are missing")
	}

	msg := err.Error()
	if !strings.Contains(msg, "claude") {
		t.Errorf("error should list missing tool 'claude', got: %s", msg)
	}
	if strings.Contains(msg, "zellij") {
		t.Errorf("error should NOT list found tool 'zellij', got: %s", msg)
	}
}

// Verify that Client satisfies the Runner interface.
var _ Runner = (*Client)(nil)
