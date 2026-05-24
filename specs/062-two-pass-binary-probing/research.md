# Research: Two-Pass Binary Probing

## Decision 1: Probe Container Execution Model

**Decision**: Use `podman run --rm` with a shell script that probes all binaries in a single container invocation.

**Rationale**: A single `podman run` with a concatenated script (one `which <binary> || find / ...` per tool) minimizes container startup overhead. The `--rm` flag handles cleanup automatically on success. On failure, the container is also removed, but the image remains for debugging per FR-013.

**Alternatives considered**:
- `podman create` + multiple `podman exec` calls: higher overhead from repeated exec syscalls and more complex error handling (need to track container lifecycle manually).
- Separate `podman run` per binary: worst overhead, N container startups instead of 1.

## Decision 2: Probe Script Structure

**Decision**: Generate a shell script that outputs JSON-structured results, one line per binary probed.

**Rationale**: JSON output parsing in Go is straightforward with `encoding/json`. Each line contains the binary name, the resolved path (if found), and the method used (which/find/not-found). This structure makes result parsing deterministic and testable.

**Format**:
```json
{"binary":"pip","path":"/usr/bin/pip","method":"which"}
{"binary":"pip3","path":"/usr/bin/pip3","method":"which"}
{"binary":"uv","path":"","method":"not-found"}
```

**Alternatives considered**:
- Plain text with delimiters: fragile, harder to parse when paths contain special characters.
- Full JSON array: requires buffering all output before parsing, no streaming.

## Decision 3: Policy Assembly Split (First-Pass vs Second-Pass)

**Decision**: Add a `stripBinaries` option to `AssemblePolicy()` (or a post-processing step) that clears the `Binaries` field on all components except those with explicit binaries (match.always components like claude-code, git-hosting).

**Rationale**: The first-pass policy needs all endpoints intact (for correct Containerfile structure and completeness) but must have empty binaries. Rather than creating a separate assembly path, a single flag or post-processing function keeps the assembly logic unified. Components with `match.always: true` and explicit binaries (like claude-code.yaml, git-hosting.yaml) keep their binaries in both passes, since they are not probed.

**Alternatives considered**:
- Two separate assembly functions: code duplication, divergence risk.
- Skip policy generation entirely for first pass: the COPY instruction in the Containerfile would fail if policy.yaml is missing.

## Decision 4: Probe Result Integration into Policy

**Decision**: Replace `resolveBinaries()` with a new `applyProbeResults()` function that merges probe results and runtime globs from component YAML into the `Binaries` field.

**Rationale**: The function takes the probe results map (component key to list of found paths), the matched components (with `runtime_globs` from YAML), and produces the final `Binaries` list per component. Explicit binaries (components where `Binaries` is already populated from YAML) are preserved unchanged, matching the current behavior of `resolveBinaries()`.

**Alternatives considered**:
- Modifying `resolveBinaries()` in-place: the function's purpose changes fundamentally (static lookup to runtime results), so a clean replacement is clearer.

## Decision 5: Component YAML Schema Extension

**Decision**: Add two optional fields to `PolicyComponent`: `probe_binaries` (list of binary names to search for) and `runtime_globs` (list of glob patterns for runtime-created binaries).

**Rationale**: These fields make each component self-contained. The `probe_binaries` list is separate from `match.tools` because matching uses substring/case-insensitive logic while probing needs exact binary names. The `runtime_globs` field replaces the hardcoded glob patterns from the well-known paths table.

**Schema change**:
```go
type PolicyComponent struct {
    Key           string           `yaml:"key"`
    Name          string           `yaml:"name"`
    Match         MatchCondition   `yaml:"match"`
    Endpoints     []PolicyEndpoint `yaml:"endpoints"`
    Binaries      []PolicyBinary   `yaml:"binaries,omitempty"`
    ProbeBinaries []string         `yaml:"probe_binaries,omitempty"`
    RuntimeGlobs  []string         `yaml:"runtime_globs,omitempty"`
}
```

## Decision 6: Containerfile Layer Ordering

**Decision**: No changes needed to the Containerfile template structure.

**Rationale**: The current ordering already places tool installation layers (packages, language-specific, github-release) before the policy COPY layer (04-openshell-extras). On the second pass, the same Containerfile is used but with the updated policy.yaml on disk. All layers before the COPY are served from cache because the Containerfile and tool installation commands are identical. Only the COPY layer and later layers rebuild.

**Alternatives considered**:
- Restructuring the Containerfile to isolate the policy layer further: unnecessary, current structure already provides the desired caching behavior.

## Decision 7: First-Pass Image Tagging

**Decision**: Tag the first-pass image as `<name>:probe-build` during the first pass. On successful second pass, remove the first-pass image. On failure, retag as `<name>:probe-debug`.

**Rationale**: A distinct tag prevents confusion with the final image. The `:probe-build` tag is a working tag used only during the two-pass process. If the second pass fails, retagging to `:probe-debug` makes the purpose clear to the developer.

## Decision 8: Timeout Implementation

**Decision**: Use Go's `context.WithTimeout` to enforce the 30-second per-binary and 5-minute total timeouts. The probe script itself runs under a single `podman run` with `--timeout` for the overall limit, and individual `timeout` commands wrap each `which`/`find` invocation in the script.

**Rationale**: Combining Go-level context timeouts with shell-level `timeout` commands provides defense in depth. The shell `timeout` handles individual binary probes cleanly, while the Go context handles the overall probe step and container lifecycle.
