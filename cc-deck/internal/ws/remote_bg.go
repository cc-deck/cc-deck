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

// LoadRemoteBG reads remote-bg from the central definition store.
func LoadRemoteBG(name string, defs *DefinitionStore) string {
	if defs != nil {
		if def, err := defs.FindByName(name); err == nil {
			return def.RemoteBG
		}
	}
	return ""
}
