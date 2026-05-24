# Feature Specification: Policy Binary Resolution

**Feature Branch**: `061-policy-binary-resolution`
**Created**: 2026-05-24
**Status**: Draft
**Input**: User description: "Resolve binary paths at policy assembly time using manifest tool data"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Automatic Binary Resolution from Manifest (Priority: P1)

A developer builds an OpenShell image using `cc-deck build`. The build manifest lists installed tools with their install methods. When the policy is assembled, each network policy entry automatically receives a `binaries` field populated from the manifest's tool data, so that the OpenShell supervisor knows which processes are allowed to access each endpoint. The developer does not need to manually specify binary paths.

**Why this priority**: Without binary resolution, the OpenShell supervisor blocks all network access for tools that lack a `binaries` field in their policy entry, making the sandbox unusable for development tasks like fetching dependencies.

**Independent Test**: Build an OpenShell image with a manifest that includes cargo (install: package) and a custom tool (install: github-release, install_path: /usr/local/bin/mytool). Inspect the generated policy.yaml and verify that the Rust packages policy entry has `/usr/bin/cargo` in its binaries, and the custom tool's policy entry has `/usr/local/bin/mytool`.

**Acceptance Scenarios**:

1. **Given** a manifest with a tool installed via package manager (e.g., cargo), **When** the policy is assembled, **Then** the matching network policy entry includes `/usr/bin/cargo` in its binaries field.
2. **Given** a manifest with a tool installed via github-release with an explicit install_path, **When** the policy is assembled, **Then** the matching network policy entry includes the specified install_path in its binaries field.
3. **Given** a manifest with a tool that has well-known non-standard paths (e.g., cargo also installs to `~/.cargo/bin/`), **When** the policy is assembled, **Then** the binaries field includes both the package path and the well-known alternative paths.

---

### User Story 2 - Catalog Components Without Hardcoded Binaries (Priority: P2)

A catalog maintainer publishes a policy component for a new package registry (e.g., Hex for Elixir). The component defines endpoints and match conditions but does not include binary paths. When a developer uses this component, the system automatically resolves the binary paths from their manifest, so the catalog component works across different installation methods and base images.

**Why this priority**: Keeps the catalog installation-independent. Catalog components should work regardless of whether tools are installed via package manager, binary download, or language-specific installer.

**Independent Test**: Remove the `binaries` field from the embedded rust.yaml component. Build a manifest with `cargo` as a package tool. Verify the assembled policy still includes cargo binary paths resolved from the manifest.

**Acceptance Scenarios**:

1. **Given** a policy component with no binaries field and match.tools listing "cargo", **When** the component matches a manifest containing a cargo tool entry, **Then** the assembled policy entry includes resolved binary paths for cargo.
2. **Given** a policy component with no binaries and a tool in match.tools that is NOT in the manifest, **When** the policy is assembled, **Then** the system skips that tool silently (no error, no binary path added).

---

### User Story 3 - Explicit Binaries Override Resolution (Priority: P3)

A developer creates a user-local policy component with explicit binary paths for a tool installed in a custom location. The system uses those explicit paths instead of resolving from the manifest, ensuring that manual overrides are always respected.

**Why this priority**: Provides an escape hatch for custom installations where automatic resolution would produce incorrect paths.

**Independent Test**: Create a user-local policy component with explicit binaries pointing to `/opt/custom/bin/cargo`. Verify the assembled policy uses `/opt/custom/bin/cargo` and does not add manifest-resolved paths.

**Acceptance Scenarios**:

1. **Given** a policy component with an explicit binaries field, **When** the policy is assembled, **Then** the explicit binaries are preserved and no additional paths are added from manifest resolution.

---

### Edge Cases

- What happens when a tool in match.tools has no entry in the manifest? The system skips it silently. The tool may be pre-installed in the base image and not listed in the manifest.
- What happens when the manifest has a tool with install type "package" but the binary name differs from the tool name? The well-known paths table provides the correct mapping. If no mapping exists, the system uses `/usr/bin/<tool-name>` as the default.
- What happens when multiple components reference the same tool? Each component gets its own copy of the resolved binary paths. There is no deduplication across components.
- What happens when a tool has both well-known paths and a manifest install_path? Both are included in the binaries field. Extra paths that do not exist on the filesystem are harmless.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST resolve binary paths for each matched policy component by looking up its match.tools entries in the manifest's tools section.
- **FR-002**: For tools with install type "package" (or omitted), the system MUST add `/usr/bin/<tool-name>` as the default binary path.
- **FR-003**: For tools with install type "github-release", the system MUST use the install_path field from the manifest as the binary path.
- **FR-004**: System MUST maintain a well-known paths table that maps tool names to additional common installation locations beyond the default.
- **FR-005**: System MUST include all well-known paths for a tool in addition to the manifest-derived path, since tools may be accessible from multiple locations.
- **FR-006**: System MUST NOT overwrite explicit binaries on components that already define them. Explicit binaries take precedence over automatic resolution.
- **FR-007**: System MUST skip tools in match.tools that do not appear in the manifest's tools section, without producing errors or warnings.
- **FR-008**: System MUST remove hardcoded binaries from the embedded package registry components (go.yaml, rust.yaml, node.yaml, python.yaml) so that resolution is the sole source of binary paths for these components.
- **FR-009**: System MUST preserve explicit binaries on claude-code.yaml, vertex-ai.yaml, and git-hosting.yaml because these components have complex glob patterns or always-match conditions that cannot be derived from the manifest.

### Key Entities

- **Well-Known Paths Table**: A mapping from tool names to lists of filesystem paths where the tool is commonly installed. Used to supplement manifest-derived paths.
- **Policy Component**: A YAML definition containing endpoints, match conditions, and optionally binaries. Components come from embedded, catalog, or user-local tiers.
- **Manifest Tool Entry**: A record in the build manifest's tools section describing a tool's name, install method, and optional install path.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All package registry policy components (Rust, Go, Node, Python) produce functional sandbox network access without hardcoded binary paths.
- **SC-002**: A new catalog component with only endpoints and match.tools (no binaries) automatically gains correct binary paths when assembled with a manifest.
- **SC-003**: Policy assembly adds less than 10 milliseconds of processing time for binary resolution (negligible compared to the overall build process).
- **SC-004**: All existing tests continue to pass after removing hardcoded binaries from embedded components.
- **SC-005**: Explicit binaries in user-local components are never modified by automatic resolution.

## Assumptions

- The manifest's tools section accurately reflects what is installed in the image. Tools not listed in the manifest but pre-installed in the base image are not covered by automatic resolution (they rely on the well-known paths table).
- The well-known paths table is maintained as part of the cc-deck codebase and covers the most common tools and installation patterns for OpenShell sandbox images.
- OpenShell treats extra binary paths (paths that do not exist on the filesystem) as harmless. They are ignored at runtime without error.
- The `/usr/bin/<tool-name>` convention holds for tools installed via standard package managers on the supported base images (Debian/Ubuntu, Fedora).
