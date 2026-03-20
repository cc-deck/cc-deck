package network

// builtinGroups defines the domain groups embedded in the cc-deck binary.
// Each group maps to a set of domain patterns required by a specific ecosystem.
// Wildcard patterns (prefixed with ".") match the domain and all subdomains.
var builtinGroups = map[string]DomainGroup{
	"anthropic": {
		Name:   "anthropic",
		Source: SourceBuiltin,
		Domains: []string{
			"api.anthropic.com",
			".anthropic.com",
			"claude.ai",
			".claude.ai",
			"platform.claude.com",
			".claude.com",
			".statsigapi.net",
			".sentry.io",
		},
	},
	"vertexai": {
		Name:   "vertexai",
		Source: SourceBuiltin,
		Domains: []string{
			"oauth2.googleapis.com",
			"aiplatform.googleapis.com",
			"us-central1-aiplatform.googleapis.com",
			"us-east5-aiplatform.googleapis.com",
			"europe-west1-aiplatform.googleapis.com",
			"asia-east1-aiplatform.googleapis.com",
		},
	},
	"python": {
		Name:   "python",
		Source: SourceBuiltin,
		Domains: []string{
			"pypi.org",
			"files.pythonhosted.org",
			"pypi.python.org",
		},
	},
	"nodejs": {
		Name:   "nodejs",
		Source: SourceBuiltin,
		Domains: []string{
			"registry.npmjs.org",
			"npmjs.com",
			".npmjs.org",
			".yarnpkg.com",
		},
	},
	"rust": {
		Name:   "rust",
		Source: SourceBuiltin,
		Domains: []string{
			"crates.io",
			"static.crates.io",
			".crates.io",
			"index.crates.io",
		},
	},
	"golang": {
		Name:   "golang",
		Source: SourceBuiltin,
		Domains: []string{
			"proxy.golang.org",
			"sum.golang.org",
			"storage.googleapis.com",
		},
	},
	"github": {
		Name:   "github",
		Source: SourceBuiltin,
		Domains: []string{
			"github.com",
			".github.com",
			".githubusercontent.com",
			".githubassets.com",
			"ghcr.io",
		},
	},
	"gitlab": {
		Name:   "gitlab",
		Source: SourceBuiltin,
		Domains: []string{
			"gitlab.com",
			".gitlab.com",
			"registry.gitlab.com",
		},
	},
	"docker": {
		Name:   "docker",
		Source: SourceBuiltin,
		Domains: []string{
			"registry-1.docker.io",
			"auth.docker.io",
			"production.cloudflare.docker.com",
			"index.docker.io",
		},
	},
	"quay": {
		Name:   "quay",
		Source: SourceBuiltin,
		Domains: []string{
			"quay.io",
			".quay.io",
			"cdn.quay.io",
		},
	},
}

// BuiltinGroupNames returns a sorted list of all built-in group names.
func BuiltinGroupNames() []string {
	names := make([]string, 0, len(builtinGroups))
	for name := range builtinGroups {
		names = append(names, name)
	}
	sortStrings(names)
	return names
}
