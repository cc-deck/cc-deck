# Research: Base Image Probe

## R1: Build Learnings File Format

**Decision**: Store probe cache in a new `probe-cache.json` file alongside `build-learnings.md`

**Rationale**: The existing `build-learnings.md` is a human-readable Markdown file containing build fix records (error/fix/date entries). It is not structured data. The probe cache requires machine-readable JSON with exact keys (image ref + digest) for O(1) lookup. Adding structured JSON to a Markdown file would complicate both parsing and human readability.

**Alternatives considered**:
- Embedding JSON in Markdown frontmatter: awkward, parsing fragile
- Converting learnings to JSON: breaks existing self-correction loop that reads/appends Markdown
- YAML file: JSON is simpler for the cache-lookup pattern (marshal/unmarshal only)

**Spec impact**: The clarification said "in the learnings file." The intent was "single location for build intelligence." We satisfy that by placing `probe-cache.json` in the same setup directory (`<setup-dir>/probe-cache.json`) alongside `build-learnings.md`, keeping all build intelligence co-located.

## R2: Existing Probe Infrastructure

**Decision**: Create a new `imageprobe` package separate from the existing `probe.go`

**Rationale**: The existing `probe.go` in `internal/build/` is for OpenShell policy binary path discovery (finding where specific binaries are installed for network policy enforcement). Feature 070 is a broader base image capabilities probe (OS family, package manager, tool versions, user setup). The concerns are distinct:

| Aspect | Existing probe.go | New base image probe |
|--------|-------------------|---------------------|
| Purpose | Find binary paths for policy YAML | Detect base image capabilities |
| Input | PolicyComponent list | Manifest tools + default tool set |
| Output | Binary path per component | OS family, pkg mgr, tool versions |
| When | OpenShell builds only | All container/openshell builds |
| Cache | None (runs every build) | Cached by image ref + digest |

**Alternatives considered**:
- Extending existing probe.go: too coupled to policy concepts, different data model
- Merging both probes into one container run: fragile, different output formats

## R3: Integration Points in the Build Pipeline

**Decision**: Hook probe into the Claude Code build skill (`cc-deck.build.md`) at section A2 (container) and C2 (openshell)

**Rationale**: The Containerfile generation happens in the Claude Code build skill (a markdown command file), not in compiled Go code. The skill orchestrates snippet assembly + generated sections. The probe must run before the generated install sections to inform which tools to skip.

**Integration flow**:
1. After reading the manifest and resolving the base image (A1/C1)
2. Before generating the Containerfile (A2/C2)
3. Check probe cache (by image ref + digest)
4. If cache miss or stale: run probe, store results
5. Pass probe results to Containerfile generation logic
6. Generated install sections use probe data to skip pre-installed tools and select the correct package manager

**Go code integration**: New `cc-deck build probe` subcommand that:
- Accepts a base image reference
- Runs the probe script via podman
- Outputs structured JSON results
- Handles caching in `probe-cache.json`

This keeps the probe logic in Go (testable, cached) while the build skill calls it as a CLI step.

## R4: Probe Script Design

**Decision**: Single self-contained shell script outputting JSON-per-line

**Rationale**: Follows the pattern of the existing `generateProbeScript()` in `probe.go`. One `podman run --rm --entrypoint /bin/sh` invocation with the script piped via stdin. The script detects:

1. OS family: read `/etc/os-release` for `ID` and `ID_LIKE`
2. Package manager: check `which dnf apt-get apk yum`
3. Tool versions: for each tool, `which <tool> && <tool> --version 2>&1`
4. User setup: `id`, `echo $HOME`, `echo $SHELL`
5. Shell availability: `which bash zsh sh`

Output is one JSON object per line, parsed by Go code.

**Alternatives considered**:
- Running multiple podman commands: too slow (each `podman run` has ~2s overhead)
- Using `podman inspect` only: gives image metadata but not runtime tool availability

## R5: Version Comparison Strategy

**Decision**: Parse semver-like versions and compare major.minor

**Rationale**: Per clarification, "compatible" means same or newer within the same major version. The comparison logic:
1. Extract version string from `<tool> --version` output (regex for `\d+\.\d+(\.\d+)?`)
2. Parse into major.minor.patch
3. Compare: same major AND probe minor >= required minor

Edge cases:
- Tools with non-semver versions (e.g., `git version 2.43.0`): extract the numeric part
- Tools that only report major.minor (e.g., `Python 3.12`): treat patch as 0
- Tools with no version output: treat as "present but unknown version" (assume compatible)

## R6: Default Tool Set

**Decision**: Embed the default tool list in Go code, derived from `base-image/scripts/install-tools.sh`

**Rationale**: The ~30 tools from the cc-deck base image install script represent the expected developer toolbox. Embedding them avoids a runtime dependency on the install script file.

Default tools: git, gh, glab, ripgrep, fd-find, fzf, jq, yq, bat, lsd, git-delta, zoxide, helix, vim, nano, curl, wget, htop, ncat, dig, ssh, make, sudo, nodejs, npm, python3, pip, uv, zsh, starship

The manifest's optional `probe_tools:` key can override or extend this list.
