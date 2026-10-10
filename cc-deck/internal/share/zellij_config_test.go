package share

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// The check is read-only. Zellij's "off" default already lets a session opt
// in, which is how cc-deck creates the canonical session, so the only value
// that blocks sharing is "disabled". Rewriting the global setting to "on"
// would have shared every session on the machine, and once produced a second
// web_sharing node above a user's deliberate "off".
func TestCheckZellijWebSharing(t *testing.T) {
	cases := map[string]struct {
		content string
		wantErr bool
	}{
		"on":                 {content: "web_sharing \"on\"\n"},
		"off is the default": {content: "web_sharing \"off\"\n"},
		"unset":              {content: "theme \"default\"\n"},
		"commented disabled": {content: "// web_sharing \"disabled\"\n"},
		"disabled":           {content: "theme \"default\"\nweb_sharing \"disabled\"\n", wantErr: true},
		"indented disabled":  {content: "    web_sharing \"disabled\" // locked down\n", wantErr: true},
		"off after a commented on": {
			content: "// web_sharing \"on\"  // re-enabled once\nweb_sharing \"off\"  // defence in depth\n",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "config.kdl")
			require.NoError(t, os.WriteFile(p, []byte(tc.content), 0o600))

			err := CheckZellijWebSharing(p)

			if tc.wantErr {
				require.ErrorContains(t, err, "disabled")
				require.ErrorContains(t, err, p)
			} else {
				require.NoError(t, err)
			}
			after, readErr := os.ReadFile(p)
			require.NoError(t, readErr)
			require.Equal(t, tc.content, string(after), "the config must never be rewritten")
		})
	}
}

func TestCheckZellijWebSharingTreatsAMissingConfigAsTheDefault(t *testing.T) {
	p := filepath.Join(t.TempDir(), "missing.kdl")
	require.NoError(t, CheckZellijWebSharing(p))
	require.NoFileExists(t, p)
}
