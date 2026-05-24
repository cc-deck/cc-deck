package record

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sampleCoreDNSLog = `[INFO] 10.0.0.2:45678 - 12345 "A IN pypi.org. udp 28 false 512" NOERROR qr,rd,ra 68 0.023s
[INFO] 10.0.0.2:45679 - 12346 "A IN files.pythonhosted.org. udp 28 false 512" NOERROR qr,rd,ra 68 0.015s
[INFO] 10.0.0.2:45680 - 12347 "AAAA IN pypi.org. udp 28 false 512" NOERROR qr,rd,ra 68 0.010s
[INFO] 10.0.0.2:45681 - 12348 "A IN crates.io. udp 28 false 512" NOERROR qr,rd,ra 68 0.020s
[INFO] 10.0.0.2:45682 - 12349 "A IN pypi.org. udp 28 false 512" NOERROR qr,rd,ra 68 0.012s
[INFO] 10.0.0.2:45683 - 12350 "A IN dns.podman. udp 28 false 512" NXDOMAIN qr,rd,ra 68 0.001s
[INFO] 10.0.0.2:45684 - 12351 "A IN localhost. udp 28 false 512" NOERROR qr,rd,ra 68 0.001s
[INFO] 10.0.0.2:45685 - 12352 "A IN _dnssd._udp.local. udp 28 false 512" NXDOMAIN qr,rd,ra 68 0.001s
[INFO] 10.0.0.2:45686 - 12353 "A IN 1.0.168.192.in-addr.arpa. udp 28 false 512" NXDOMAIN qr,rd,ra 68 0.001s
[INFO] 10.0.0.2:45687 - 12354 "AAAA IN ipv6only.example.com. udp 28 false 512" NOERROR qr,rd,ra 68 0.015s
[INFO] 10.0.0.2:45688 - 12355 "A IN service.podman.internal. udp 28 false 512" NXDOMAIN qr,rd,ra 68 0.001s
[INFO] 10.0.0.2:45689 - 12356 "A IN myapp.localhost. udp 28 false 512" NXDOMAIN qr,rd,ra 68 0.001s
`

func TestParseDNSLog(t *testing.T) {
	entries := ParseDNSLog(strings.NewReader(sampleCoreDNSLog))
	require.Len(t, entries, 12, "sample log has 12 parseable INFO lines")

	var domains []string
	for _, e := range entries {
		domains = append(domains, e.Domain)
	}

	assert.Contains(t, domains, "pypi.org")
	assert.Contains(t, domains, "files.pythonhosted.org")
	assert.Contains(t, domains, "crates.io")
	assert.Contains(t, domains, "dns.podman")
	assert.Contains(t, domains, "localhost")
}

func TestParseDNSLog_QueryTypes(t *testing.T) {
	entries := ParseDNSLog(strings.NewReader(sampleCoreDNSLog))

	typeMap := make(map[string][]string)
	for _, e := range entries {
		typeMap[e.Domain] = append(typeMap[e.Domain], e.QueryType)
	}

	assert.Contains(t, typeMap["pypi.org"], "A")
	assert.Contains(t, typeMap["pypi.org"], "AAAA")
}

func TestParseDNSLog_EmptyInput(t *testing.T) {
	entries := ParseDNSLog(strings.NewReader(""))
	assert.Empty(t, entries)
}

func TestParseDNSLog_NonInfoLines(t *testing.T) {
	input := `[WARNING] some warning
[ERROR] something failed
not a log line at all
`
	entries := ParseDNSLog(strings.NewReader(input))
	assert.Empty(t, entries)
}

func TestParseDNSLog_TrailingDotStripped(t *testing.T) {
	input := `[INFO] 10.0.0.2:45678 - 12345 "A IN example.com. udp 28 false 512" NOERROR qr,rd,ra 68 0.023s
`
	entries := ParseDNSLog(strings.NewReader(input))
	require.Len(t, entries, 1)
	assert.Equal(t, "example.com", entries[0].Domain)
}

func TestDeduplicateDomains(t *testing.T) {
	input := []string{"PyPI.org", "crates.io", "pypi.org", "CRATES.IO", "new-domain.com"}
	result := DeduplicateDomains(input)

	assert.Equal(t, []string{"crates.io", "new-domain.com", "pypi.org"}, result)
}

func TestDeduplicateDomains_Empty(t *testing.T) {
	result := DeduplicateDomains(nil)
	assert.Empty(t, result)
}

func TestFilterNoise(t *testing.T) {
	entries := ParseDNSLog(strings.NewReader(sampleCoreDNSLog))
	result := FilterNoise(entries)

	assert.Len(t, result.Domains, 3)
	assert.Contains(t, result.Domains, "pypi.org")
	assert.Contains(t, result.Domains, "files.pythonhosted.org")
	assert.Contains(t, result.Domains, "crates.io")

	assert.NotContains(t, result.Domains, "dns.podman")
	assert.NotContains(t, result.Domains, "localhost")
	assert.NotContains(t, result.Domains, "_dnssd._udp.local")
	assert.NotContains(t, result.Domains, "1.0.168.192.in-addr.arpa")
	assert.NotContains(t, result.Domains, "service.podman.internal")
	assert.NotContains(t, result.Domains, "myapp.localhost")
	assert.NotContains(t, result.Domains, "ipv6only.example.com")

	assert.True(t, result.NoiseCount > 0, "noise count should be positive")
}

func TestFilterNoise_DuplicateEntries(t *testing.T) {
	entries := ParseDNSLog(strings.NewReader(sampleCoreDNSLog))
	result := FilterNoise(entries)

	count := 0
	for _, d := range result.Domains {
		if d == "pypi.org" {
			count++
		}
	}
	assert.Equal(t, 1, count, "pypi.org should appear exactly once")
}

