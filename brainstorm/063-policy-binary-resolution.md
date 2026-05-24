# Policy Binary Resolution at Assembly Time

## Problem

OpenShell policy components define which network endpoints a tool can access, but they also need a `binaries` field listing the exact filesystem paths of the processes allowed to make those connections. Without `binaries`, the OpenShell supervisor blocks all access to the endpoint (CONNECT tunnel 403).

Currently, binary paths are hardcoded in the embedded policy components (`internal/build/policies/*.yaml`). This breaks the catalog model: catalog components should be installation-independent and shared across different setups. Hardcoded paths like `/usr/bin/cargo` or `/sandbox/.cargo/bin/cargo` are assumptions about where a tool was installed, which varies by base image, install method, and user configuration.

## Proposed Solution

Resolve binary paths at policy assembly time by cross-referencing the policy component's `match.tools` list with the manifest's `tools` section. The manifest already records how each tool was installed and (for github-release tools) where it was placed.

### How it works

1. **Catalog/embedded components** define only `endpoints` and `match.tools`. No `binaries` field.

2. **During `AssemblePolicy()`**, after matching components to the manifest:
   - For each matched component, read its `match.tools` list
   - Look up each tool name in the manifest's `tools` section
   - Resolve the binary path based on the tool's `install` method:
     - `install: package` (or omitted): `/usr/bin/<name>` (standard package manager destination)
     - `install: github-release`: use the `install_path` field from the manifest entry
   - Add a well-known-paths table for tools with non-standard locations (e.g., `claude` at `/sandbox/.local/bin/claude`, `cargo` also at `/sandbox/.cargo/bin/cargo`)
   - Merge resolved paths into the component's `binaries` field

3. **If a component already has `binaries`** (e.g., user-local override or explicit catalog entry), those take precedence. Only add resolved paths for components that lack binaries.

4. **If a tool is in `match.tools` but not in the manifest's `tools` section**, skip it silently. The tool may be pre-installed in the base image and not listed in the manifest.

### Well-known binary paths

Some tools install to predictable but non-standard locations. These should be encoded as a lookup table in the Go code, not in the catalog:

| Tool | Well-known paths |
|------|-----------------|
| `cargo` | `/sandbox/.cargo/bin/cargo`, `/sandbox/.rustup/toolchains/*/bin/cargo` |
| `rustc` | `/sandbox/.cargo/bin/rustc`, `/sandbox/.rustup/toolchains/*/bin/rustc` |
| `go` | `/usr/local/go/bin/go` |
| `claude` | `/sandbox/.local/bin/claude`, `/sandbox/.local/share/claude/**`, `/usr/local/bin/claude` |
| `node` | `/usr/bin/node`, `/usr/local/bin/node` |
| `npm` | `/usr/bin/npm`, `/usr/local/bin/npm` |
| `npx` | `/usr/local/bin/npx` |
| `pip` | `/sandbox/.venv/bin/pip` |
| `uv` | `/sandbox/.local/bin/uv` |
| `git` | `/usr/bin/git` |
| `gh` | `/usr/bin/gh`, `/usr/local/bin/gh` |

### What changes

- `internal/build/policy.go`: `AssemblePolicy()` gains a binary resolution step after component matching
- `internal/build/policy.go` or new file `internal/build/policy_binaries.go`: well-known paths table and resolution logic
- `internal/build/policies/*.yaml`: remove hardcoded `binaries` from embedded components (except `claude-code.yaml` and `vertex-ai.yaml` which have specific glob patterns)
- `internal/build/policy_test.go`: tests for binary resolution
- Catalog components (remote): already clean (no binaries), no changes needed

### What stays the same

- `claude-code.yaml` and `vertex-ai.yaml` keep their explicit `binaries` because Claude Code has a complex version-specific glob pattern (`/sandbox/.local/share/claude/**`) that cannot be derived from the manifest
- `git-hosting.yaml` could keep its binaries as a fallback, or move to the resolution table
- The manifest format does not change
- The `MergePolicy()` function does not change

## Edge cases

- Tool installed but not in manifest (pre-installed in base image): the well-known paths table covers common cases. The base image probe in `/cc-deck.build` (step C2) discovers pre-installed tools, but those findings are not currently stored in the manifest. If a component matches via `match.always: true` but has no tools in the manifest, it gets no binaries from resolution. The explicit `binaries` in `claude-code.yaml` handles this for the most critical case.

- Multiple install locations for the same tool: include all resolved paths. OpenShell allows any listed binary to connect. Extra paths that do not exist are harmless.

- User-local component overrides with explicit `binaries`: respected as-is, no automatic resolution applied.

## Not in scope

- Storing base image probe results in the manifest (future enhancement)
- Runtime binary detection inside the sandbox
- Per-endpoint binary restrictions (all binaries in a component can access all its endpoints)
