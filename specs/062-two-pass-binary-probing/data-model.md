# Data Model: Two-Pass Binary Probing

## Entity Changes

### PolicyComponent (modified)

**File**: `cc-deck/internal/build/component.go`

Two new optional fields added to the existing struct:

```go
type PolicyComponent struct {
    Key           string           `yaml:"key"`
    Name          string           `yaml:"name"`
    Match         MatchCondition   `yaml:"match"`
    Endpoints     []PolicyEndpoint `yaml:"endpoints"`
    Binaries      []PolicyBinary   `yaml:"binaries,omitempty"`
    ProbeBinaries []string         `yaml:"probe_binaries,omitempty"`  // NEW
    RuntimeGlobs  []string         `yaml:"runtime_globs,omitempty"`   // NEW
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `probe_binaries` | `[]string` | No | Exact binary names to search for via `which`/`find`. Falls back to `match.tools` if absent. |
| `runtime_globs` | `[]string` | No | Glob patterns for binaries created at runtime (venvs, toolchains). Merged into `Binaries` alongside probe results. |

**Validation rules**:
- `probe_binaries` entries must not contain path separators (they are binary names, not paths).
- `runtime_globs` entries must start with `/` (absolute paths).
- Both fields are ignored for components with explicit `binaries` (which take precedence).

### ProbeResult (new)

**File**: `cc-deck/internal/build/probe.go` (new file)

```go
type ProbeResult struct {
    Binary string `json:"binary"`
    Path   string `json:"path"`
    Method string `json:"method"` // "which", "find", or "not-found"
}
```

Represents a single binary probe result. One per binary name probed in the container.

| Field | Type | Description |
|-------|------|-------------|
| `binary` | `string` | The binary name that was searched for (e.g., "pip3"). |
| `path` | `string` | The absolute path found (e.g., "/usr/bin/pip3"). Empty if not found. |
| `method` | `string` | How the binary was found: "which" (first try), "find" (fallback), "not-found". |

### ProbeReport (new)

**File**: `cc-deck/internal/build/probe.go`

```go
type ProbeReport struct {
    Results  map[string][]ProbeResult // component key -> probe results
    Warnings []string                 // tools not found
    Duration time.Duration            // total probe time
}
```

Aggregated probe results for the entire probe step, keyed by component key.

## Component YAML Changes

### Before (current schema)

```yaml
key: pkg_python
name: python packages
match:
  tools:
    - python
    - pip
    - uv
endpoints:
  - host: pypi.org
    port: 443
  - host: files.pythonhosted.org
    port: 443
```

### After (extended schema)

```yaml
key: pkg_python
name: python packages
match:
  tools:
    - python
    - pip
    - uv
probe_binaries:
  - pip
  - pip3
  - uv
  - python3
runtime_globs:
  - /sandbox/**/bin/pip
  - /sandbox/**/bin/pip3
  - /sandbox/**/bin/uv
  - /sandbox/**/bin/python
  - /sandbox/**/bin/python3
endpoints:
  - host: pypi.org
    port: 443
  - host: files.pythonhosted.org
    port: 443
```

## Affected Embedded Components

| Component | probe_binaries | runtime_globs |
|-----------|---------------|---------------|
| `python.yaml` | pip, pip3, uv, python3 | /sandbox/**/bin/pip, /sandbox/**/bin/pip3, /sandbox/**/bin/uv, /sandbox/**/bin/python, /sandbox/**/bin/python3 |
| `rust.yaml` | cargo, rustc | /sandbox/.rustup/toolchains/*/bin/cargo, /sandbox/.rustup/toolchains/*/bin/rustc |
| `node.yaml` | node, npm, npx | /sandbox/**/node_modules/.bin/* |
| `go.yaml` | go | /sandbox/go/bin/* |
| `claude-code.yaml` | (none, has explicit binaries) | (none) |
| `git-hosting.yaml` | (none, has explicit binaries) | (none) |
| `vertex-ai.yaml` | (none, has explicit binaries) | (none) |

## Removed Entities

### wellKnownPaths table (removed)

**File**: `cc-deck/internal/build/policy_binaries.go` (entire file removed)

The `var wellKnownPaths map[string][]string` table and the `resolveBinaries()` function are deleted. Their functionality is replaced by:
- `probe_binaries` field in component YAML (replaces the binary name lookup)
- `runtime_globs` field in component YAML (replaces the glob/alternative paths)
- `ProbeBinaries()` function in `probe.go` (replaces the runtime resolution)

## State Transitions

### Build Process States

```
Idle
  → First-Pass Build (policy with empty binaries)
    → Probe (create container, run which/find, collect results)
      → Second-Pass Build (policy with probed paths + globs)
        → Label Stamp (oci.StampPolicyLabel)
          → Cleanup (remove first-pass image)
            → Done

Failure at any probe/second-pass stage:
  → Keep first-pass image tagged as <name>:probe-debug
  → Report error
```
