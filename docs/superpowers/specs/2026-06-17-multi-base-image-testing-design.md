# Multi-Base-Image Testing & Discovery

**Date:** 2026-06-17
**Status:** proposed

## Problem

cc-deck builds container images for two targets (container and openshell), each
currently hardcoded to a single default base image. The Red Hat AI ecosystem is
rapidly evolving its base image story, with multiple teams (AIPCC, OpenShell
upstream, AI Catalyst) producing base images that cc-deck should be able to build
on top of. There is no automated way to:

1. Validate that the build pipeline works against multiple base images
2. Detect when upstream base images change in breaking ways
3. Track which base images exist and whether they are current

## Goals

- Make cc-deck's build pipeline flexible across multiple base images per target
  type without restructuring the template system
- Provide a tiered e2e test suite: fast build-and-probe (CI gate) and deeper
  session smoke tests (on-demand/nightly)
- Provide a discovery skill that checks known sources for base image updates and
  helps maintain the registry

## Non-Goals

- Replacing the manifest-level `base:` override (users can still pin any image)
- Automated base image selection or recommendation
- Full integration testing of upstream OpenShell features

## Design

### 1. Base Image Registry

A YAML file at the repo root (`base-images.yaml`) lists known base images
grouped by target type:

```yaml
openshell:
  - name: nvidia-upstream
    ref: ghcr.io/nvidia/openshell-community/sandboxes/base:latest
    default: true
  - name: rh-ubi-openshell
    ref: quay.io/aipcc/openshell-base:latest
container:
  - name: fedora-41
    ref: registry.fedoraproject.org/fedora:41
    default: true
  - name: rh-ubi9
    ref: registry.access.redhat.com/ubi9/ubi:latest
```

Each entry has:
- `name`: Human-readable identifier used in test output and Make target filtering
- `ref`: Full container image reference
- `default`: Boolean. The entry used when the user's manifest does not specify a
  `base:` override. Exactly one entry per target type must be `default: true`.

The file is checked into the repo and maintained manually, assisted by the
discovery skill.

### 2. Build Pipeline Integration

#### Default resolution

The hardcoded constants `DefaultOpenShellBaseImage` and `DefaultBaseImage` in
`internal/build/manifest.go` are replaced with a lookup function that:

1. Reads `base-images.yaml` from the repo root (or an embedded copy)
2. Returns the `ref` of the `default: true` entry for the requested target type
3. Falls back to the current hardcoded values if the file is missing or
   unparseable (graceful degradation, so `go install` and standalone CLI
   usage continue to work without the file present)

The `Manifest.BaseImage()` and `Manifest.OpenShellBaseImage()` methods continue
to prefer the manifest's own `base:` field when set.

#### Go types

```go
// BaseImageEntry describes a single base image in the registry.
type BaseImageEntry struct {
    Name    string `yaml:"name"`
    Ref     string `yaml:"ref"`
    Default bool   `yaml:"default,omitempty"`
}

// BaseImageRegistry is the top-level structure of base-images.yaml.
type BaseImageRegistry struct {
    OpenShell []BaseImageEntry `yaml:"openshell,omitempty"`
    Container []BaseImageEntry `yaml:"container,omitempty"`
}
```

#### New functions

```go
// LoadBaseImageRegistry reads and parses base-images.yaml.
func LoadBaseImageRegistry(path string) (*BaseImageRegistry, error)

// DefaultRef returns the ref of the default entry for the given target type.
// Returns an empty string if no default is set.
func (r *BaseImageRegistry) DefaultRef(target string) string

// EntriesForTarget returns all entries for the given target type.
func (r *BaseImageRegistry) EntriesForTarget(target string) []BaseImageEntry
```

### 3. Probe Suite (Tier 1)

A Go test file at `internal/e2e/image_probe_test.go` that validates built images
work correctly. Gated behind a build tag (`//go:build e2e`) so it does not run
during normal `make test`.

#### Test flow per base image entry

1. Generate a Containerfile using the existing template system with the entry's
   `ref` as the base image
2. Build the image with `podman build`
3. Start a container with `podman run -d`
4. Execute probe checks inside the container via `podman exec`
5. Collect results, tear down with `podman rm -f`

#### Probe checks

