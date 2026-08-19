package share

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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

// EnsureZellijWebSharing checks the Zellij config.kdl for web_sharing "on"
// and enables it if needed. Returns whether the config was modified.
func EnsureZellijWebSharing(configPath string) (bool, error) {
	if configPath == "" {
		configPath = zellijConfigPath()
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, fmt.Errorf("Zellij config not found at %s; create it with: zellij setup --dump-config > %s", configPath, configPath)
		}
		return false, fmt.Errorf("read Zellij config: %w", err)
	}
	content := string(data)

	uncommented := regexp.MustCompile(`(?m)^\s*web_sharing\s+"on"`)
	if uncommented.MatchString(content) {
		return false, nil
	}

	commented := regexp.MustCompile(`(?m)^(\s*)//\s*web_sharing\s+"on"(.*)$`)
	if commented.MatchString(content) {
		updated := commented.ReplaceAllString(content, `${1}web_sharing "on"${2}`)
		if err := os.WriteFile(configPath, []byte(updated), 0644); err != nil {
			return false, fmt.Errorf("enable web_sharing in Zellij config: %w", err)
		}
		return true, nil
	}

	var insertion string
	if strings.Contains(content, "// web_sharing") {
		re := regexp.MustCompile(`(?m)^(\s*)//\s*web_sharing\b.*$`)
		insertion = re.ReplaceAllString(content, "${1}web_sharing \"on\"")
	} else {
		insertion = content + "\nweb_sharing \"on\"\n"
	}
	if err := os.WriteFile(configPath, []byte(insertion), 0644); err != nil {
		return false, fmt.Errorf("add web_sharing to Zellij config: %w", err)
	}
	return true, nil
}
