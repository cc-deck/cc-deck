package record

import (
	"io/fs"
	"os"
	"strings"

	"github.com/cc-deck/cc-deck/internal/build"
)

// BuildDomainIndex loads all catalog components (embedded, cached, user-local)
// and builds a map from endpoint host to component name for reverse lookup.
func BuildDomainIndex(catalogFS fs.FS, catalogRoot string, userLocalFS fs.FS, userLocalRoot string) map[string]string {
	index := make(map[string]string)

	embedded, _ := build.LoadEmbeddedComponents()
	for _, comp := range embedded {
		for _, ep := range comp.Endpoints {
			index[strings.ToLower(ep.Host)] = comp.Name
		}
	}

	if catalogFS != nil {
		catalog, _ := build.LoadComponentTier(catalogFS, catalogRoot)
		for _, comp := range catalog {
			for _, ep := range comp.Endpoints {
				index[strings.ToLower(ep.Host)] = comp.Name
			}
		}
	}

	if userLocalFS != nil {
		userLocal, _ := build.LoadComponentTier(userLocalFS, userLocalRoot)
		for _, comp := range userLocal {
			for _, ep := range comp.Endpoints {
				index[strings.ToLower(ep.Host)] = comp.Name
			}
		}
	}

	return index
}

// MatchAgainstCatalog classifies observed domains as either covered
// (by catalog components or existing allowed_domains) or new.
func MatchAgainstCatalog(result *RecordingResult, manifest *build.Manifest, catalogDir, userLocalDir string) *RecordingResult {
	var catalogFS fs.FS
	var catalogRoot string
	if catalogDir != "" {
		if info, err := os.Stat(catalogDir); err == nil && info.IsDir() {
			catalogFS = os.DirFS(catalogDir)
			catalogRoot = "."
		}
	}

	var userLocalFS fs.FS
	var userLocalRoot string
	if userLocalDir != "" {
		if info, err := os.Stat(userLocalDir); err == nil && info.IsDir() {
			userLocalFS = os.DirFS(userLocalDir)
			userLocalRoot = "."
		}
	}

	domainIndex := BuildDomainIndex(catalogFS, catalogRoot, userLocalFS, userLocalRoot)

	existingDomains := make(map[string]bool)
	if manifest.Network != nil {
		for _, d := range manifest.Network.AllowedDomains {
			existingDomains[strings.ToLower(d)] = true
		}
	}

	var covered []CoveredDomain
	var newDomains []string

	for _, domain := range result.ObservedDomains {
		lower := strings.ToLower(domain)

		if compName, ok := domainIndex[lower]; ok {
			covered = append(covered, CoveredDomain{Domain: domain, CoveredBy: compName})
		} else if existingDomains[lower] {
			covered = append(covered, CoveredDomain{Domain: domain, CoveredBy: "allowed_domains"})
		} else {
			newDomains = append(newDomains, domain)
		}
	}

	result.CoveredDomains = covered
	result.NewDomains = newDomains
	return result
}