func TestFilterNoise_DomainWithBothAAndAAAA(t *testing.T) {
	input := `[INFO] 10.0.0.2:1 - 1 "A IN example.com. udp 28 false 512" NOERROR qr,rd,ra 68 0.01s
[INFO] 10.0.0.2:2 - 2 "AAAA IN example.com. udp 28 false 512" NOERROR qr,rd,ra 68 0.01s
`
	entries := ParseDNSLog(strings.NewReader(input))
	result := FilterNoise(entries)

	assert.Contains(t, result.Domains, "example.com",
		"domain with both A and AAAA records should NOT be filtered")
}

func TestFilterNoise_EmptyInput(t *testing.T) {
	result := FilterNoise(nil)
	assert.Empty(t, result.Domains)
	assert.Equal(t, 0, result.NoiseCount)
}

func TestFilterNoise_ContainerRegistryDomains(t *testing.T) {
	input := `[INFO] 10.0.0.2:1 - 1 "A IN registry-1.docker.io. udp 28 false 512" NOERROR qr,rd,ra 68 0.01s
[INFO] 10.0.0.2:2 - 2 "A IN auth.docker.io. udp 28 false 512" NOERROR qr,rd,ra 68 0.01s
[INFO] 10.0.0.2:3 - 3 "A IN production.cloudflare.docker.com. udp 28 false 512" NOERROR qr,rd,ra 68 0.01s
`
	entries := ParseDNSLog(strings.NewReader(input))
	result := FilterNoise(entries)

	assert.Contains(t, result.Domains, "registry-1.docker.io")
	assert.Contains(t, result.Domains, "auth.docker.io")
	assert.Contains(t, result.Domains, "production.cloudflare.docker.com")
	assert.Equal(t, 0, result.NoiseCount)
}

func TestFilterNoise_MDNSServiceDiscovery(t *testing.T) {
	input := `[INFO] 10.0.0.2:1 - 1 "A IN _dnssd._udp.local. udp 28 false 512" NXDOMAIN qr,rd,ra 68 0.01s
[INFO] 10.0.0.2:2 - 2 "A IN _http._tcp.local. udp 28 false 512" NXDOMAIN qr,rd,ra 68 0.01s
[INFO] 10.0.0.2:3 - 3 "A IN myprinter.local. udp 28 false 512" NXDOMAIN qr,rd,ra 68 0.01s
`
	entries := ParseDNSLog(strings.NewReader(input))
	result := FilterNoise(entries)

	assert.Empty(t, result.Domains, "all .local domains should be filtered")
	assert.Equal(t, 3, result.NoiseCount)
}

func TestFilterNoise_DuplicateAAndAAAA(t *testing.T) {
	input := `[INFO] 10.0.0.2:1 - 1 "A IN example.com. udp 28 false 512" NOERROR qr,rd,ra 68 0.01s
[INFO] 10.0.0.2:2 - 2 "AAAA IN example.com. udp 28 false 512" NOERROR qr,rd,ra 68 0.01s
[INFO] 10.0.0.2:3 - 3 "A IN example.com. udp 28 false 512" NOERROR qr,rd,ra 68 0.01s
[INFO] 10.0.0.2:4 - 4 "AAAA IN example.com. udp 28 false 512" NOERROR qr,rd,ra 68 0.01s
`
	entries := ParseDNSLog(strings.NewReader(input))
	result := FilterNoise(entries)

	count := 0
	for _, d := range result.Domains {
		if d == "example.com" {
			count++
		}
	}
	assert.Equal(t, 1, count, "duplicate A+AAAA queries should produce single output")
}

func TestFilterNoise_InternalSuffix_KnownLimitation(t *testing.T) {
	input := `[INFO] 10.0.0.2:1 - 1 "A IN api.corp.internal. udp 28 false 512" NOERROR qr,rd,ra 68 0.01s
[INFO] 10.0.0.2:2 - 2 "A IN api.internal.company.com. udp 28 false 512" NOERROR qr,rd,ra 68 0.01s
`
	entries := ParseDNSLog(strings.NewReader(input))
	result := FilterNoise(entries)

	assert.NotContains(t, result.Domains, "api.corp.internal")
	assert.Contains(t, result.Domains, "api.internal.company.com")
}

func TestFilterNoise_ReverseDNS(t *testing.T) {
	input := `[INFO] 10.0.0.2:1 - 1 "A IN 1.0.168.192.in-addr.arpa. udp 28 false 512" NXDOMAIN qr,rd,ra 68 0.01s
[INFO] 10.0.0.2:2 - 2 "A IN 8.b.d.0.1.0.0.2.ip6.arpa. udp 28 false 512" NXDOMAIN qr,rd,ra 68 0.01s
`
	entries := ParseDNSLog(strings.NewReader(input))
	result := FilterNoise(entries)

	assert.Empty(t, result.Domains, "reverse DNS lookups should be filtered")
}

func TestFilterNoise_PodmanDomains(t *testing.T) {
	input := `[INFO] 10.0.0.2:1 - 1 "A IN dns.podman. udp 28 false 512" NXDOMAIN qr,rd,ra 68 0.01s
[INFO] 10.0.0.2:2 - 2 "A IN gateway.podman. udp 28 false 512" NXDOMAIN qr,rd,ra 68 0.01s
`
	entries := ParseDNSLog(strings.NewReader(input))
	result := FilterNoise(entries)

	assert.Empty(t, result.Domains, "all .podman domains should be filtered")
}
