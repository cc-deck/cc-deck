package voice

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// TurnMode selects the turn detection strategy for a recording.
type TurnMode string

const (
	// TurnModeBasic uses Whisper dash markers and the pause-break threshold.
	TurnModeBasic TurnMode = "basic"
	// TurnModeTdrz uses whisper-cli with the tinydiarize model for
	// model-based speaker change detection.
	TurnModeTdrz TurnMode = "tdrz"
)

// ParseSpeakerSplit parses the user-facing speaker split setting
// (case-insensitive): "pause" selects TurnModeBasic and "voice" selects
// TurnModeTdrz. Any other value returns an error.
func ParseSpeakerSplit(s string) (TurnMode, error) {
	switch strings.ToLower(s) {
	case "pause":
		return TurnModeBasic, nil
	case "voice":
		return TurnModeTdrz, nil
	default:
		return "", fmt.Errorf("unknown speaker split %q (valid: pause, voice)", s)
	}
}

// Label returns the user-facing description of the mode, as shown after
// "Speakers:" in the TUI.
func (m TurnMode) Label() string {
	if m == TurnModeTdrz {
		return "by voice"
	}
	return "by pause"
}

// Segment is a piece of transcribed text with timing and turn information.
// A passage yields one or more segments; it yields more than one when it
// contains a turn change (a dash marker in basic mode, or a speaker_turn_next
// flag in tdrz mode).
type Segment struct {
	Text      string        // sanitized text (no dash marker, no bracket annotations)
	Start     time.Duration // offset from recording start
	End       time.Duration // offset from recording start
	At        time.Time     // wall-clock time of Start (recordingStartedAt + Start)
	TurnStart bool          // true when this segment begins a new turn
	Speaker   string        // always empty; reserved for future speaker labels
}

// dashSplitRe matches turn markers: a dash at the start of text, or a dash
// that follows sentence-ending punctuation (., ?, !) with optional whitespace.
// The first capture group keeps the punctuation on the preceding piece.
var dashSplitRe = regexp.MustCompile(`(^|[.?!])\s*-\s+`)

// speakerLabelNoDashRe matches a speaker label at the start of a segment
// piece after the leading dash was already consumed by the split. Requires
// a colon to distinguish from regular capitalized words.
var speakerLabelNoDashRe = regexp.MustCompile(`^[A-Z][a-z]+:\s*`)

// SplitDashTurns splits raw transcription text at Whisper dash markers into
// segments. A dash at the start of the text, or a dash following sentence-ending
// punctuation (., ?, !) with optional whitespace, is a split point. The
// punctuation stays with the preceding piece. Each piece is sanitized with the
// relay's text helpers. Empty pieces are dropped, but their TurnStart flag
// carries to the next kept piece.
func SplitDashTurns(raw string) []Segment {
	matches := dashSplitRe.FindAllStringSubmatchIndex(raw, -1)
	if len(matches) == 0 {
		// No dash markers: single segment, not a turn start (the caller
		// applies the pause-break rule).
		text := cleanSegmentText(raw)
		if text == "" {
			return nil
		}
		return []Segment{{Text: text}}
	}

	// Build parts from the split points. Each match produces:
	// - text before the marker (up to and including captured punctuation): NOT a turn start
	// - text after the marker: IS a turn start
	type part struct {
		text      string
		turnStart bool
	}
	var parts []part

	for i, loc := range matches {
		// loc[2]:loc[3] is the captured punctuation (or empty for ^)
		groupStart, groupEnd := loc[2], loc[3]

		// cutAt is where the preceding piece ends, keeping the punctuation.
		cutAt := groupEnd
		if groupStart == groupEnd {
			// Empty capture (^ anchor): cut at the start of the match.
			cutAt = loc[0]
		}

		// Text before this marker (only for the first match, before any marker).
		if i == 0 && cutAt > 0 {
			parts = append(parts, part{text: raw[0:cutAt], turnStart: false})
		}

		// For subsequent matches, text between the previous marker's end
		// and this marker's cut point is a turn start (it followed a marker).
		if i > 0 {
			prevMatchEnd := matches[i-1][1]
			if cutAt > prevMatchEnd {
				parts = append(parts, part{text: raw[prevMatchEnd:cutAt], turnStart: true})
			}
		}

		// Text after the last marker is handled below.
	}

	// Remainder after the last marker is a turn start.
	lastMatchEnd := matches[len(matches)-1][1]
	if lastMatchEnd < len(raw) {
		parts = append(parts, part{text: raw[lastMatchEnd:], turnStart: true})
	}

	// Clean and build segments, carrying TurnStart over empty pieces.
	var segs []Segment
	pendingTurn := false
	for _, p := range parts {
		text := cleanSegmentText(p.text)
		isTurn := p.turnStart || pendingTurn
		if text == "" {
			if isTurn {
				pendingTurn = true
			}
			continue
		}
		pendingTurn = false
		segs = append(segs, Segment{
			Text:      text,
			TurnStart: isTurn,
		})
	}

	return segs
}

