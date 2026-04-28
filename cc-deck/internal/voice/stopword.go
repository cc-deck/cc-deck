package voice

import (
	"strings"
	"unicode"
)

var fillerWords = map[string]bool{
	"um":  true,
	"uh":  true,
	"hmm": true,
	"ah":  true,
	"er":  true,
}

var commandWords = map[string]string{
	"submit": "submit",
	"enter":  "enter",
}

// IsWhisperArtifact returns true if the text is a non-speech transcription
// artifact from Whisper (background noise descriptions, blank audio markers,
// music notation, or empty JSON responses).
func IsWhisperArtifact(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return true
	}
	if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
		return true
	}
	if strings.HasPrefix(trimmed, "(") && strings.HasSuffix(trimmed, ")") {
		return true
	}
	if strings.HasPrefix(trimmed, "{") {
		return true
	}
	lower := strings.ToLower(trimmed)
	for _, pattern := range []string{"blank_audio", "music", "clicking", "applause", "laughter", "silence"} {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	for _, r := range trimmed {
		if r == '♪' || r == '♫' || r == '🎵' {
			return true
		}
	}
	stripped := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) {
			return -1
		}
		return r
	}, trimmed)
	if stripped == "" {
		return true
	}
	return false
}

// ProcessStopwords analyzes transcription text for command words.
// A command word is "standalone" when the entire text, after trimming
// whitespace and removing filler words, equals exactly the command word.
func ProcessStopwords(text string) TranscriptionResult {
	stripped := stripFillers(text)

	if action, ok := commandWords[stripped]; ok {
		return TranscriptionResult{
			Text:          text,
			IsCommand:     true,
			CommandAction: action,
		}
	}

	return TranscriptionResult{
		Text:      text,
		IsCommand: false,
	}
}

func stripFillers(text string) string {
	words := strings.Fields(strings.TrimSpace(text))
	var remaining []string
	for _, w := range words {
		lower := strings.ToLower(w)
		// Strip punctuation for matching
		cleaned := strings.TrimRight(lower, ".,!?;:")
		if !fillerWords[cleaned] {
			remaining = append(remaining, cleaned)
		}
	}
	if len(remaining) == 1 {
		return remaining[0]
	}
	return strings.Join(remaining, " ")
}
