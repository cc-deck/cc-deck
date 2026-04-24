package voice

import "testing"

func TestProcessStopwords(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		isCommand bool
		action    string
	}{
		{"submit standalone", "submit", true, "submit"},
		{"enter standalone", "enter", true, "enter"},
		{"submit in sentence", "please submit the form", false, ""},
		{"enter in sentence", "press enter to continue", false, ""},
		{"submit with prefix", "okay submit", false, ""},
		{"submit with suffix", "submit it", false, ""},
		{"submit after filler", "um, submit", true, "submit"},
		{"submit after multiple fillers", "uh um submit", true, "submit"},
		{"enter after filler", "ah enter", true, "enter"},
		{"empty string", "", false, ""},
		{"whitespace only", "   ", false, ""},
		{"filler only", "um uh hmm", false, ""},
		{"regular text", "add error handling to the API", false, ""},
		{"submit uppercase", "Submit", true, "submit"},
		{"enter uppercase", "ENTER", true, "enter"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ProcessStopwords(tt.input)
			if result.IsCommand != tt.isCommand {
				t.Errorf("IsCommand = %v, want %v (input: %q)", result.IsCommand, tt.isCommand, tt.input)
			}
			if result.CommandAction != tt.action {
				t.Errorf("CommandAction = %q, want %q (input: %q)", result.CommandAction, tt.action, tt.input)
			}
		})
	}
}
