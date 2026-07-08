package voice

import (
	"strings"
	"unicode"
)

// Whisper hallucinates these phrases on silence or low-energy audio
// because its training data is dominated by YouTube videos.
var whisperHallucinations = []string{
	"thank you for watching",
	"thanks for watching",
	"please subscribe",
	"like and subscribe",
	"don't forget to subscribe",
	"see you in the next",
	"see you next time",
	"thanks for listening",
	"thank you for listening",
	"please like and subscribe",
	"hit the bell",
	"leave a comment",
	"bye bye",
	"goodbye",
}

var fillerWords = map[string]bool{
	"um":  true,
	"uh":  true,
	"hmm": true,
	"ah":  true,
	"er":  true,
}

// DefaultCommands maps action names to their default trigger words.
var DefaultCommands = map[string][]string{
	"submit":        {"send"},
	"attend":        {"next"},
	"submit_attend": {"ship"},
}

// BuildCommandMap flattens an action-to-words map into a word-to-action
// lookup table for use by ProcessStopwords.
func BuildCommandMap(actions map[string][]string) map[string]string {
	m := make(map[string]string)
	for action, words := range actions {
		for _, w := range words {
			m[strings.ToLower(w)] = action
		}
	}
	return m
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
	if strings.HasPrefix(trimmed, "*") && strings.HasSuffix(trimmed, "*") {
		return true
	}
	if strings.HasPrefix(trimmed, "{") {
		return true
	}
	lower := strings.ToLower(trimmed)
	for _, phrase := range whisperHallucinations {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
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
	if hasRepetitionLoop(lower) {
		return true
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
// The commands map is word -> action (built by BuildCommandMap).
// If commands is nil, DefaultCommands is used.
func ProcessStopwords(text string, commands map[string]string) TranscriptionResult {
	if commands == nil {
		commands = BuildCommandMap(DefaultCommands)
	}

	stripped := stripFillers(text)

	if action, ok := commands[stripped]; ok {
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

// hasRepetitionLoop detects Whisper's repetition hallucination where it
// gets stuck repeating the same short fragment. Splits on sentence
// boundaries, normalizes, and flags if any fragment appears 2+ times.
func hasRepetitionLoop(lower string) bool {
	seps := strings.NewReplacer(".", "\n", "!", "\n", "?", "\n")
	parts := strings.Split(seps.Replace(lower), "\n")

	seen := make(map[string]int)
	for _, p := range parts {
		norm := strings.TrimSpace(p)
		if len(norm) < 3 {
			continue
		}
		seen[norm]++
		if seen[norm] >= 2 {
			return true
		}
	}
	return false
}

func stripFillers(text string) string {
	words := strings.Fields(strings.TrimSpace(text))
	var remaining []string
	for _, w := range words {
		lower := strings.ToLower(w)
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
