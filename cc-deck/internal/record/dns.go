package record

import (
	"bufio"
	"io"
	"regexp"
	"sort"
	"strings"
)

// DNSLogEntry represents a single parsed DNS query from the CoreDNS log.
type DNSLogEntry struct {
	Domain    string
	QueryType string
}

// queryRe matches the quoted query section of a CoreDNS log line.
// Example: [INFO] 10.0.0.2:45678 - 12345 "A IN pypi.org. udp 28 false 512" NOERROR ...
var queryRe = regexp.MustCompile(`"(\w+)\s+IN\s+(\S+?)\.?\s+`)

// ParseDNSLog reads a CoreDNS log and extracts DNS query entries.
func ParseDNSLog(r io.Reader) []DNSLogEntry {
	var entries []DNSLogEntry
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, "[INFO]") {
			continue
		}

		matches := queryRe.FindStringSubmatch(line)
		if len(matches) < 3 {
			continue
		}

		qtype := matches[1]
		domain := strings.TrimRight(matches[2], ".")

		if domain == "" {
			continue
		}

		entries = append(entries, DNSLogEntry{
			Domain:    domain,
			QueryType: qtype,
		})
	}

	return entries
}

// DeduplicateDomains returns a sorted, unique list of domain names
// using case-insensitive comparison.
func DeduplicateDomains(domains []string) []string {
	seen := make(map[string]bool, len(domains))
	var result []string

	for _, d := range domains {
		lower := strings.ToLower(d)
		if !seen[lower] {
			seen[lower] = true
			result = append(result, lower)
		}
	}

	sort.Strings(result)
	return result
}

// FilterResult holds the output of noise filtering with separate counts.
type FilterResult struct {
	Domains      []string
	NoiseCount   int
}

// FilterNoise removes infrastructure noise domains from the list.
// Filtered patterns: .local, .internal, .podman, .localhost suffixes;
// exact "localhost"; reverse DNS (*.in-addr.arpa, *.ip6.arpa);
// AAAA-only queries (no corresponding A record for the same domain).
func FilterNoise(entries []DNSLogEntry) FilterResult {
	hasA := make(map[string]bool)
	hasAAAA := make(map[string]bool)

	for _, e := range entries {
		lower := strings.ToLower(e.Domain)
		switch e.QueryType {
		case "A":
			hasA[lower] = true
		case "AAAA":
			hasAAAA[lower] = true
		}
	}

	seen := make(map[string]bool)
	var domains []string
	noiseCount := 0

	for _, e := range entries {
		lower := strings.ToLower(e.Domain)
		if seen[lower] {
			continue
		}
		seen[lower] = true

		if isNoiseDomain(lower) {
			noiseCount++
			continue
		}

		if hasAAAA[lower] && !hasA[lower] {
			noiseCount++
			continue
		}

		domains = append(domains, lower)
	}

	return FilterResult{Domains: domains, NoiseCount: noiseCount}
}

var noiseSuffixes = []string{
	".local",
	".internal",
	".podman",
	".localhost",
	".in-addr.arpa",
	".ip6.arpa",
}

func isNoiseDomain(domain string) bool {
	if domain == "localhost" {
		return true
	}
	for _, suffix := range noiseSuffixes {
		if strings.HasSuffix(domain, suffix) {
			return true
		}
	}
	return false
}
