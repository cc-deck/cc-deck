package env

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var requiredGitignoreEntries = []string{"status.yaml", "run/"}

// EnsureCCDeckGitignore idempotently creates or updates .cc-deck/.gitignore
// with the required entries (status.yaml and run/).
func EnsureCCDeckGitignore(projectRoot string) error {
	dir := filepath.Join(projectRoot, ".cc-deck")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating .cc-deck directory: %w", err)
	}

	gitignorePath := filepath.Join(dir, ".gitignore")

	existing := ""
	data, err := os.ReadFile(gitignorePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading .cc-deck/.gitignore: %w", err)
	}
	if err == nil {
		existing = string(data)
	}

	lines := strings.Split(existing, "\n")
	present := make(map[string]bool)
	for _, line := range lines {
		present[strings.TrimSpace(line)] = true
	}

	var toAdd []string
	for _, entry := range requiredGitignoreEntries {
		if !present[entry] {
			toAdd = append(toAdd, entry)
		}
	}

	if len(toAdd) == 0 {
		return nil
	}

	content := existing
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += strings.Join(toAdd, "\n") + "\n"

	if err := os.WriteFile(gitignorePath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing .cc-deck/.gitignore: %w", err)
	}

	return nil
}
