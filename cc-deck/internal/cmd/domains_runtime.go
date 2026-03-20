package cmd

import (
	"bufio"
	"fmt"
	"os/exec"
	"strings"

	"github.com/cc-deck/cc-deck/internal/compose"
	"github.com/spf13/cobra"
)

func newDomainsBlockedCmd() *cobra.Command {
	var since string

	cmd := &cobra.Command{
		Use:   "blocked <session>",
		Short: "Show blocked requests from proxy logs (Podman only)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDomainsBlocked(args[0], since)
		},
	}

	cmd.Flags().StringVar(&since, "since", "1h", "Show blocks from the last N duration")
	return cmd
}

func runDomainsBlocked(sessionName, since string) error {
	// Locate the proxy container via compose project name convention
	proxyContainer := sessionName + "-proxy"

	// Fetch proxy logs via podman logs (tinyproxy logs to stdout in foreground mode)
	out, err := exec.Command("podman", "logs", "--since", since, proxyContainer).CombinedOutput()
	if err != nil {
		return fmt.Errorf("fetching proxy logs for %q: %w\n%s", proxyContainer, err, string(out))
	}

	// Parse tinyproxy logs for refused entries
	// Format: NOTICE    Mar 19 18:31:08.838 [1]: Proxying refused on filtered domain "evil-server.com"
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	fmt.Printf("%-28s %s\n", "TIMESTAMP", "DOMAIN")

	found := false
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, "Proxying refused on filtered domain") {
			continue
		}
		// Extract timestamp and domain
		timestamp := extractTimestamp(line)
		domain := extractQuoted(line)
		found = true
		fmt.Printf("%-28s %s\n", timestamp, domain)
	}

	if !found {
		fmt.Println("No blocked requests found.")
	}

	return nil
}

func newDomainsAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <session> <domain-or-group>",
		Short: "Add a domain to a running Podman session",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDomainsModify(args[0], args[1], true)
		},
	}
}

func newDomainsRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <session> <domain-or-group>",
		Short: "Remove a domain from a running Podman session",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDomainsModify(args[0], args[1], false)
		},
	}
}

func runDomainsModify(sessionName, domainOrGroup string, add bool) error {
	proxyContainer := sessionName + "-proxy"

	// Read current whitelist from the proxy container
	out, err := exec.Command("podman", "exec", proxyContainer, "cat", "/etc/tinyproxy/whitelist").CombinedOutput()
	if err != nil {
		return fmt.Errorf("reading whitelist from %q: %w\n%s", proxyContainer, err, string(out))
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")

	// Convert domain to the regex pattern used in the whitelist
	pattern := compose.ToRegexPattern(domainOrGroup)

	if add {
		// Check if already present
		for _, l := range lines {
			if strings.TrimSpace(l) == pattern {
				return fmt.Errorf("domain %q is already in the whitelist", domainOrGroup)
			}
		}
		lines = append(lines, pattern)
	} else {
		var newLines []string
		found := false
		for _, l := range lines {
			if strings.TrimSpace(l) == pattern {
				found = true
				continue
			}
			newLines = append(newLines, l)
		}
		if !found {
			return fmt.Errorf("domain %q not found in the whitelist", domainOrGroup)
		}
		lines = newLines
	}

	// Write updated whitelist back to the container
	newWhitelist := strings.Join(lines, "\n") + "\n"
	writeCmd := exec.Command("podman", "exec", "-i", proxyContainer, "sh", "-c", "cat > /etc/tinyproxy/whitelist")
	writeCmd.Stdin = strings.NewReader(newWhitelist)
	if out, err := writeCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("writing whitelist to %q: %w\n%s", proxyContainer, err, string(out))
	}

	// Restart the proxy to pick up the new whitelist
	restartOut, err := exec.Command("podman", "restart", proxyContainer).CombinedOutput()
	if err != nil {
		return fmt.Errorf("restarting proxy %q: %w\n%s", proxyContainer, err, string(restartOut))
	}

	action := "Added"
	if !add {
		action = "Removed"
	}
	fmt.Printf("%s %q. Proxy restarted.\n", action, domainOrGroup)
	return nil
}

// extractTimestamp extracts the timestamp from a tinyproxy log line.
// Example: "NOTICE    Mar 19 18:31:08.838 [1]: ..." -> "Mar 19 18:31:08.838"
func extractTimestamp(line string) string {
	// Skip the log level prefix (e.g., "NOTICE    ")
	idx := strings.Index(line, "[")
	if idx < 0 {
		return ""
	}
	// Timestamp is between the level whitespace and the "[" bracket
	part := strings.TrimSpace(line[:idx])
	// Remove the log level word
	if space := strings.IndexByte(part, ' '); space >= 0 {
		return strings.TrimSpace(part[space:])
	}
	return part
}

// extractQuoted extracts the first double-quoted string from a line.
func extractQuoted(line string) string {
	start := strings.IndexByte(line, '"')
	if start < 0 {
		return ""
	}
	end := strings.IndexByte(line[start+1:], '"')
	if end < 0 {
		return ""
	}
	return line[start+1 : start+1+end]
}
