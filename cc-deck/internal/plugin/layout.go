package plugin

import (
	"fmt"
	"strings"
)

const (
	// InjectionStart is the sentinel marker for the start of an injected plugin block.
	InjectionStart = "// cc-deck-plugin-start (managed by cc-deck, do not edit)"

	// InjectionEnd is the sentinel marker for the end of an injected plugin block.
	InjectionEnd = "// cc-deck-plugin-end"
)

// MinimalLayout returns a minimal KDL layout with the cc-deck plugin bar.
func MinimalLayout(pluginsDir string) string {
	return fmt.Sprintf(`layout {
    pane
    pane size=1 borderless=true {
        plugin location="file:%s/cc_deck.wasm"
    }
}
`, pluginsDir)
}

// FullLayout returns an opinionated KDL layout for Claude Code sessions.
// Compared to MinimalLayout it:
//   - Uses default_tab_template so every new tab auto-includes the plugin bar
//   - Keeps the Zellij tab-bar at the top for tab visibility
//   - Sets plugin config (idle_timeout)
//   - Starts with a named "main" tab
func FullLayout(pluginsDir string) string {
	return fmt.Sprintf(`layout {
    default_tab_template {
        pane size=1 borderless=true {
            plugin location="tab-bar"
        }
        children
        pane size=1 borderless=true {
            plugin location="file:%s/cc_deck.wasm" {
                idle_timeout "300"
            }
        }
    }
    tab name="main" focus=true {
        pane
    }
}
`, pluginsDir)
}

// InjectionBlock returns the KDL snippet that gets appended to a default layout.
func InjectionBlock(pluginsDir string) string {
	return fmt.Sprintf(`%s
pane size=1 borderless=true {
    plugin location="file:%s/cc_deck.wasm"
}
%s
`, InjectionStart, pluginsDir, InjectionEnd)
}

// HasInjection returns true if the content contains the cc-deck injection markers.
func HasInjection(content string) bool {
	return strings.Contains(content, InjectionStart) && strings.Contains(content, InjectionEnd)
}

// InjectPlugin appends the cc-deck plugin block to the given layout content.
// If the content already has an injection, it is returned unchanged.
func InjectPlugin(content, pluginsDir string) string {
	if HasInjection(content) {
		return content
	}
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return content + InjectionBlock(pluginsDir)
}

// RemoveInjection removes the cc-deck plugin block from the given layout content.
// If no injection is found, the content is returned unchanged.
func RemoveInjection(content string) string {
	startIdx := strings.Index(content, InjectionStart)
	if startIdx < 0 {
		return content
	}
	endIdx := strings.Index(content, InjectionEnd)
	if endIdx < 0 {
		return content
	}
	endIdx += len(InjectionEnd)

	// Remove trailing newline after the end marker if present
	if endIdx < len(content) && content[endIdx] == '\n' {
		endIdx++
	}

	// Remove leading newline before the start marker if present
	if startIdx > 0 && content[startIdx-1] == '\n' {
		startIdx--
	}

	return content[:startIdx] + content[endIdx:]
}
