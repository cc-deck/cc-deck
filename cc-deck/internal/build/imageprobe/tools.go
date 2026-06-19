package imageprobe

import "sort"

// DefaultTools lists the canonical tool names that the cc-deck base image
// installs. Derived from base-image/scripts/install-tools.sh.
var DefaultTools = []string{
	"git",
	"gh",
	"glab",
	"rg",
	"fd",
	"fzf",
	"jq",
	"yq",
	"bat",
	"lsd",
	"delta",
	"zoxide",
	"hx",
	"vim",
	"nano",
	"curl",
	"wget",
	"htop",
	"ncat",
	"dig",
	"ssh",
	"make",
	"sudo",
	"node",
	"npm",
	"python3",
	"pip",
	"uv",
	"zsh",
	"starship",
}

// ProbeToolEntry is a tool to probe for, with an optional required version.
type ProbeToolEntry struct {
	Name    string
	Version string
}

// MergeToolSets combines the default tool set with manifest-declared
// probe_tools. Manifest entries override defaults (by name). If the
// manifest list is nil, the defaults are used unchanged.
func MergeToolSets(manifestTools []ProbeToolEntry) []ProbeToolEntry {
	byName := make(map[string]ProbeToolEntry, len(DefaultTools))
	for _, name := range DefaultTools {
		byName[name] = ProbeToolEntry{Name: name}
	}

	for _, mt := range manifestTools {
		byName[mt.Name] = mt
	}

	result := make([]ProbeToolEntry, 0, len(byName))
	for _, name := range DefaultTools {
		if entry, ok := byName[name]; ok {
			result = append(result, entry)
			delete(byName, name)
		}
	}

	extra := make([]string, 0, len(byName))
	for name := range byName {
		extra = append(extra, name)
	}
	sort.Strings(extra)
	for _, name := range extra {
		result = append(result, byName[name])
	}

	return result
}