| Check | Command | Pass condition |
|-------|---------|----------------|
| Claude Code binary | `claude --version` | Exit 0 |
| Zellij binary | `zellij --version` | Exit 0, version >= 0.44 |
| cc-deck binary | `cc-deck --version` | Exit 0 |
| User identity | `whoami` | Expected user (dev/sandbox) |
| Home directory | `echo $HOME` | Expected path |
| Shell config | `echo $SHELL` | Expected shell |
| PATH completeness | `which cc-session cc-setup` | All found |
| Write permissions | `touch $HOME/test-write` | Exit 0 |
| Plugin installed | `ls $HOME/.config/zellij/plugins/cc_deck.wasm` | File exists |

#### Make targets

```makefile
test-images:        ## Run probe suite against all base images
test-images-quick:  ## Run probe suite against default base images only
```

Optional filter: `make test-images BASE=rh-ubi-openshell` runs only the named
entry.

#### Failure semantics

- Default base image probe failure: hard error (exit 1)
- Non-default base image probe failure: warning printed to stderr, test marked as
  expected-fail with output captured for review

### 4. Session Smoke Test (Tier 2)

A follow-up test tier in `internal/e2e/image_session_test.go` that validates
the full runtime stack. Also gated behind `//go:build e2e`.

#### Test flow

1. Start the container with an API key passed via environment variable
2. Inside the container, start `cc-deck run` which launches Zellij with the
   cc-deck layout
3. Wait for Zellij to start and plugin to load (detect via log file or process
   list)
4. Verify the sidebar plugin process exists
5. Verify pipe communication by checking the plugin debug log for initialization
   messages
6. Tear down

#### Requirements

- Requires `ANTHROPIC_API_KEY` or `GOOGLE_APPLICATION_CREDENTIALS` (or skips)
- Runs on-demand (`make test-images-session`) or nightly in CI
- Timeout: 60 seconds per image (container start + Zellij init + plugin load)

### 5. Discovery Skill

A local Claude Code skill at `.claude/skills/cc-deck-base-images/` that assists
with maintaining `base-images.yaml`.

#### Invocation

`/cc-deck.base-images` (no arguments: full check) or
`/cc-deck.base-images update` (check and apply updates)

#### Discovery sources

1. **Registry digest check**: For each entry in `base-images.yaml`, run
   `skopeo inspect --raw docker://<ref>` to get the current digest. Compare
   against a stored digest file (`.base-images-digests.json`) to detect upstream
   rebuilds.

2. **GitHub release check**: Check
   `NVIDIA/OpenShell-Community` releases for new sandbox base image tags. Check
   `red-hat-data-services/agentic-starter-kits` for new Containerfile base image
   references.

3. **Known registry scan**: Check `quay.io/aipcc/` and
   `ghcr.io/nvidia/openshell-community/sandboxes/` for new image tags matching
   patterns like `base:*` or `openshell-*`.

#### Output

The skill reports:
- **Digest changes**: "nvidia-upstream: digest changed since last check
  (sha256:abc... -> sha256:def...)"
- **New images found**: "New image in quay.io/aipcc/: openshell-base:v0.2"
- **Stale entries**: "rh-ubi-openshell: image not found (404), may have been
  renamed or removed"

When run with `update`, it offers to modify `base-images.yaml` with discovered
changes and updates `.base-images-digests.json`.

#### Digest tracking

`.base-images-digests.json` is gitignored (it's local state). It stores the last
known digest per entry so the skill can detect upstream rebuilds between checks:

```json
{
  "nvidia-upstream": "sha256:abc123...",
  "rh-ubi-openshell": "sha256:def456..."
}
```

### 6. File Layout

```
base-images.yaml                           # Base image registry (checked in)
.base-images-digests.json                  # Last-known digests (gitignored)
cc-deck/internal/build/registry.go         # Go types + loader for base-images.yaml
cc-deck/internal/build/registry_test.go    # Unit tests for registry loading
cc-deck/internal/e2e/image_probe_test.go   # Tier 1: build-and-probe tests
cc-deck/internal/e2e/image_session_test.go # Tier 2: session smoke tests
cc-deck/internal/e2e/testdata/             # Test fixtures (sample manifests)
.claude/skills/cc-deck-base-images/        # Discovery skill
```

### 7. Rollout Order

1. Add `base-images.yaml` with current defaults (NVIDIA upstream + Fedora 41)
2. Implement `registry.go` with types, loader, and default resolution
3. Wire `Manifest.BaseImage()` and `Manifest.OpenShellBaseImage()` to use the
   registry as fallback
4. Implement Tier 1 probe suite with Make targets
5. Add Red Hat image entries to `base-images.yaml` (once references are known)
6. Implement the discovery skill
7. Implement Tier 2 session smoke tests
