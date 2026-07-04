package voice

import (
	"bufio"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// tokenWarningThreshold is the approximate character count at which the merged
// glossary may exceed Whisper's 224-token prompt limit. We log a warning but
// still pass the full list because Whisper truncates from the beginning,
// preserving project-specific terms appended at the end.
const tokenWarningThreshold = 800

// Glossary manages global and per-project term lists that are passed to
// Whisper as the initial prompt to bias recognition toward domain vocabulary.
type Glossary struct {
	globalTerms  []string
	projectCache map[string][]string // dir path -> parsed terms
}

// NewGlossary creates a Glossary pre-loaded with global terms from the
// config file. Project terms are loaded lazily on the first call to
// ResolvePrompt for a given working directory.
func NewGlossary(globalTerms []string) *Glossary {
	return &Glossary{
		globalTerms:  globalTerms,
		projectCache: make(map[string][]string),
	}
}

// LoadFile reads a glossary file at the given path and returns the parsed
// terms. Each non-empty line that does not start with '#' is treated as a
// term. Leading and trailing whitespace is trimmed. Returns nil (not an
// error) when the file does not exist.
func LoadFile(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var terms []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		terms = append(terms, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return terms, nil
}

// ResolvePrompt merges the global glossary with project-specific terms from
// .cc-deck/voice-glossary.txt in workingDir. Global terms come first, project
// terms last (giving them priority in Whisper's 224-token window). Duplicate
// terms are removed case-insensitively, with the last occurrence (project)
// winning. Returns an empty string when no terms are configured or when
// workingDir is empty.
func (g *Glossary) ResolvePrompt(workingDir string) string {
	var projectTerms []string
	if workingDir != "" {
		var ok bool
		projectTerms, ok = g.projectCache[workingDir]
		if !ok {
			glossaryPath := filepath.Join(workingDir, ".cc-deck", "voice-glossary.txt")
			loaded, err := LoadFile(glossaryPath)
			if err != nil {
				log.Printf("[voice] failed to load project glossary %s: %v", glossaryPath, err)
			}
			// Cache even on error (nil) to avoid repeated attempts.
			g.projectCache[workingDir] = loaded
			projectTerms = loaded
		}
	}

	merged := dedup(g.globalTerms, projectTerms)
	if len(merged) == 0 {
		return ""
	}

	prompt := strings.Join(merged, ", ")
	if len(prompt) > tokenWarningThreshold {
		log.Printf("[voice] glossary prompt exceeds %d characters (%d chars, ~%d terms); Whisper may truncate from the beginning",
			tokenWarningThreshold, len(prompt), len(merged))
	}
	return prompt
}

// dedup merges two term slices and removes duplicates case-insensitively.
// When a term appears in both slices with different casing, the second
// slice's casing wins (last occurrence takes precedence).
func dedup(first, second []string) []string {
	seen := make(map[string]int) // lowercase -> index in result
	var result []string

	for _, term := range first {
		lower := strings.ToLower(term)
		if _, exists := seen[lower]; !exists {
			seen[lower] = len(result)
			result = append(result, term)
		}
	}
	for _, term := range second {
		lower := strings.ToLower(term)
		if idx, exists := seen[lower]; exists {
			// Project casing wins: replace the earlier occurrence.
			result[idx] = term
		} else {
			seen[lower] = len(result)
			result = append(result, term)
		}
	}
	return result
}
