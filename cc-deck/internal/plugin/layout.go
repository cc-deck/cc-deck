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

// LayoutVariant identifies a layout style.
type LayoutVariant string

const (
	LayoutMinimal  LayoutVariant = "minimal"  // sidebar + compact-bar (default)
	LayoutStandard LayoutVariant = "standard" // sidebar + tab-bar top + status-bar bottom
	LayoutClean    LayoutVariant = "clean"    // sidebar only, no bars
)

// ValidLayouts returns the list of supported layout variant names.
func ValidLayouts() []string {
	return []string{string(LayoutMinimal), string(LayoutStandard), string(LayoutClean)}
}

// GenerateLayout creates a layout KDL string for the given variant.
func GenerateLayout(pluginsDir string, variant LayoutVariant) string {
	switch variant {
	case LayoutStandard:
		return standardLayout(pluginsDir)
	case LayoutClean:
		return cleanLayout(pluginsDir)
	default:
		return minimalLayout(pluginsDir)
	}
}

// LayoutFilename returns the layout filename for a variant.
func LayoutFilename(variant LayoutVariant) string {
	switch variant {
	case LayoutMinimal:
		return "cc-deck-minimal.kdl"
	case LayoutClean:
		return "cc-deck-clean.kdl"
	default:
		return "cc-deck.kdl"
	}
}

// minimalLayout: sidebar + compact-bar at bottom. The default.
func minimalLayout(pluginsDir string) string {
	sidebar := sidebarPluginBlock(pluginsDir)
	return fmt.Sprintf(`// cc-deck layout: minimal (sidebar + compact-bar)
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

// standardLayout: sidebar + tab-bar at top + status-bar at bottom (beginner-friendly).
func standardLayout(pluginsDir string) string {
	sidebar := sidebarPluginBlock(pluginsDir)
	return fmt.Sprintf(`// cc-deck layout: standard (sidebar + tab-bar top + status-bar bottom)
layout {
    default_tab_template {
        pane size=1 borderless=true {
            plugin location="tab-bar"
        }
        pane split_direction="vertical" {
            %s
            children
        }
        pane size=2 borderless=true {
            plugin location="status-bar"
        }
    }
    new_tab_template {
        pane size=1 borderless=true {
            plugin location="tab-bar"
        }
        pane split_direction="vertical" {
            %s
            pane
        }
        pane size=2 borderless=true {
            plugin location="status-bar"
        }
    }
    tab name="main" focus=true {
        pane
    }
}
`, sidebar, sidebar)
}

// cleanLayout: sidebar only, no bars. Maximum terminal space.
func cleanLayout(pluginsDir string) string {
	sidebar := sidebarPluginBlock(pluginsDir)
	return fmt.Sprintf(`// cc-deck layout: clean (sidebar only, no bars)
layout {
    default_tab_template {
        pane split_direction="vertical" {
            %s
            children
        }
    }
    new_tab_template {
        pane split_direction="vertical" {
            %s
            pane
        }
    }
    tab name="main" focus=true {
        pane
    }
}
`, sidebar, sidebar)
}

// SidebarLayout returns the default (minimal) layout. Kept for backwards compatibility.
func SidebarLayout(pluginsDir string) string {
	return minimalLayout(pluginsDir)
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
