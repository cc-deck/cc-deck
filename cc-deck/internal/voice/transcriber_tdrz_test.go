package voice

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseTdrzJSON_TurnFlags(t *testing.T) {
	data := []byte(`{
		"transcription": [
			{"offsets":{"from":0,"to":2380},"text":" So what about Friday?","speaker_turn_next":true},
			{"offsets":{"from":2380,"to":4100},"text":" Friday works for me.","speaker_turn_next":false}
		]
	}`)

	segs, err := ParseTdrzJSON(data)
	if err != nil {
		t.Fatalf("ParseTdrzJSON: %v", err)
	}
	if len(segs) != 2 {
		t.Fatalf("got %d segments, want 2", len(segs))
	}

	// First segment: TurnStart is false (relay applies pause-break rule).
	if segs[0].TurnStart {
		t.Error("seg[0].TurnStart should be false")
	}
	if segs[0].Text != "So what about Friday?" {
		t.Errorf("seg[0].Text = %q, want %q", segs[0].Text, "So what about Friday?")
	}
	if segs[0].Start != 0 {
		t.Errorf("seg[0].Start = %v, want 0", segs[0].Start)
	}
	if segs[0].End != 2380*time.Millisecond {
		t.Errorf("seg[0].End = %v, want 2380ms", segs[0].End)
	}

	// Second segment: TurnStart is true (speaker_turn_next on first segment).
	if !segs[1].TurnStart {
		t.Error("seg[1].TurnStart should be true (speaker_turn_next on seg[0])")
	}
	if segs[1].Text != "Friday works for me." {
		t.Errorf("seg[1].Text = %q, want %q", segs[1].Text, "Friday works for me.")
	}
}

func TestParseTdrzJSON_MissingField(t *testing.T) {
	// speaker_turn_next is absent: treated as false.
	data := []byte(`{
		"transcription": [
			{"offsets":{"from":0,"to":1000},"text":"hello"},
			{"offsets":{"from":1000,"to":2000},"text":"world"}
		]
	}`)

	segs, err := ParseTdrzJSON(data)
	if err != nil {
		t.Fatalf("ParseTdrzJSON: %v", err)
	}
	if len(segs) != 2 {
		t.Fatalf("got %d segments, want 2", len(segs))
	}
	if segs[1].TurnStart {
		t.Error("seg[1].TurnStart should be false when speaker_turn_next is absent")
	}
}

func TestParseTdrzJSON_EmptyTranscription(t *testing.T) {
	data := []byte(`{"transcription":[]}`)
	segs, err := ParseTdrzJSON(data)
	if err != nil {
		t.Fatalf("ParseTdrzJSON: %v", err)
	}
	if len(segs) != 0 {
		t.Errorf("got %d segments, want 0", len(segs))
	}
}

func TestParseTdrzJSON_MalformedJSON(t *testing.T) {
	_, err := ParseTdrzJSON([]byte(`not json`))
	if err == nil {
		t.Error("expected error for malformed JSON")
	}
}

func TestParseTdrzJSON_EmptyTextDroppedWithCarryOver(t *testing.T) {
	boolTrue := true
	_ = boolTrue // used indirectly through JSON
	data := []byte(`{
		"transcription": [
			{"offsets":{"from":0,"to":1000},"text":"hello","speaker_turn_next":true},
			{"offsets":{"from":1000,"to":1500},"text":"   ","speaker_turn_next":false},
			{"offsets":{"from":1500,"to":2500},"text":"world"}
		]
	}`)

	segs, err := ParseTdrzJSON(data)
	if err != nil {
		t.Fatalf("ParseTdrzJSON: %v", err)
	}
	if len(segs) != 2 {
		t.Fatalf("got %d segments, want 2 (empty dropped)", len(segs))
	}
	// TurnStart should carry from the dropped empty segment.
	if !segs[1].TurnStart {
		t.Error("seg[1].TurnStart should be true (carried from dropped empty)")
	}
}

