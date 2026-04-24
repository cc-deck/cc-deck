package ws

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestLocalPipeChannel_Send_EmptyName(t *testing.T) {
	ch := &localPipeChannel{name: "test"}
	err := ch.Send(context.Background(), "", "payload")
	if err == nil {
		t.Fatal("expected error for empty pipe name")
	}
	var chErr *ChannelError
	if !errors.As(err, &chErr) {
		t.Fatalf("expected ChannelError, got %T", err)
	}
	if chErr.Op != "send" {
		t.Errorf("Op = %q, want %q", chErr.Op, "send")
	}
}

func TestLocalPipeChannel_SendReceive_EmptyName(t *testing.T) {
	ch := &localPipeChannel{name: "test"}
	_, err := ch.SendReceive(context.Background(), "", "payload")
	if err == nil {
		t.Fatal("expected error for empty pipe name")
	}
	var chErr *ChannelError
	if !errors.As(err, &chErr) {
		t.Fatalf("expected ChannelError, got %T", err)
	}
	if chErr.Op != "sendReceive" {
		t.Errorf("Op = %q, want %q", chErr.Op, "sendReceive")
	}
}

func TestExecPipeChannel_Send_EmptyName(t *testing.T) {
	ch := &execPipeChannel{
		name: "test",
		execFn: func(_ context.Context, _ []string) error {
			return nil
		},
	}
	err := ch.Send(context.Background(), "", "payload")
	if err == nil {
		t.Fatal("expected error for empty pipe name")
	}
	var chErr *ChannelError
	if !errors.As(err, &chErr) {
		t.Fatalf("expected ChannelError, got %T", err)
	}
}

func TestExecPipeChannel_Send_Success(t *testing.T) {
	var capturedCmd []string
	ch := &execPipeChannel{
		name: "test-ws",
		execFn: func(_ context.Context, cmd []string) error {
			capturedCmd = cmd
			return nil
		},
	}
	err := ch.Send(context.Background(), "cc-deck:voice", "hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"zellij", "pipe", "--name", "cc-deck:voice", "--", "hello world"}
	if len(capturedCmd) != len(expected) {
		t.Fatalf("cmd length = %d, want %d", len(capturedCmd), len(expected))
	}
	for i, v := range expected {
		if capturedCmd[i] != v {
			t.Errorf("cmd[%d] = %q, want %q", i, capturedCmd[i], v)
		}
	}
}

func TestExecPipeChannel_Send_ExecError(t *testing.T) {
	ch := &execPipeChannel{
		name: "test-ws",
		execFn: func(_ context.Context, _ []string) error {
			return fmt.Errorf("exec failed")
		},
	}
	err := ch.Send(context.Background(), "cc-deck:voice", "hello")
	if err == nil {
		t.Fatal("expected error from exec failure")
	}
	var chErr *ChannelError
	if !errors.As(err, &chErr) {
		t.Fatalf("expected ChannelError, got %T", err)
	}
	if chErr.Channel != "pipe" {
		t.Errorf("Channel = %q, want %q", chErr.Channel, "pipe")
	}
}

func TestExecPipeChannel_SendReceive_NilExecOutputFn(t *testing.T) {
	ch := &execPipeChannel{
		name:   "test",
		execFn: nil,
	}
	_, err := ch.SendReceive(context.Background(), "pipe", "payload")
	if !errors.Is(err, ErrNotSupported) {
		t.Errorf("expected ErrNotSupported, got %v", err)
	}
}

func TestExecPipeChannel_SendReceive_EmptyName(t *testing.T) {
	ch := &execPipeChannel{
		name: "test",
		execOutputFn: func(_ context.Context, _ []string) (string, error) {
			return "", nil
		},
	}
	_, err := ch.SendReceive(context.Background(), "", "payload")
	if err == nil {
		t.Fatal("expected error for empty pipe name")
	}
	var chErr *ChannelError
	if !errors.As(err, &chErr) {
		t.Fatalf("expected ChannelError, got %T", err)
	}
	if chErr.Op != "sendReceive" {
		t.Errorf("Op = %q, want %q", chErr.Op, "sendReceive")
	}
}

func TestExecPipeChannel_SendReceive_Success(t *testing.T) {
	var capturedCmd []string
	ch := &execPipeChannel{
		name: "test-ws",
		execOutputFn: func(_ context.Context, cmd []string) (string, error) {
			capturedCmd = cmd
			return "  response text\n", nil
		},
	}
	result, err := ch.SendReceive(context.Background(), "cc-deck:voice-control", "listen")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "response text" {
		t.Errorf("result = %q, want %q", result, "response text")
	}
	expected := []string{"zellij", "pipe", "--name", "cc-deck:voice-control", "--", "listen"}
	if len(capturedCmd) != len(expected) {
		t.Fatalf("cmd length = %d, want %d", len(capturedCmd), len(expected))
	}
	for i, v := range expected {
		if capturedCmd[i] != v {
			t.Errorf("cmd[%d] = %q, want %q", i, capturedCmd[i], v)
		}
	}
}

func TestExecPipeChannel_SendReceive_ExecError(t *testing.T) {
	ch := &execPipeChannel{
		name: "test-ws",
		execOutputFn: func(_ context.Context, _ []string) (string, error) {
			return "", fmt.Errorf("exec failed")
		},
	}
	_, err := ch.SendReceive(context.Background(), "cc-deck:voice", "hello")
	if err == nil {
		t.Fatal("expected error from exec failure")
	}
	var chErr *ChannelError
	if !errors.As(err, &chErr) {
		t.Fatalf("expected ChannelError, got %T", err)
	}
	if chErr.Channel != "pipe" {
		t.Errorf("Channel = %q, want %q", chErr.Channel, "pipe")
	}
}

func TestExecPipeChannel_SendReceive_EmptyResponse(t *testing.T) {
	ch := &execPipeChannel{
		name: "test-ws",
		execOutputFn: func(_ context.Context, _ []string) (string, error) {
			return "", nil
		},
	}
	result, err := ch.SendReceive(context.Background(), "cc-deck:voice", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("result = %q, want empty string", result)
	}
}
