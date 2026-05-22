# Feature Specification: Deterministic Policy Generation

**Feature Branch**: `059-deterministic-policy-generation`  
**Created**: 2026-05-22  
**Status**: Draft  
**Input**: Move OpenShell network policy generation from non-deterministic Claude Code reasoning to declarative YAML component files with a two-tier catalog (remote GitHub repo + embedded fallback)

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Policy generated deterministically from manifest (Priority: P1)

A developer runs `cc-deck build refresh` after updating their `build.yaml` manifest. The command reads the manifest, matches detected tools and credentials against policy component files, and writes `openshell/policy.yaml` with the correct endpoints, binary globs, and access fields. The resulting policy is identical every time the same manifest is used, regardless of which Claude model is active or how many times the command runs.

**Why this priority**: This eliminates the root cause of policy failures (non-deterministic generation). Without this, every `/cc-deck.build` run can produce a subtly different policy that may fail OpenShell 0.0.46 validation.

**Independent Test**: Run `cc-deck build refresh` twice with the same manifest. Verify both runs produce byte-identical `openshell/policy.yaml`. Verify the policy includes the correct endpoints and binary globs for each detected tool and credential type.

**Acceptance Scenarios**:

1. **Given** a `build.yaml` with `tools: [{name: cargo}]` and `credentials: [{type: claude}]`, **When** the user runs `cc-deck build refresh`, **Then** `openshell/policy.yaml` contains a `claude_code` section with `api.anthropic.com` (protocol: rest, access: full), `downloads.claude.ai`, `raw.githubusercontent.com`, and the Claude Code binary glob (`/sandbox/.local/share/claude/**`), plus a `rust_crates` section with `crates.io`, `static.crates.io`, `index.crates.io`.
2. **Given** the same manifest, **When** `cc-deck build refresh` is run a second time, **Then** the output is byte-identical to the first run.
3. **Given** a `build.yaml` with `credentials: [{type: claude-vertex}]`, **When** `cc-deck build refresh` runs, **Then** the policy contains a `vertex_ai` section with bare `aiplatform.googleapis.com`, regional endpoints, `oauth2.googleapis.com`, `www.googleapis.com`, `accounts.google.com`, all with the Claude Code binary glob.

---

### User Story 2 - Component files define policy fragments (Priority: P1)

A developer wants to understand or customize what endpoints are allowed for a given tool. They open the component YAML file (e.g., `internal/build/policies/rust.yaml`) and see a self-contained declaration of endpoints, binaries, and match conditions. No Go code changes are needed to add, remove, or modify endpoints.

**Why this priority**: Decoupling endpoint definitions from code makes the policy auditable, maintainable, and extensible without recompilation.

**Independent Test**: Edit an embedded component file (e.g., add a new endpoint to `rust.yaml`). Rebuild the binary. Run `build refresh`. Verify the new endpoint appears in the generated policy.

**Acceptance Scenarios**:

1. **Given** a component file with `match: { tools: [cargo, rust] }` and endpoints for `crates.io:443`, **When** the manifest contains `tools: [{name: cargo}]`, **Then** the component matches and its endpoints appear in the policy.
2. **Given** a component file with `match: { tools: [go] }` and the manifest has no Go tool, **When** `build refresh` runs, **Then** the component does not appear in the policy.
3. **Given** a component file with `match: { always: true }`, **When** `build refresh` runs, **Then** the component always appears in the policy regardless of manifest content.

---

### User Story 3 - Remote catalog updates components without binary release (Priority: P2)

A new version of Claude Code starts reaching `platform.claude.com` on startup. The cc-deck team adds this endpoint to the `claude-code.yaml` component in the catalog repo. Users who run `cc-deck.capture` fetch the updated component automatically. Their next `build refresh` produces a policy with the new endpoint, without upgrading the cc-deck binary.

**Why this priority**: Policy requirements change when upstream tools change. Decoupling updates from binary releases reduces the time between discovering a missing endpoint and fixing it for all users.

**Independent Test**: Modify a component in the catalog repo. Run `cc-deck.capture` in a project. Verify the updated component is cached locally. Run `build refresh`. Verify the new endpoint appears in the policy.

**Acceptance Scenarios**:

1. **Given** a catalog repo with an updated `claude-code.yaml` containing `platform.claude.com:443`, **When** the user runs `cc-deck.capture`, **Then** the updated component is downloaded and cached in `.cc-deck/setup/openshell/components/`.
2. **Given** cached catalog components and embedded components with the same name, **When** `build refresh` assembles the policy, **Then** the cached (catalog) version takes precedence over the embedded version.
3. **Given** no network connectivity, **When** the user runs `cc-deck.capture`, **Then** capture warns that the catalog is unreachable and continues. `build refresh` uses the embedded fallback components.

---

### User Story 4 - User-local policy overrides (Priority: P3)

