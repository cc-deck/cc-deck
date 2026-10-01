package voice

import (
	"testing"
	"time"
)

func TestParseTurnMode(t *testing.T) {
	tests := []struct {
		input   string
		want    TurnMode
		wantErr bool
	}{
		{"basic", TurnModeBasic, false},
		{"Basic", TurnModeBasic, false},
		{"BASIC", TurnModeBasic, false},
		{"tdrz", TurnModeTdrz, false},
		{"Tdrz", TurnModeTdrz, false},
		{"TDRZ", TurnModeTdrz, false},
		{"", "", true},
		{"auto", "", true},
		{"whisper", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseTurnMode(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTurnMode(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseTurnMode(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSplitDashTurns(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantTexts []string
		wantTurns []bool
	}{
		{
			name:      "no markers",
			input:     "Hello world",
			wantTexts: []string{"Hello world"},
			wantTurns: []bool{false},
		},
		{
			name:      "leading dash marker",
			input:     "- Hello world",
			wantTexts: []string{"Hello world"},
			wantTurns: []bool{true},
		},
		{
			name:      "speaker label dash",
			input:     "- Name: Hello world",
			wantTexts: []string{"Hello world"},
			wantTurns: []bool{true},
		},
		{
			name:      "mid-text marker after period",
			input:     "Sure. - What about Friday?",
			wantTexts: []string{"Sure.", "What about Friday?"},
			wantTurns: []bool{false, true},
		},
		{
			name:      "mid-text marker after question mark",
			input:     "Really? - Yes, definitely.",
			wantTexts: []string{"Really?", "Yes, definitely."},
			wantTurns: []bool{false, true},
		},
		{
			name:      "mid-text marker after exclamation",
			input:     "Great! - Let us move on.",
			wantTexts: []string{"Great!", "Let us move on."},
			wantTurns: []bool{false, true},
		},
		{
			name:      "mid-text marker with extra spaces",
			input:     "Sure.  -  What about Friday?",
			wantTexts: []string{"Sure.", "What about Friday?"},
			wantTurns: []bool{false, true},
		},
		{
			name:      "parenthetical dash NOT split",
			input:     "well - you know what I mean",
			wantTexts: []string{"well - you know what I mean"},
			wantTurns: []bool{false},
		},
		{
			name:      "bracket annotations removed",
			input:     "- [typing] Hello world",
			wantTexts: []string{"Hello world"},
			wantTurns: []bool{true},
		},
		{
			name:      "empty pieces dropped with TurnStart carry-over",
			input:     "- [silence]",
			wantTexts: nil,
			wantTurns: nil,
		},
		{
			name:      "multiple turns",
			input:     "- First speaker. - Second speaker. - Third speaker.",
			wantTexts: []string{"First speaker.", "Second speaker.", "Third speaker."},
			wantTurns: []bool{true, true, true},
		},
		{
			name:      "empty input",
			input:     "",
			wantTexts: nil,
			wantTurns: nil,
		},
		{
			name:      "whitespace only",
			input:     "   ",
			wantTexts: nil,
			wantTurns: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			segs := SplitDashTurns(tt.input)

			if len(segs) != len(tt.wantTexts) {
				var texts []string
				for _, s := range segs {
					texts = append(texts, s.Text)
				}
				t.Fatalf("got %d segments %v, want %d %v", len(segs), texts, len(tt.wantTexts), tt.wantTexts)
			}

			for i, seg := range segs {
				if seg.Text != tt.wantTexts[i] {
					t.Errorf("seg[%d].Text = %q, want %q", i, seg.Text, tt.wantTexts[i])
				}
				if seg.TurnStart != tt.wantTurns[i] {
					t.Errorf("seg[%d].TurnStart = %v, want %v", i, seg.TurnStart, tt.wantTurns[i])
				}
			}
		})
	}
}

func TestApplyPauseBreak(t *testing.T) {
	tests := []struct {
		name      string
		gap       time.Duration
		threshold time.Duration
		first     bool
		initial   bool
		want      bool
	}{
		{"first passage forces turn", 0, 3 * time.Second, true, false, true},
		{"gap above threshold", 4 * time.Second, 3 * time.Second, false, false, true},
		{"gap below threshold", 2 * time.Second, 3 * time.Second, false, false, false},
		{"gap equal to threshold not a turn", 3 * time.Second, 3 * time.Second, false, false, false},
		{"preserves existing true", 2 * time.Second, 3 * time.Second, false, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			segs := []Segment{{Text: "test", TurnStart: tt.initial}}
			ApplyPauseBreak(segs, tt.gap, tt.threshold, tt.first)
			if segs[0].TurnStart != tt.want {
				t.Errorf("TurnStart = %v, want %v", segs[0].TurnStart, tt.want)
			}
		})
	}

	// Empty slice should not panic.
	ApplyPauseBreak(nil, time.Second, time.Second, true)
	ApplyPauseBreak([]Segment{}, time.Second, time.Second, true)
}
