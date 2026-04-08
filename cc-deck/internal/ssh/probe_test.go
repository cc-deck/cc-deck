package ssh

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// mockRunner implements Runner for testing.
type mockRunner struct {
	output string
	err    error
}

func (m *mockRunner) Run(_ context.Context, _ string) (string, error) {
	return m.output, m.err
}

func TestProbe_Success(t *testing.T) {
	runner := &mockRunner{
		output: "/usr/bin/zellij\n/usr/local/bin/cc-deck\n/usr/local/bin/claude",
		err:    nil,
	}

	err := Probe(context.Background(), runner)
	if err != nil {
		t.Errorf("Probe() returned error for provisioned host: %v", err)
	}
}

func TestProbe_MissingTools(t *testing.T) {
	runner := &mockRunner{
		output: "",
		err:    fmt.Errorf("exit status 1"),
	}

	err := Probe(context.Background(), runner)
	if err == nil {
		t.Fatal("Probe() should return error for unprovisioned host")
	}

	msg := err.Error()
	if !strings.Contains(msg, "host appears unprovisioned") {
		t.Errorf("error should mention 'host appears unprovisioned', got: %s", msg)
	}
	if !strings.Contains(msg, "cc-deck setup") {
		t.Errorf("error should mention 'cc-deck setup', got: %s", msg)
	}
}

func TestProbe_MissingToolsWithOutput(t *testing.T) {
	runner := &mockRunner{
		output: "/usr/bin/zellij",
		err:    fmt.Errorf("exit status 1"),
	}

	err := Probe(context.Background(), runner)
	if err == nil {
		t.Fatal("Probe() should return error when some tools are missing")
	}

	msg := err.Error()
	if !strings.Contains(msg, "/usr/bin/zellij") {
		t.Errorf("error should include partial output, got: %s", msg)
	}
}

// Verify that Client satisfies the Runner interface.
var _ Runner = (*Client)(nil)