A developer has internal MCP servers and custom APIs that need policy entries. They create component files in `.cc-deck/setup/openshell/policies/` following the same YAML format. These are merged on top of catalog and embedded components during `build refresh`.

**Why this priority**: Projects have unique endpoint requirements that do not belong in the shared catalog. Local overrides allow customization without forking.

**Independent Test**: Create a custom component file in `.cc-deck/setup/openshell/policies/`. Run `build refresh`. Verify the custom endpoints appear in the generated policy alongside the standard components.

**Acceptance Scenarios**:

1. **Given** a file `.cc-deck/setup/openshell/policies/internal-api.yaml` with endpoints for `api.internal.corp:8443`, **When** `build refresh` runs, **Then** the policy contains those endpoints.
2. **Given** a user-local component with the same name as a catalog component, **When** `build refresh` runs, **Then** the user-local version takes precedence.
3. **Given** a user-local component with `match: { always: true }`, **When** `build refresh` runs, **Then** the component is always included.

---

### Edge Cases

- What happens when a component file has invalid YAML? The system should report the filename and error, skip the invalid component, and continue processing the remaining components.
- What happens when two components define the same policy key (e.g., both name their output `claude_code`)? The later one in resolution order (embedded < catalog < user-local) wins entirely, replacing the earlier one.
- What happens when the catalog repo URL is unreachable and no components are cached locally? The embedded fallback is used. This is the first-run experience for offline users.
- What happens when the manifest has no tools and no credentials? Only `always: true` components (claude-code, git-hosting) are included.
- What happens when `targets.openshell.policy` is set in the manifest? Explicit overrides are merged on top of the assembled policy using existing merge semantics (replace by host match).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Policy generation MUST be deterministic: the same manifest and component files MUST produce byte-identical output on every run.
- **FR-002**: Endpoints and binary paths MUST NOT be hardcoded in Go source code. They MUST be defined in declarative YAML component files.
- **FR-003**: Each component file MUST declare its own match conditions (`always`, `tools`, `credentials`, `features`) and the system MUST evaluate them against the manifest.
- **FR-004**: Each component file MUST declare its own binary paths. The system MUST NOT assume which binaries use which endpoints.
- **FR-005**: `cc-deck build refresh` MUST regenerate `openshell/policy.yaml` from components when an openshell target is configured in the manifest.
- **FR-006**: Component resolution MUST follow the precedence order: embedded (lowest) < cached catalog < user-local (highest).
- **FR-007**: `cc-deck.capture` MUST attempt to fetch the catalog index from the remote repo when network is available, and cache matching components locally.
- **FR-008**: When the catalog is unreachable, capture MUST warn and continue. `build refresh` MUST fall back to embedded components.
- **FR-009**: The `/cc-deck.build` command MUST NOT generate policy. It MUST use the pre-rendered `openshell/policy.yaml` produced by `build refresh`.
- **FR-010**: The generated policy MUST comply with OpenShell 0.0.46 requirements: `rest` protocol endpoints MUST have `access` or `rules`, deprecated fields (`tls: terminate`, `enforcement: enforce`) MUST NOT appear.

### Key Entities

- **Component File**: A YAML document declaring a named policy fragment with match conditions, endpoints, and binary paths. Lives embedded in the binary, in the catalog repo, or in the user's project.
- **Catalog**: A GitHub repo serving as a versioned registry of component files. Contains a `catalog.yaml` index and individual component files.
- **Manifest**: The `build.yaml` file containing tool, credential, and domain declarations that drive component matching.
- **Policy File**: The assembled `openshell/policy.yaml` output, ready for COPY into the container image.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Running `cc-deck build refresh` twice with the same manifest produces byte-identical policy output in 100% of cases.
- **SC-002**: Adding a new endpoint to a component file and running `build refresh` reflects the change in the policy without modifying Go code.
- **SC-003**: A user with no network connectivity can generate a working policy using only embedded components.
- **SC-004**: Updating a component in the catalog repo is reflected in user policy within one `capture` + `refresh` cycle, without upgrading the cc-deck binary.
- **SC-005**: The generated policy passes OpenShell 0.0.46 supervisor validation on first sandbox creation, with no manual `openshell policy set` needed.

## Assumptions

- The catalog repo is owned by the cc-deck project and trusted by default. Supply chain signing is deferred to a future spec.
- The embedded component files are a snapshot from the last cc-deck release. They may lag behind the catalog.
- MCP server endpoints are not yet handled by the component system. They require a separate mechanism (either captured explicitly into the manifest or parsed from MCP config). This is an open question for a future spec.
- Binary paths are well-known constants for standard tools. Discovery from inside the image (e.g., `which go`) is not needed.
- The catalog index (`catalog.yaml`) is small enough to fetch in a single HTTP request during capture.
