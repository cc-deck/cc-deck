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
}

// GenerateTinyproxyConf generates the tinyproxy.conf content for allowlist-mode filtering.
func GenerateTinyproxyConf(cfg ProxyConfig) string {
	port := cfg.Port
	if port == 0 {
		port = DefaultProxyPort
	}

	return fmt.Sprintf(`Port %d
Timeout 600
MaxClients 100
FilterExtended On
FilterDefaultDeny Yes
Filter "/etc/tinyproxy/whitelist"
ConnectPort 443
ConnectPort 563
LogLevel Info
`, port)
}

// GenerateWhitelist generates the tinyproxy whitelist file from domain patterns.
// Converts cc-deck wildcard notation (".example.com") to fnmatch ("*.example.com").
func GenerateWhitelist(domains []string) string {
	if len(domains) == 0 {
		return ""
	}

	var lines []string
	for _, d := range domains {
		lines = append(lines, ToRegexPattern(d))
	}
	return strings.Join(lines, "\n") + "\n"
}

// toRegexPattern converts a domain pattern to a POSIX extended regex for tinyproxy.
// ".example.com" becomes "(^|\.)example\.com$" (matches subdomains).
// "example.com" becomes "(^|\.)example\.com$" (matches exact and subdomains).
func ToRegexPattern(domain string) string {
	d := strings.TrimPrefix(domain, ".")
	escaped := strings.ReplaceAll(d, ".", `\.`)
	return `(^|\.)` + escaped + `$`
}
