package ws

import (
	"errors"
	"fmt"
	"testing"
)

func TestChannelError_Error(t *testing.T) {
	err := newChannelError("pipe", "send", "dev", "workspace 'dev' is not running", nil)
	want := "pipe send failed: workspace 'dev' is not running"
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestChannelError_Unwrap(t *testing.T) {
	cause := fmt.Errorf("connection refused")
	err := newChannelError("data", "push", "prod", "transport error", cause)
	if got := err.Unwrap(); got != cause {
		t.Errorf("Unwrap() = %v, want %v", got, cause)
	}
}

func TestChannelError_UnwrapNil(t *testing.T) {
	err := newChannelError("git", "fetch", "dev", "no commits", nil)
	if got := err.Unwrap(); got != nil {
		t.Errorf("Unwrap() = %v, want nil", got)
	}
}

func TestChannelError_ErrorsIs(t *testing.T) {
	cause := ErrNotSupported
	err := newChannelError("git", "fetch", "local", "not supported", cause)
	if !errors.Is(err, ErrNotSupported) {
		t.Error("errors.Is should find ErrNotSupported through Unwrap chain")
	}
}

func TestChannelError_ErrorsAs(t *testing.T) {
	cause := fmt.Errorf("timeout")
	wrapped := fmt.Errorf("outer: %w", newChannelError("pipe", "send", "dev", "timed out", cause))

	var chErr *ChannelError
	if !errors.As(wrapped, &chErr) {
		t.Fatal("errors.As should find ChannelError in wrapped chain")
	}
	if chErr.Channel != "pipe" {
		t.Errorf("Channel = %q, want %q", chErr.Channel, "pipe")
	}
	if chErr.Op != "send" {
		t.Errorf("Op = %q, want %q", chErr.Op, "send")
	}
	if chErr.Workspace != "dev" {
		t.Errorf("Workspace = %q, want %q", chErr.Workspace, "dev")
	}
}
