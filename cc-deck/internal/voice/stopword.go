package voice

import "strings"

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