// cleanSegmentText sanitizes a segment piece using the relay's text helpers.
func cleanSegmentText(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	s = sanitizeTerminalText(s)
	s = stripBracketedAnnotations(s)
	s = stripLeadingDash(s)
	// Strip speaker labels that lost their leading dash during splitting
	// (e.g., "Name: Hello" after the "- " was consumed by the split regex).
	if m := speakerLabelNoDashRe.FindString(s); m != "" {
		rest := strings.TrimSpace(s[len(m):])
		if rest != "" {
			s = rest
		}
	}
	s = strings.TrimSpace(s)
	return s
}

// TurnTranscriber transcribes audio with model-based turn detection (tdrz).
// It produces segments directly, unlike the standard Transcriber which
// returns plain text that must be split with SplitDashTurns.
type TurnTranscriber interface {
	TranscribeTurns(ctx context.Context, audio []int16, sampleRate int) ([]Segment, error)
	SetPrompt(prompt string)
	Close() error
}

// tdrzJSON mirrors the JSON output of `whisper-cli -tdrz -oj`.
type tdrzJSON struct {
	Transcription []tdrzSegment `json:"transcription"`
}

type tdrzSegment struct {
	Offsets struct {
		From int64 `json:"from"`
		To   int64 `json:"to"`
	} `json:"offsets"`
	Text            string `json:"text"`
	SpeakerTurnNext *bool  `json:"speaker_turn_next,omitempty"`
}

// ParseTdrzJSON parses the JSON output of `whisper-cli -tdrz -oj` into
// segments. The `speaker_turn_next` flag on segment i sets TurnStart on
// segment i+1. A missing field is treated as false. Text is sanitized
// and empty texts are dropped, with TurnStart carrying over to the next
// non-empty segment.
func ParseTdrzJSON(data []byte) ([]Segment, error) {
	var doc tdrzJSON
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parsing tdrz JSON: %w", err)
	}

	if len(doc.Transcription) == 0 {
		return nil, nil
	}

	// First pass: build raw segments and track turn-next flags.
	type raw struct {
		text      string
		start     time.Duration
		end       time.Duration
		turnStart bool
	}
	raws := make([]raw, 0, len(doc.Transcription))
	pendingTurn := false

	for _, ts := range doc.Transcription {
		text := cleanSegmentText(ts.Text)

		isTurn := pendingTurn
		if text == "" {
			// Carry TurnStart to next non-empty segment.
			if isTurn {
				pendingTurn = true
			}
			// Propagate speaker_turn_next even for empty segments.
			if ts.SpeakerTurnNext != nil && *ts.SpeakerTurnNext {
				pendingTurn = true
			}
			continue
		}

		pendingTurn = false
		raws = append(raws, raw{
			text:      text,
			start:     time.Duration(ts.Offsets.From) * time.Millisecond,
			end:       time.Duration(ts.Offsets.To) * time.Millisecond,
			turnStart: isTurn,
		})

		if ts.SpeakerTurnNext != nil && *ts.SpeakerTurnNext {
			pendingTurn = true
		}
	}

	segs := make([]Segment, len(raws))
	for i, r := range raws {
		segs[i] = Segment{
			Text:      r.text,
			Start:     r.start,
			End:       r.end,
			TurnStart: r.turnStart,
		}
	}
	return segs, nil
}

// ApplyPauseBreak sets segs[0].TurnStart to true when the gap before the
// passage exceeds the pause-break threshold or when it is the first passage
// of the recording.
func ApplyPauseBreak(segs []Segment, gap, threshold time.Duration, first bool) {
	if len(segs) == 0 {
		return
	}
	if first || gap > threshold {
		segs[0].TurnStart = true
	}
}
