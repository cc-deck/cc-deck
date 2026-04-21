package ws

import (
	"fmt"
	"os"
)

// SetRemoteBG emits the OSC 11 escape to set the terminal background color.
func SetRemoteBG(color string) {
	if color != "" {
		fmt.Fprintf(os.Stdout, "\033]11;%s\a", color)
	}
}

// ResetBGEscape returns the OSC 111 escape sequence to reset the terminal background.
const ResetBGEscape = "\033]111\a"

// LoadRemoteBG reads remote-bg from the project-local workspace definition
// or the global definition store.
func LoadRemoteBG(name string, defs *DefinitionStore) string {
	cwd, cwdErr := os.Getwd()
	if cwdErr == nil {
		if projDef, projErr := LoadProjectDefinition(cwd); projErr == nil && projDef.RemoteBG != "" {
			return projDef.RemoteBG
		}
	}
	if defs != nil {
		if globalDef, globalErr := defs.FindByName(name); globalErr == nil {
			return globalDef.RemoteBG
		}
	}
	return ""
}
