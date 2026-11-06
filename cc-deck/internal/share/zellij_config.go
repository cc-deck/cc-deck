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

var (
	// activeWebSharing matches an uncommented web_sharing node with any string value.
	activeWebSharing = regexp.MustCompile(`(?m)^([ \t]*)web_sharing[ \t]+"([^"]*)"(.*)$`)
	// commentedWebSharing matches a commented-out web_sharing example.
	commentedWebSharing = regexp.MustCompile(`(?m)^([ \t]*)//[ \t]*web_sharing\b[ \t]*(?:"[^"]*")?(.*)$`)
)

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

	var updated string
	if matches := activeWebSharing.FindAllStringSubmatch(content, -1); len(matches) > 0 {
		allOn := true
		for _, m := range matches {
			if m[2] != "on" {
				allOn = false
				break
			}
		}
		if allOn {
			return false, nil
		}
		// Rewrite active values in place so no duplicate node is appended.
		updated = activeWebSharing.ReplaceAllString(content, `${1}web_sharing "on"${3}`)
	} else if loc := commentedWebSharing.FindStringIndex(content); loc != nil {
		// Uncomment only the first commented example.
		line := commentedWebSharing.ReplaceAllString(content[loc[0]:loc[1]], `${1}web_sharing "on"${2}`)
		updated = content[:loc[0]] + line + content[loc[1]:]
	} else {
		updated = content + "\nweb_sharing \"on\"\n"
	}
	if err := writeZellijConfig(configPath, []byte(updated)); err != nil {
		return false, fmt.Errorf("enable web_sharing in Zellij config: %w", err)
	}
	return true, nil
}

// writeZellijConfig replaces the config atomically and keeps its file mode.
// Symlinks (for example from a dotfile manager) are resolved so the link stays intact.
func writeZellijConfig(configPath string, content []byte) error {
	if resolved, err := filepath.EvalSymlinks(configPath); err == nil {
		configPath = resolved
	}
	mode := os.FileMode(0644)
	if info, err := os.Stat(configPath); err == nil {
		mode = info.Mode().Perm()
	}
	f, err := os.CreateTemp(filepath.Dir(configPath), filepath.Base(configPath)+".*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err := f.Chmod(mode); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(content); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, configPath)
}
