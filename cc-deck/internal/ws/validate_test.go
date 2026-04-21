package ws

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateWsName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		// Valid names
		{name: "single char", input: "a", wantErr: false},
		{name: "two chars", input: "ab", wantErr: false},
		{name: "hyphenated", input: "my-env", wantErr: false},
		{name: "alphanumeric with hyphens", input: "a1-b2", wantErr: false},
		{name: "all digits", input: "123", wantErr: false},
		{name: "max length", input: strings.Repeat("a", 40), wantErr: false},

		// Invalid names
		{name: "uppercase", input: "A", wantErr: true},
		{name: "underscore", input: "my_env", wantErr: true},
		{name: "starts with hyphen", input: "-start", wantErr: true},
		{name: "ends with hyphen", input: "end-", wantErr: true},
		{name: "exceeds max length", input: "too-long-name-exceeding-forty-characters-limit", wantErr: true},
		{name: "empty string", input: "", wantErr: true},
		{name: "contains spaces", input: "my env", wantErr: true},
		{name: "mixed case", input: "MyEnv", wantErr: true},
		{name: "special characters", input: "my.env", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWsName(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateWsName(%q) = nil, want error", tt.input)
				} else if !errors.Is(err, ErrInvalidName) {
					t.Errorf("ValidateWsName(%q) error = %v, want wrapping ErrInvalidName", tt.input, err)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateWsName(%q) = %v, want nil", tt.input, err)
				}
			}
		})
	}
}
