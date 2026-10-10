package share

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

func zellijConfigPath() string {
	if p := os.Getenv("ZELLIJ_CONFIG_FILE"); p != "" {
		return p
	}
	dir := os.Getenv("ZELLIJ_CONFIG_DIR")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config", "zellij")
	}
	return filepath.Join(dir, "config.kdl")
}

// webSharingDisabled matches an active (uncommented) web_sharing "disabled"
// setting. Zellij's other two values both permit sharing: "on" shares every
// session on the machine, and "off" (the default) shares only sessions that
// opt in, which is what cc-deck does when it creates the canonical session.
// Only "disabled" forbids that opt-in, so only "disabled" is a blocker.
var webSharingDisabled = regexp.MustCompile(`(?m)^\s*web_sharing\s+"disabled"`)

// CheckZellijWebSharing refuses to share when the user's Zellij config forbids
// it. It never writes the config: the setting is global, so switching it to
// "on" would expose every session on the machine rather than the one being
// shared, and a user who chose "off" as a safeguard would lose it silently.
// A missing config is fine, because Zellij's default is "off".
func CheckZellijWebSharing(configPath string) error {
	if configPath == "" {
		configPath = zellijConfigPath()
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read Zellij config: %w", err)
	}
	if webSharingDisabled.Match(data) {
		return fmt.Errorf("web sharing is disabled in the Zellij config at %s (web_sharing \"disabled\"); "+
			"change it to \"off\" so that sessions may opt in, then restart Zellij", configPath)
	}
	return nil
}
