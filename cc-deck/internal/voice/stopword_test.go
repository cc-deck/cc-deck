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
		{"asterisk noise", "*sigh*", true},
		{"asterisk birds", "*Birds chirping*", true},
		{"asterisk door", "*Door slams*", true},
		{"hallucination thank you", "Thank you for watching!", true},
		{"hallucination thanks", "Thanks for watching!", true},
		{"hallucination subscribe", "Please subscribe and like!", true},
		{"hallucination see you", "See you in the next video!", true},
		{"hallucination bye", "Bye bye!", true},
		{"hallucination mixed case", "THANK YOU FOR WATCHING", true},
		{"hallucination in sentence", "So thank you for watching and goodbye", true},
		{"real speech", "add error handling to the API", false},
		{"single word", "hello", false},
		{"command word", "send", false},
		{"mixed alpha and bracket", "go [do something", false},
		{"asterisk mid-sentence", "press *this* button", false},
		{"repetition loop", "and the other one.and the other one. Thank you.", true},
		{"repetition with thank you", "Thank you.Thank you for watching!Thank you.", true},
		{"repetition short", "yes.yes.", true},
		{"no repetition", "first thing. second thing. third thing.", false},
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
		{"send it standalone", "send it", true, "submit"},
		{"send it uppercase", "Send it", true, "submit"},
		{"send it after filler", "um, send it", true, "submit"},
		{"send it after multiple fillers", "uh um send it", true, "submit"},
		{"send it with punctuation", "Send it!", true, "submit"},
		{"send alone is not command", "send", false, ""},
		{"send in sentence", "please send the email", false, ""},
		{"submit is not default", "submit", false, ""},
		{"enter is not default", "enter", false, ""},
		{"empty string", "", false, ""},
		{"whitespace only", "   ", false, ""},
		{"filler only", "um uh hmm", false, ""},
		{"regular text", "add error handling to the API", false, ""},
		{"ship it standalone", "ship it", true, "submit_attend"},
		{"ship it uppercase", "Ship it", true, "submit_attend"},
		{"ship it after filler", "um, ship it", true, "submit_attend"},
		{"ship alone is not command", "ship", false, ""},
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

func TestProcessStopwords_DefaultAttend(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		isCommand bool
		action    string
	}{
		{"go next standalone", "go next", true, "attend"},
		{"go next uppercase", "Go next", true, "attend"},
		{"go next after filler", "um, go next", true, "attend"},
		{"go next after multiple fillers", "uh um go next", true, "attend"},
		{"next alone is not command", "next", false, ""},
		{"next in sentence", "the next step is to refactor", false, ""},
		{"send it still works", "send it", true, "submit"},
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

func TestProcessStopwords_CustomAttendWord(t *testing.T) {
	commands := BuildCommandMap(map[string][]string{
		"submit": {"send"},
		"attend": {"switch"},
	})

	tests := []struct {
		name      string
		input     string
		isCommand bool
		action    string
	}{
		{"switch triggers attend", "switch", true, "attend"},
		{"switch uppercase", "SWITCH", true, "attend"},
		{"switch after filler", "um switch", true, "attend"},
		{"next no longer triggers", "next", false, ""},
		{"send still works", "send", true, "submit"},
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

func TestProcessStopwords_MultipleAttendWords(t *testing.T) {
	commands := BuildCommandMap(map[string][]string{
		"submit": {"send"},
		"attend": {"next", "switch"},
	})

	tests := []struct {
		name      string
		input     string
		isCommand bool
		action    string
	}{
		{"next triggers attend", "next", true, "attend"},
		{"switch triggers attend", "switch", true, "attend"},
		{"send still works", "send", true, "submit"},
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
