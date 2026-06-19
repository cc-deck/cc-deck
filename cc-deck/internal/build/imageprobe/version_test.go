package imageprobe

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input   string
		want    Version
		wantOK  bool
	}{
		{"2.43.0", Version{2, 43, 0}, true},
		{"Python 3.12.4", Version{3, 12, 4}, true},
		{"go version go1.22.5 linux/amd64", Version{1, 22, 5}, true},
		{"jq-1.7", Version{1, 7, 0}, true},
		{"3.12", Version{3, 12, 0}, true},
		{"git version 2.43.0", Version{2, 43, 0}, true},
		{"curl 8.2.1 (x86_64)", Version{8, 2, 1}, true},
		{"no version here", Version{}, false},
		{"", Version{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := ParseVersion(tt.input)
			assert.Equal(t, tt.wantOK, ok)
			if ok {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestIsCompatible(t *testing.T) {
	tests := []struct {
		name       string
		installed  string
		required   string
		compatible bool
	}{
		{"same version", "2.43.0", "2.43.0", true},
		{"newer minor", "3.14.0", "3.12.0", true},
		{"older minor", "1.22.0", "1.25.0", false},
		{"different major", "3.12.0", "2.12.0", false},
		{"empty installed", "", "3.12.0", true},
		{"empty required", "3.12.0", "", true},
		{"both empty", "", "", true},
		{"unparseable installed", "unknown", "3.12.0", true},
		{"unparseable required", "3.12.0", "latest", true},
		{"newer patch same minor", "2.43.5", "2.43.0", true},
		{"older patch same minor", "2.43.0", "2.43.5", true},
		{"minor exactly equal", "1.25.0", "1.25.0", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.compatible, IsCompatible(tt.installed, tt.required))
		})
	}
}

func TestVersionString(t *testing.T) {
	v := Version{Major: 1, Minor: 22, Patch: 5}
	assert.Equal(t, "1.22.5", v.String())
}
