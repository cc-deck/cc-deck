package ws

import (
	"fmt"
	"regexp"
)

const maxWsNameLength = 40

var wsNameRegex = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

// ValidateWsName checks that name conforms to the workspace naming rules:
//   - must match ^[a-z0-9]([a-z0-9-]*[a-z0-9])?$
//   - maximum 40 characters
//
// Returns nil on success or an error wrapping ErrInvalidName with a
// descriptive message.
func ValidateWsName(name string) error {
	if name == "" {
		return fmt.Errorf("name must not be empty: %w", ErrInvalidName)
	}
	if len(name) > maxWsNameLength {
		return fmt.Errorf("name %q exceeds maximum length of %d characters: %w",
			name, maxWsNameLength, ErrInvalidName)
	}
	if !wsNameRegex.MatchString(name) {
		return fmt.Errorf("name %q must contain only lowercase letters, digits, and hyphens, "+
			"and must start and end with a letter or digit: %w", name, ErrInvalidName)
	}
	return nil
}
