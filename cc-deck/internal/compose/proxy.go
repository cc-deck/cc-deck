package compose

import (
	"fmt"
	"strings"
)

// DefaultProxyPort is the default port for the tinyproxy sidecar.
const DefaultProxyPort = 8888

// ProxyConfig holds the configuration for generating tinyproxy config files.
type ProxyConfig struct {
	Port    int
	Domains []string
	LogPath string
}

// GenerateTinyproxyConf generates the tinyproxy.conf content for allowlist-mode filtering.
func GenerateTinyproxyConf(cfg ProxyConfig) string {
	port := cfg.Port
	if port == 0 {
		port = DefaultProxyPort
	}

	logPath := cfg.LogPath
	if logPath == "" {
		logPath = "/var/log/tinyproxy/tinyproxy.log"
	}

	return fmt.Sprintf(`Port %d
Timeout 600
MaxClients 100
FilterDefaultDeny Yes
FilterType fnmatch
Filter /etc/tinyproxy/whitelist
FilterURLs On
ConnectPort 443
ConnectPort 563
LogLevel Info
LogFile %s
`, port, logPath)
}

// GenerateWhitelist generates the tinyproxy whitelist file from domain patterns.
// Converts cc-deck wildcard notation (".example.com") to fnmatch ("*.example.com").
func GenerateWhitelist(domains []string) string {
	if len(domains) == 0 {
		return ""
	}

	var lines []string
	for _, d := range domains {
		lines = append(lines, toFnmatchPattern(d))
	}
	return strings.Join(lines, "\n") + "\n"
}

// toFnmatchPattern converts a domain pattern to tinyproxy fnmatch format.
// ".example.com" becomes "*.example.com" (wildcard subdomain match).
// "example.com" stays as "example.com" (exact match).
func toFnmatchPattern(domain string) string {
	if strings.HasPrefix(domain, ".") {
		return "*" + domain
	}
	return domain
}
