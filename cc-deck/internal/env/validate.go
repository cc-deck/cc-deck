package env

import (
	"fmt"
	"regexp"
)

const maxEnvNameLength = 40

var envNameRegex = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

// ValidateEnvName checks that name conforms to the environment naming rules:
//   - must match ^[a-z0-9]([a-z0-9-]*[a-z0-9])?$
//   - maximum 40 characters
//
// Returns nil on success or an error wrapping ErrInvalidName with a
// descriptive message.
func ValidateEnvName(name string) error {
	if name == "" {
		return fmt.Errorf("name must not be empty: %w", ErrInvalidName)
	}
	if len(name) > maxEnvNameLength {
		return fmt.Errorf("name %q exceeds maximum length of %d characters: %w",
			name, maxEnvNameLength, ErrInvalidName)
	}
	if !envNameRegex.MatchString(name) {
		return fmt.Errorf("name %q must contain only lowercase letters, digits, and hyphens, "+
			"and must start and end with a letter or digit: %w", name, ErrInvalidName)
	}
	return nil
}
