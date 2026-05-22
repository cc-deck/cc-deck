# Research: Deterministic Policy Generation

## R-001: Deterministic YAML Output in Go

**Decision**: Use `gopkg.in/yaml.v3` marshaling with sorted map keys. Go's `yaml.v3` marshals `map[string]` keys in sorted order by default. To guarantee byte-identical output, the assembly step must use a sorted structure (either sorted map or ordered slice) and avoid any runtime-dependent values (timestamps, random IDs).

**Rationale**: The existing `PolicyFile` struct uses `map[string]NetworkPolicy` for `network_policies`. Go's `yaml.v3` already sorts map keys alphabetically during marshaling. Combined with alphabetical component assembly order (from clarification), this guarantees determinism without changing the output format.

**Alternatives considered**:
- Custom YAML encoder with explicit ordering: unnecessary, `yaml.v3` already sorts maps
- JSON intermediate representation: adds complexity, YAML is the native format

## R-002: Component File Schema Design

**Decision**: Each component YAML file contains a `key` field (output section name), `name` (display name), `match` block, `endpoints` list, and `binaries` list. The schema maps directly to the existing `NetworkPolicy` struct plus match metadata.

**Rationale**: The existing `PolicyEndpoint` and `PolicyBinary` structs already model the output format. Adding a thin wrapper with match conditions keeps component files self-contained while reusing existing serialization.

**Alternatives considered**:
- Separate match file + endpoint file per component: over-engineering for ~15 components
- Embedding match conditions in the manifest: violates FR-002 (endpoints must not be in code)

## R-003: Component Matching Logic

**Decision**: Match evaluation follows short-circuit OR within a condition type and AND across condition types. A component matches if:
1. `always: true` is set, OR
2. ANY tool in `match.tools` appears in the manifest's tools list, OR
3. ANY credential type in `match.credentials` appears in the manifest's credentials, OR
4. ANY feature in `match.features` appears in the manifest's features

Multiple condition types in a single component use OR (any match suffices). This keeps component files simple: a component for "rust" endpoints only needs `match: { tools: [cargo, rust] }`.

**Rationale**: OR semantics match the current `addToolEndpoints()` behavior (any keyword match triggers inclusion). AND semantics would require components to know about unrelated manifest sections.

**Alternatives considered**:
- AND across condition types: too restrictive, would require duplicating components
- Weighted/priority matching: unnecessary complexity for binary include/exclude decisions

## R-004: Catalog Repo Structure

**Decision**: The catalog repo contains a `catalog.yaml` index file listing available component filenames. The `capture` command fetches this index, then downloads each listed component file via raw GitHub URL. Components are cached in `.cc-deck/setup/openshell/components/`.

**Rationale**: A simple index + individual files approach keeps the catalog repo structure flat and auditable. Using raw GitHub URLs avoids needing a custom API server. The index file prevents the client from needing to enumerate the repo.

**Alternatives considered**:
- GitHub API tree listing: requires authentication for private repos, rate-limited
- Bundled tarball: harder to inspect, all-or-nothing updates
- Git clone: heavy for ~15 small YAML files

## R-005: Embedded Component Extraction from Hardcoded Maps

**Decision**: Extract the following embedded components from current hardcoded data in `policy.go`:
1. `claude-code.yaml` from `DefaultPolicy()` claude_code section + `claudeCodeBinaries()`
2. `git-hosting.yaml` from `DefaultPolicy()` github section
3. `rust.yaml` from `toolEndpoints["rust"]` and `toolEndpoints["cargo"]`
4. `go.yaml` from `toolEndpoints["go"]`
5. `node.yaml` from `toolEndpoints["node"]` and `toolEndpoints["npm"]`
6. `python.yaml` from `toolEndpoints["python"]`, `toolEndpoints["pip"]`, `toolEndpoints["uv"]`
7. `vertex-ai.yaml` from `vertexEndpoints()`

**Rationale**: Direct 1:1 mapping from existing hardcoded data ensures no endpoint regressions. The `claude-code` and `git-hosting` components use `always: true` matching (they are always needed). Tool components use `match.tools`. Vertex uses `match.credentials`.

**Alternatives considered**:
- Combining all tool registries into one component: violates the "one concern per component" principle

## R-006: Build Refresh Policy Integration

**Decision**: The `build refresh` command adds a policy generation step after existing snippet extraction. When the manifest has an `openshell` target, it:
1. Loads components from all three tiers (embedded, cached catalog, user-local)
2. Resolves precedence by filename stem
3. Evaluates match conditions against the manifest
4. Assembles matching components into a `PolicyFile`
5. Applies explicit overrides from `targets.openshell.policy` via existing `MergePolicy()`
6. Writes `openshell/policy.yaml`

**Rationale**: Inserting into the existing `build refresh` flow (in `refreshOpenShellTarget()`) keeps the command semantics unchanged. The existing `MergePolicy()` function handles explicit overrides as a final step, maintaining backward compatibility.

**Alternatives considered**:
- Separate `build policy` subcommand: fragments the workflow, user must remember two commands
- Generating during `build run`: violates FR-009 (build must use pre-rendered policy)

## R-007: Capture Catalog Integration

**Decision**: Add a catalog fetch step to the capture command. After the existing workspace scan completes, capture downloads the catalog index from the configured GitHub repo URL, fetches all component files, and caches them in `.cc-deck/setup/openshell/components/`. On network failure, warns and continues.

**Rationale**: Capture already handles workspace analysis and manifest updates. Adding catalog fetch here keeps all "gather external resources" logic in one command. The existing `--all` flag behavior (auto-accept) works naturally.

**Alternatives considered**:
- Separate `cc-deck catalog update` command: adds another command to remember
- Fetch during `build refresh`: mixes network I/O with deterministic assembly
