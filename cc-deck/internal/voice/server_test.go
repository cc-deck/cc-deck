package voice

import (
	"strings"
	"testing"
)

func TestWhisperServer_ArgsWithoutSpeechFilter(t *testing.T) {
	s := NewWhisperServer("/models/ggml-medium.bin", 8234)
	args := strings.Join(s.args(), " ")
	if args != "-m /models/ggml-medium.bin --host 127.0.0.1 --port 8234" {
		t.Errorf("args = %q", args)
	}
}

func TestWhisperServer_ArgsWithSpeechFilter(t *testing.T) {
	s := NewWhisperServer("/models/ggml-large-v3-turbo.bin", 8234)
	s.SetVADModel("/models/ggml-silero-v6.2.0.bin")
	args := strings.Join(s.args(), " ")
	if !strings.HasSuffix(args, "--vad --vad-model /models/ggml-silero-v6.2.0.bin") {
		t.Errorf("args should enable the speech filter, got %q", args)
	}

	s.SetVADModel("")
	if strings.Contains(strings.Join(s.args(), " "), "--vad") {
		t.Error("an empty VAD model path should disable --vad")
	}
}
