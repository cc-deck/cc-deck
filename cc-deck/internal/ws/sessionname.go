package ws

import (
	"fmt"
	"slices"
	"strings"
)

// parseZellijSessionNames extracts the session names from the output of
// "zellij list-sessions -n", skipping blank lines and sessions marked as
// exited. The name is the first whitespace-delimited field on each line.
func parseZellijSessionNames(out string) []string {
	var names []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "(EXITED") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		names = append(names, fields[0])
	}
	return names
}

// sessionRenameHint returns a recovery hint when live Zellij sessions exist but
// none carries the canonical name. This happens for workspaces created before
// cc-deck adopted canonical "cc-deck-<name>" session names: an attach targeting
// the canonical name would fail against the pre-existing differently named
// session. The hint tells the user how to recover. It returns "" when no hint
// is warranted (no sessions, or the canonical session is already present).
func sessionRenameHint(workspace, canonical string, names []string) string {
	if len(names) == 0 || slices.Contains(names, canonical) {
		return ""
	}
	return fmt.Sprintf(
		"a Zellij session already exists under a different name (%s); this workspace predates cc-deck's canonical session naming. Run `cc-deck ws kill-session %s` and reattach.",
		strings.Join(names, ", "), workspace)
}
