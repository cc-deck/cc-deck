package compose

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateTinyproxyConf(t *testing.T) {
	conf := GenerateTinyproxyConf(ProxyConfig{Port: 8888})

	assert.Contains(t, conf, "Port 8888")
	assert.Contains(t, conf, "FilterDefaultDeny Yes")
	assert.Contains(t, conf, "FilterType fnmatch")
	assert.Contains(t, conf, "Filter /etc/tinyproxy/whitelist")
	assert.Contains(t, conf, "FilterURLs On")
	assert.Contains(t, conf, "ConnectPort 443")
}

func TestGenerateTinyproxyConf_DefaultPort(t *testing.T) {
	conf := GenerateTinyproxyConf(ProxyConfig{})
	assert.Contains(t, conf, "Port 8888")
}

func TestGenerateWhitelist(t *testing.T) {
	domains := []string{
		"pypi.org",
		".github.com",
		"api.anthropic.com",
	}

	whitelist := GenerateWhitelist(domains)
	lines := strings.Split(strings.TrimSpace(whitelist), "\n")

	assert.Len(t, lines, 3)
	assert.Equal(t, "pypi.org", lines[0])
	assert.Equal(t, "*.github.com", lines[1])
	assert.Equal(t, "api.anthropic.com", lines[2])
}

func TestGenerateWhitelist_Empty(t *testing.T) {
	assert.Equal(t, "", GenerateWhitelist(nil))
	assert.Equal(t, "", GenerateWhitelist([]string{}))
}

func TestToFnmatchPattern(t *testing.T) {
	assert.Equal(t, "pypi.org", toFnmatchPattern("pypi.org"))
	assert.Equal(t, "*.github.com", toFnmatchPattern(".github.com"))
	assert.Equal(t, "api.anthropic.com", toFnmatchPattern("api.anthropic.com"))
}
