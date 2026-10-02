package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractTimestamp(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{
			name: "typical tinyproxy notice line",
			line: `NOTICE    Mar 19 18:31:08.838 [1]: Proxying refused on filtered domain "evil-server.com"`,
			want: "Mar 19 18:31:08.838",
		},
		{
			name: "no bracket present",
			line: "no timestamp here at all",
			want: "",
		},
		{
			name: "bracket with no leading level word",
			line: "[1]: something",
			want: "",
		},
		{
			name: "extra whitespace around level",
			line: "WARNING   Jan  1 00:00:00.000 [42]: blocked",
			want: "Jan  1 00:00:00.000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, extractTimestamp(tt.line))
		})
	}
}

func TestExtractQuoted(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{
			name: "single quoted substring",
			line: `NOTICE [1]: Proxying refused on filtered domain "evil-server.com"`,
			want: "evil-server.com",
		},
		{
			name: "no quotes present",
			line: "no quotes here",
			want: "",
		},
		{
			name: "only one quote",
			line: `unterminated "quote`,
			want: "",
		},
		{
			name: "empty quoted string",
			line: `value is ""`,
			want: "",
		},
		{
			name: "multiple quoted segments returns first",
			line: `"first" and "second"`,
			want: "first",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, extractQuoted(tt.line))
		})
	}
}
