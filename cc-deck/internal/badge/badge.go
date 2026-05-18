package badge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cc-deck/cc-deck/internal/config"
)

// Evaluate resolves all badge rules against the given working directory.
// Returns a list of emoji strings for badges that matched. Errors are
// silently ignored (missing files, bad JSON, unmatched paths).
func Evaluate(rules []config.BadgeRule, workingDir string) []string {
	if len(rules) == 0 || workingDir == "" {
		return nil
	}

	var badges []string
	for _, rule := range rules {
		if rule.File == "" || rule.Format != "json" || rule.Extract == "" {
			continue
		}
		if emoji := evaluateRule(rule, workingDir); emoji != "" {
			badges = append(badges, emoji)
		}
	}
	return badges
}

func evaluateRule(rule config.BadgeRule, workingDir string) string {
	path := filepath.Join(workingDir, rule.File)
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	var doc interface{}
	if err := json.Unmarshal(data, &doc); err != nil {
		return ""
	}

	val := extractDotPath(doc, rule.Extract)
	if val == "" {
		if rule.Default != "" {
			return rule.Default
		}
		return ""
	}

	if emoji, ok := rule.Values[val]; ok {
		return emoji
	}
	if rule.Default != "" {
		return rule.Default
	}
	return ""
}

func extractDotPath(doc interface{}, path string) string {
	path = strings.TrimPrefix(path, ".")
	if path == "" {
		return ""
	}

	segments := strings.Split(path, ".")
	current := doc

	for _, seg := range segments {
		m, ok := current.(map[string]interface{})
		if !ok {
			return ""
		}
		current, ok = m[seg]
		if !ok {
			return ""
		}
	}

	switch v := current.(type) {
	case string:
		return v
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", v)
	}
}