func TestTdrzTranscriber_ArgumentsAndPrompt(t *testing.T) {
	var capturedArgs []string
	var capturedName string

	tr := &tdrzTranscriber{
		modelPath: "/models/ggml-small.en-tdrz.bin",
		runCmd: func(ctx context.Context, name string, args ...string) error {
			capturedName = name
			capturedArgs = args

			// Write canned JSON output.
			for i, a := range args {
				if a == "-of" && i+1 < len(args) {
					outPath := args[i+1] + ".json"
					return os.WriteFile(outPath, []byte(`{"transcription":[{"offsets":{"from":0,"to":1000},"text":"hello"}]}`), 0o644)
				}
			}
			return nil
		},
	}

	tr.SetPrompt("test prompt")

	segs, err := tr.TranscribeTurns(context.Background(), make([]int16, 16000), 16000)
	if err != nil {
		t.Fatalf("TranscribeTurns: %v", err)
	}

	if capturedName != "whisper-cli" {
		t.Errorf("command = %q, want whisper-cli", capturedName)
	}

	argsStr := strings.Join(capturedArgs, " ")
	if !strings.Contains(argsStr, "-m /models/ggml-small.en-tdrz.bin") {
		t.Errorf("missing -m flag in args: %v", capturedArgs)
	}
	if !strings.Contains(argsStr, "-tdrz") {
		t.Errorf("missing -tdrz flag in args: %v", capturedArgs)
	}
	if !strings.Contains(argsStr, "-oj") {
		t.Errorf("missing -oj flag in args: %v", capturedArgs)
	}
	if !strings.Contains(argsStr, "-np") {
		t.Errorf("missing -np flag in args: %v", capturedArgs)
	}
	if !strings.Contains(argsStr, "--prompt test prompt") {
		t.Errorf("missing --prompt in args: %v", capturedArgs)
	}

	if len(segs) != 1 {
		t.Fatalf("got %d segments, want 1", len(segs))
	}
	if segs[0].Text != "hello" {
		t.Errorf("seg[0].Text = %q, want %q", segs[0].Text, "hello")
	}
}

func TestTdrzTranscriber_RunCmdError(t *testing.T) {
	tr := &tdrzTranscriber{
		modelPath: "/models/test.bin",
		runCmd: func(ctx context.Context, name string, args ...string) error {
			return fmt.Errorf("exit status 1")
		},
	}

	_, err := tr.TranscribeTurns(context.Background(), make([]int16, 1000), 16000)
	if err == nil {
		t.Fatal("expected error for non-zero exit")
	}
	if !strings.Contains(err.Error(), "whisper-cli failed") {
		t.Errorf("error = %q, want whisper-cli failed", err.Error())
	}
}

func TestTdrzTranscriber_ContextDeadline(t *testing.T) {
	tr := &tdrzTranscriber{
		modelPath: "/models/test.bin",
		runCmd: func(ctx context.Context, name string, args ...string) error {
			<-ctx.Done()
			return ctx.Err()
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := tr.TranscribeTurns(ctx, make([]int16, 1000), 16000)
	if err == nil {
		t.Fatal("expected error for context deadline")
	}
}

func TestTdrzTranscriber_TempDirRemoved(t *testing.T) {
	var tmpDirSeen string

	tr := &tdrzTranscriber{
		modelPath: "/models/test.bin",
		runCmd: func(ctx context.Context, name string, args ...string) error {
			for i, a := range args {
				if a == "-f" && i+1 < len(args) {
					tmpDirSeen = filepath.Dir(args[i+1])
				}
				if a == "-of" && i+1 < len(args) {
					outPath := args[i+1] + ".json"
					return os.WriteFile(outPath, []byte(`{"transcription":[]}`), 0o644)
				}
			}
			return nil
		},
	}

	_, err := tr.TranscribeTurns(context.Background(), make([]int16, 1000), 16000)
	if err != nil {
		t.Fatalf("TranscribeTurns: %v", err)
	}

	if tmpDirSeen == "" {
		t.Fatal("could not determine temp dir from args")
	}
	if _, err := os.Stat(tmpDirSeen); !os.IsNotExist(err) {
		t.Errorf("temp dir %s should be removed after success", tmpDirSeen)
	}
}

func TestTdrzTranscriber_TempDirRemovedOnError(t *testing.T) {
	var tmpDirSeen string

	tr := &tdrzTranscriber{
		modelPath: "/models/test.bin",
		runCmd: func(ctx context.Context, name string, args ...string) error {
			for i, a := range args {
				if a == "-f" && i+1 < len(args) {
					tmpDirSeen = filepath.Dir(args[i+1])
				}
			}
			return fmt.Errorf("exit status 1")
		},
	}

	_, _ = tr.TranscribeTurns(context.Background(), make([]int16, 1000), 16000)

	if tmpDirSeen == "" {
		t.Fatal("could not determine temp dir from args")
	}
	if _, err := os.Stat(tmpDirSeen); !os.IsNotExist(err) {
		t.Errorf("temp dir %s should be removed after failure", tmpDirSeen)
	}
}

func TestTdrzTranscriber_NoPrompt(t *testing.T) {
	var capturedArgs []string

	tr := &tdrzTranscriber{
		modelPath: "/models/test.bin",
		runCmd: func(ctx context.Context, name string, args ...string) error {
			capturedArgs = args
			for i, a := range args {
				if a == "-of" && i+1 < len(args) {
					outPath := args[i+1] + ".json"
					return os.WriteFile(outPath, []byte(`{"transcription":[]}`), 0o644)
				}
			}
			return nil
		},
	}
	// No SetPrompt call.

	_, err := tr.TranscribeTurns(context.Background(), make([]int16, 1000), 16000)
	if err != nil {
		t.Fatalf("TranscribeTurns: %v", err)
	}

	for _, a := range capturedArgs {
		if a == "--prompt" {
			t.Error("--prompt should not be passed when no prompt is set")
		}
	}
}
