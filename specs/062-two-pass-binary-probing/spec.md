# Feature Specification: Two-Pass Binary Probing

**Feature Branch**: `062-two-pass-binary-probing`
**Created**: 2026-05-24
**Status**: Draft
**Input**: User description: "Replace static well-known paths table with two-pass image build that probes the actual container for binary locations"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Automatic Binary Path Discovery via Image Probing (Priority: P1)

A developer runs `cc-deck build` to create an OpenShell image. The build process first produces the image with tools installed but without binary restrictions in the policy. It then probes the built image for actual binary paths using `which` (with `find` as fallback). The policy is regenerated with the probed paths, and the image is rebuilt with the corrected policy. The developer never needs to know where tools are installed.

**Why this priority**: This is the core value proposition. The well-known paths table is fragile and requires manual maintenance. Probing the actual image produces correct paths regardless of install method, base image, or user configuration.

**Independent Test**: Build an OpenShell image with a manifest that includes cargo and python3. Inspect the generated policy.yaml and verify that binary paths match the actual locations in the image (e.g., `/usr/bin/cargo` on Fedora, `/usr/local/bin/cargo` if installed via rustup). Compare against running `which cargo` inside the image.

**Acceptance Scenarios**:

1. **Given** a manifest with cargo as a package manager tool, **When** the user runs `cc-deck build`, **Then** the generated policy contains the actual path where cargo is installed in the image (as reported by `which cargo` inside the container).
2. **Given** a manifest with python3 installed via package manager, **When** the build completes, **Then** the policy contains the correct path for pip/pip3 in the image, plus glob patterns for virtual environment binaries.
3. **Given** a tool installed via github-release to a custom location, **When** the build completes, **Then** the probed path matches the actual install location and appears in the policy.

---

### User Story 2 - Runtime-Created Binaries Covered by Glob Patterns (Priority: P1)

A developer creates a Python virtual environment inside the OpenShell sandbox after the image is built. The venv creates new pip and python binaries under `/sandbox/.venv/bin/`. Because the policy includes glob patterns for Python runtime binaries (e.g., `/sandbox/**/bin/pip`), the supervisor allows the venv's pip to access PyPI without any manual policy changes.

**Why this priority**: Runtime-created binaries are the primary failure mode of the static table approach. Tools like pip, cargo, and npx create new binaries that did not exist at build time. Without glob coverage, developers hit 403 errors when using these tools.

**Independent Test**: Build an image with Python tools. Inside the sandbox, create a virtual environment. Run `pip install requests` from the venv. Verify the request succeeds (no 403 from the supervisor).

**Acceptance Scenarios**:

1. **Given** a built image with Python tools and glob patterns in the policy, **When** a developer creates a venv and runs `pip install` from within it, **Then** the supervisor allows the request because `/sandbox/.venv/bin/pip` matches the glob `/sandbox/**/bin/pip`.
2. **Given** a built image with Rust tools, **When** a developer installs a new toolchain via rustup, **Then** the supervisor allows cargo from the new toolchain because the glob `/sandbox/.rustup/toolchains/*/bin/cargo` covers it.
3. **Given** a built image with Node tools, **When** a developer runs an npx-installed tool, **Then** the supervisor allows it because the glob `/sandbox/**/node_modules/.bin/*` covers npx binaries.

---

### User Story 3 - Well-Known Paths Table Eliminated (Priority: P2)

