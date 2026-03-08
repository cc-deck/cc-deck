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

// sidebarPluginBlock returns the KDL snippet for the sidebar plugin pane.
func sidebarPluginBlock(pluginsDir string) string {
	return fmt.Sprintf(`pane size=22 borderless=true {
                plugin location="file:%s/cc_deck.wasm" {
                    mode "sidebar"
                }
            }`, pluginsDir)
}

// SidebarLayout returns a KDL layout with the cc-deck sidebar on every tab.
// Uses default_tab_template for layout-defined tabs and new_tab_template for
// dynamically created tabs (zellij action new-tab). The difference: default_tab_template
// uses "children" placeholder, new_tab_template uses explicit "pane".
func SidebarLayout(pluginsDir string) string {
	sidebar := sidebarPluginBlock(pluginsDir)
	return fmt.Sprintf(`// cc-deck layout (managed by cc-deck install)
layout {
    default_tab_template {
        pane split_direction="vertical" {
            %s
            children
        }
        pane size=1 borderless=true {
            plugin location="compact-bar"
        }
    }
    new_tab_template {
        pane split_direction="vertical" {
            %s
            pane
        }
        pane size=1 borderless=true {
            plugin location="compact-bar"
        }
    }
    tab name="main" focus=true {
        pane
    }
}
`, sidebar, sidebar)
}

// MinimalLayout returns a minimal KDL layout with the cc-deck plugin bar.
// Kept for backwards compatibility.
func MinimalLayout(pluginsDir string) string {
	return SidebarLayout(pluginsDir)
}

// FullLayout returns the sidebar layout (same as SidebarLayout in v2).
func FullLayout(pluginsDir string) string {
	return SidebarLayout(pluginsDir)
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

	if endIdx < len(content) && content[endIdx] == '\n' {
		endIdx++
	}
	if startIdx > 0 && content[startIdx-1] == '\n' {
		startIdx--
	}

	return content[:startIdx] + content[endIdx:]
}
