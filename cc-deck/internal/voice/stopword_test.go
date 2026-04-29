package voice

import "testing"

func TestIsWhisperArtifact(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		artifact bool
	}{
		{"empty string", "", true},
		{"whitespace only", "   ", true},
		{"bracketed noise", "[background noise]", true},
		{"parenthesized noise", "(wind blowing)", true},
		{"music notation", "♪ ♪", true},
		{"music emoji", "🎵", true},
		{"blank audio keyword", "blank_audio", true},
		{"music keyword", "[Music]", true},
		{"clicking keyword", "(clicking)", true},
		{"applause keyword", "[Applause]", true},
		{"laughter keyword", "(Laughter)", true},
		{"silence keyword", "[SILENCE]", true},
		{"JSON-like response", "{}", true},
		{"JSON object", "{\"text\": \"\"}", true},
		{"punctuation only", "...", true},
		{"real speech", "add error handling to the API", false},
		{"single word", "hello", false},
		{"command word", "send", false},
		{"mixed alpha and bracket", "go [do something", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsWhisperArtifact(tt.input)
			if got != tt.artifact {
				t.Errorf("IsWhisperArtifact(%q) = %v, want %v", tt.input, got, tt.artifact)
			}
		})
	}
}

func TestProcessStopwords_Defaults(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		isCommand bool
		action    string
	}{
		{"send standalone", "send", true, "submit"},
		{"send uppercase", "Send", true, "submit"},
		{"send after filler", "um, send", true, "submit"},
		{"send after multiple fillers", "uh um send", true, "submit"},
		{"send in sentence", "please send the email", false, ""},
		{"send with suffix", "send it", false, ""},
		{"submit is not default", "submit", false, ""},
		{"enter is not default", "enter", false, ""},
		{"empty string", "", false, ""},
		{"whitespace only", "   ", false, ""},
		{"filler only", "um uh hmm", false, ""},
		{"regular text", "add error handling to the API", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ProcessStopwords(tt.input, nil)
			if result.IsCommand != tt.isCommand {
				t.Errorf("IsCommand = %v, want %v (input: %q)", result.IsCommand, tt.isCommand, tt.input)
			}
			if result.CommandAction != tt.action {
				t.Errorf("CommandAction = %q, want %q (input: %q)", result.CommandAction, tt.action, tt.input)
			}
		})
	}
}

func TestProcessStopwords_CustomCommands(t *testing.T) {
	commands := BuildCommandMap(map[string][]string{
		"submit": {"go", "done", "fire"},
	})

	tests := []struct {
		name      string
		input     string
		isCommand bool
		action    string
	}{
		{"go standalone", "go", true, "submit"},
		{"done standalone", "done", true, "submit"},
		{"fire standalone", "fire", true, "submit"},
		{"fire uppercase", "FIRE", true, "submit"},
		{"fire after filler", "um fire", true, "submit"},
		{"send not configured", "send", false, ""},
		{"regular text", "go ahead and fix it", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ProcessStopwords(tt.input, commands)
			if result.IsCommand != tt.isCommand {
				t.Errorf("IsCommand = %v, want %v (input: %q)", result.IsCommand, tt.isCommand, tt.input)
			}
			if result.CommandAction != tt.action {
				t.Errorf("CommandAction = %q, want %q (input: %q)", result.CommandAction, tt.action, tt.input)
			}
		})
	}
}

func TestBuildCommandMap(t *testing.T) {
	actions := map[string][]string{
		"submit": {"send", "go"},
		"attend": {"next", "switch"},
	}
	m := BuildCommandMap(actions)

	expected := map[string]string{
		"send":   "submit",
		"go":     "submit",
		"next":   "attend",
		"switch": "attend",
	}

	for word, action := range expected {
		if got := m[word]; got != action {
			t.Errorf("BuildCommandMap[%q] = %q, want %q", word, got, action)
		}
	}

	if len(m) != len(expected) {
		t.Errorf("BuildCommandMap has %d entries, want %d", len(m), len(expected))
	}
}