A contributor to cc-deck adds support for a new tool (e.g., Elixir's mix). They define a policy component with endpoints and match conditions. They do not need to research or add binary path entries to a lookup table. The probe step discovers the correct path automatically during the build.

**Why this priority**: Removing the well-known paths table reduces maintenance burden and eliminates a class of bugs where the table entries are wrong for specific base images or install methods.

**Independent Test**: Add a new policy component for a tool (e.g., mix for Hex packages). Build an image with the tool installed. Verify the policy contains the correct binary path without any changes to `policy_binaries.go`.

**Acceptance Scenarios**:

1. **Given** a new policy component with match.tools listing "mix" and no well-known paths table entry for mix, **When** the image is built, **Then** the probe discovers `/usr/bin/mix` (or wherever mix is installed) and populates the binaries field.
2. **Given** an existing embedded component for Rust with no hardcoded binaries, **When** the image is built with cargo installed via rustup (not package manager), **Then** the probe discovers cargo at `~/.cargo/bin/cargo` and the policy is correct.

---

### User Story 4 - Layer Caching Minimizes Rebuild Cost (Priority: P3)

A developer iterates on their manifest, adding and removing tools. Each build runs the two-pass process. The second pass (policy rebuild) is fast because all tool installation layers are cached from the first pass. Only the policy COPY layer and subsequent layers are rebuilt.

**Why this priority**: Build performance is important for developer experience. If the two-pass approach added significant build time, developers would avoid rebuilding.

**Independent Test**: Build an image. Time the build. Change only the manifest's tool list. Rebuild. Verify the second pass completes in under 10 seconds (only the policy layer and later layers rebuild).

**Acceptance Scenarios**:

1. **Given** a previously built image, **When** the developer rebuilds with the same tools but changed endpoints, **Then** the second pass completes in under 10 seconds because only the policy layer is invalidated.
2. **Given** a fresh build with no layer cache, **When** the developer builds, **Then** the total build time increases by no more than 30 seconds compared to a single-pass build (probe overhead plus policy COPY rebuild).

---

### Edge Cases

- What happens when `which` fails to find a tool in the image? The system falls back to `find / -name <tool> -type f -executable`. If both fail, the tool's policy entry gets only glob patterns (no exact paths), and a warning is logged.
- What happens when a tool is a symlink (e.g., `python3` -> `python3.12`)? The probe records the path returned by `which`, which is the symlink path. The OpenShell supervisor matches against the calling process path, which is the symlink path the user invoked.
- What happens when the first-pass image fails to build? The build fails entirely with the same error as a normal build. No probe or second pass is attempted.
- What happens when the probe container cannot start? The build fails with a clear error indicating the probe step failed. The error message includes instructions to check the base image for compatibility.
- What happens when no tools are listed in the manifest? No probe is needed. The policy is generated with only always-match components (claude-code, git-hosting) that have their own explicit binaries.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The build process MUST execute in two passes for OpenShell targets: a first pass that builds the image without binary restrictions, and a second pass that rebuilds with probed binary paths in the policy.
- **FR-002**: The first-pass policy MUST contain all endpoint definitions but MUST leave the `binaries` field empty on all entries. This allows the image to build successfully since the OpenShell supervisor is not running during the build.
- **FR-003**: After the first pass, the system MUST create a temporary container from the built image and probe for binary paths by running `which <tool>` for each tool listed in the manifest's tools section and each tool referenced in `match.tools` of matched policy components.
- **FR-004**: When `which` fails to locate a tool, the system MUST fall back to `find / -name <tool> -type f -executable` inside the container.
- **FR-005**: When both `which` and `find` fail for a tool, the system MUST log a warning and continue. That tool's policy entry receives only glob patterns (if applicable) and no exact path.
- **FR-006**: The system MUST add tool-specific glob patterns alongside probed paths for tools known to create binaries at runtime:
  - Python: `/sandbox/**/bin/pip`, `/sandbox/**/bin/pip3`, `/sandbox/**/bin/uv`, `/sandbox/**/bin/python`, `/sandbox/**/bin/python3`
  - Rust: `/sandbox/.rustup/toolchains/*/bin/cargo`, `/sandbox/.rustup/toolchains/*/bin/rustc`
  - Node: `/sandbox/**/node_modules/.bin/*`
  - Go: `/sandbox/go/bin/*`
- **FR-007**: Policy regeneration MUST populate each matched component's `binaries` field from the probe results for its `match.tools` entries, combined with applicable glob patterns.
- **FR-008**: The second pass MUST rebuild the image using the updated policy. All layers before the policy COPY instruction MUST be served from the build cache.
- **FR-009**: The `resolveBinaries()` function and the well-known paths table in `policy_binaries.go` MUST be removed and replaced by the probe-based approach.
- **FR-010**: Label stamping (`oci.StampPolicyLabel`) MUST run after the second build, labeling the final image.
- **FR-011**: Components with explicit binaries (e.g., claude-code.yaml, vertex-ai.yaml, git-hosting.yaml) MUST NOT be overwritten by probe results. Explicit binaries always take precedence.
- **FR-012**: The probe step MUST NOT apply to non-OpenShell targets. Container and SSH targets are unaffected by this change.

### Key Entities

- **First-Pass Image**: The intermediate image built with all tools installed but with a binaries-less policy. Used only as a probe target, not shipped.
- **Probe Container**: A temporary container created from the first-pass image. Runs `which` and `find` commands to discover binary paths. Destroyed after probing.
- **Runtime Glob Pattern**: A filesystem glob added to policy entries for tools that create binaries after the image is built (venvs, toolchain installs, npx).
- **Second-Pass Image**: The final image rebuilt with the corrected policy containing probed paths and globs. This is the image that gets labeled and shipped.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All tools listed in the manifest have their actual binary paths (as found inside the image) present in the generated policy, without any hardcoded path table.
- **SC-002**: Runtime-created binaries (Python venvs, Rust toolchains, npx tools) can access their package registries without 403 errors from the supervisor.
- **SC-003**: The second-pass rebuild completes in under 10 seconds when all tool installation layers are cached.
- **SC-004**: Adding a new tool to the manifest and rebuilding results in correct binary paths without any code changes to the path resolution logic.
- **SC-005**: All existing tests continue to pass after removing the well-known paths table.
- **SC-006**: The total build time overhead from the two-pass approach is less than 30 seconds compared to a single-pass build.

## Assumptions

- The OpenShell supervisor is not running during the image build process. Missing binaries in the first-pass policy do not cause build failures.
- The base image includes standard utilities (`which`, `find`) needed for the probe step. If the base image lacks these tools, the probe step fails gracefully with a clear error.
- Layer caching is effective for the two-pass approach because the Containerfile is structured so that tool installation layers precede the policy COPY instruction.
- OpenShell treats extra binary paths (paths that do not exist on the filesystem) as harmless. Glob patterns that match no files are ignored at runtime.
- The probe container runs in the same architecture as the target image. Cross-architecture builds are not supported by the probe step.
- Symlink resolution is not required. The OpenShell supervisor matches against the process path as invoked, not the resolved real path.
