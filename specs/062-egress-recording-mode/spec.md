# Feature Specification: Egress Recording Mode

**Feature Branch**: `062-egress-recording-mode`  
**Created**: 2026-05-24  
**Status**: Draft  
**Input**: User description: "A mode to start an OpenShell sandbox without protection in a recording mode, extract accessed egress URLs, and feed results into the catalog-driven build process as component files usable by cc-deck build"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Record Egress for a New Project (Priority: P1)

A developer sets up cc-deck for a new project for the first time. They have run `/cc-deck.capture` to detect tools and settings, but they are unsure which network endpoints their tools actually contact at runtime. Instead of manually researching domain lists for each tool, they run `cc-deck build record` to launch a recording session. During the session, they install dependencies, run builds, and use Claude Code normally. When they exit, cc-deck tells them exactly which domains were accessed and generates the appropriate policy files.

**Why this priority**: This is the core value proposition. Without this, users must manually research and maintain domain lists for their workspace tools, which is error-prone and incomplete.

**Independent Test**: Can be fully tested by running `cc-deck build record` on a project with known dependencies (e.g., a Python project with pip install), verifying the DNS log captures the expected domains (pypi.org, files.pythonhosted.org), and confirming component files are generated.

**Acceptance Scenarios**:

1. **Given** a project with a valid build.yaml manifest and a built workspace image, **When** the user runs `cc-deck build record`, **Then** a Podman pod is created with the workspace container and a DNS logger sidecar, and the user is attached interactively to the workspace container.

2. **Given** an active recording session where the user installs Python packages via pip, **When** the user exits the session, **Then** cc-deck extracts the DNS query log and reports that `pypi.org` and `files.pythonhosted.org` were observed.

3. **Given** a completed recording session with observed domains, **When** cc-deck processes the results, **Then** catalog-compatible component YAML files are written to the setup directory and the user is shown a summary of discovered vs. already-known domains.

---

### User Story 2 - Catalog Reverse Matching (Priority: P2)

After a recording session completes, cc-deck matches the observed domains against the existing catalog components. Domains that match known components (e.g., `pypi.org` matches the `python` component) are reported as "already covered." Domains that do not match any catalog component are grouped into a new `recorded-custom` component for the user to review.

**Why this priority**: Without reverse matching, the recording output would duplicate domains already handled by the catalog. This deduplication makes the output actionable and tells the user exactly what is new.

**Independent Test**: Can be tested by running a recording session on a project with known tools (Python, Go) and verifying that the output separates catalog-matched domains from novel ones.

**Acceptance Scenarios**:

1. **Given** a recording session that observed `pypi.org`, `crates.io`, and `internal-api.corp.example.com`, **When** cc-deck processes the results with catalog components for Python and Rust loaded, **Then** the report shows `pypi.org` and `crates.io` as "covered by catalog" and `internal-api.corp.example.com` as "new, added to recorded-custom component."

2. **Given** a recording session where all observed domains are already covered by catalog components, **When** cc-deck processes the results, **Then** no new component file is generated and the user is told "all observed domains are already in the catalog."

---

### User Story 3 - Integrate Recorded Domains into Build Pipeline (Priority: P2)

After reviewing the recording results, the user runs `cc-deck build refresh` and the recorded component files are picked up automatically. For OpenShell targets, the recorded endpoints are assembled into the policy. For container targets, the recorded domains are available as `allowed_domains`. The user does not need to manually edit any policy or manifest files.

**Why this priority**: The recording is only useful if the results flow seamlessly into the existing build pipeline. This story ensures the output format is compatible with downstream consumers.

**Independent Test**: Can be tested by running a recording session, then running `cc-deck build refresh`, and verifying the generated policy includes the recorded endpoints.

**Acceptance Scenarios**:

1. **Given** recorded component files exist in the setup directory, **When** the user runs `cc-deck build refresh`, **Then** the policy generator includes the recorded endpoints alongside catalog and embedded components.

2. **Given** a manifest with an OpenShell target and recorded components, **When** the user runs `cc-deck build run --target openshell`, **Then** the built image contains a policy file that includes endpoints from the recording session.

---

### User Story 4 - Noise Filtering (Priority: P3)

During a recording session, the DNS sidecar captures all queries, including infrastructure noise (container registry lookups, internal Podman DNS names, mDNS, duplicate queries for the same domain). The post-processing step filters out this noise so the user sees only meaningful egress domains.

**Why this priority**: Without filtering, the output contains dozens of irrelevant domains that obscure the actual tool egress. Filtering is important for usability but not for core functionality.

**Independent Test**: Can be tested by running a recording session and verifying that known noise domains (e.g., `dns.podman`, local hostnames, AAAA-only queries for IPv4 services) are excluded from the output.

**Acceptance Scenarios**:

1. **Given** a recording session where the DNS log contains queries for `pypi.org`, `dns.podman`, `localhost`, and `_dnssd._udp.local`, **When** cc-deck processes the results, **Then** only `pypi.org` appears in the output.

2. **Given** a recording session where the same domain is queried 50 times (both A and AAAA records), **When** cc-deck processes the results, **Then** the domain appears exactly once in the output.

---

### Edge Cases

- What happens when the workspace image has not been built yet? The command should build it first or error with a clear message pointing the user to `cc-deck build run`.
- What happens when Podman is not installed or not running? The command should fail with a clear error message.
- What happens when the DNS sidecar fails to start? The recording session should not proceed; the user should see the sidecar error.
- What happens when the user exits immediately without performing any work? The DNS log will be empty or contain only infrastructure noise. The command should report "no meaningful egress domains observed" and not generate component files.
- What happens when the DNS sidecar image is not available locally? The command should pull it automatically before starting the pod.
- What happens when the user runs `build record` multiple times? Each session should produce a separate recording. The user should be offered to merge results or replace previous recordings.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST provide a `cc-deck build record` subcommand that orchestrates a DNS-recording session using a Podman pod.
- **FR-002**: The recording session MUST create a Podman pod containing the workspace container and a DNS logger sidecar container sharing the same network namespace.
- **FR-003**: The workspace container MUST use the image specified in the build manifest (same image that `build run` produces).
- **FR-004**: The DNS logger sidecar MUST capture all DNS queries made by the workspace container and write them to a structured log file.
- **FR-005**: The workspace container MUST resolve DNS through the sidecar (via the pod's shared localhost network) so all queries are captured.
- **FR-006**: The recording session MUST allow full outbound network access (no domain restrictions) so the user can exercise all tools normally.
- **FR-007**: The user MUST be attached interactively to the workspace container during the recording session.
- **FR-008**: On session exit, the system MUST extract the DNS query log from the sidecar container.
- **FR-009**: The system MUST deduplicate observed domains and filter out infrastructure noise (internal Podman DNS, mDNS, localhost, container registries used during image pull).
- **FR-010**: The system MUST match observed domains against loaded catalog components (embedded, cached, and user-local) and classify each domain as "catalog-covered" or "new."
- **FR-011**: For domains not covered by the catalog, the system MUST generate one or more catalog-compatible component YAML files in the user-local overrides directory (`.cc-deck/setup/openshell/policies/`).
- **FR-012**: The generated component files MUST follow the same format as existing catalog components (name, match, endpoints fields) so they are consumed by `AssemblePolicy()` and `build refresh` without modification.
- **FR-013**: The system MUST present a summary report to the user showing: total domains observed, domains already in catalog, new domains added, and the path to generated files.
- **FR-014**: The system MUST be target-agnostic. The recording mechanism (Podman pod + DNS sidecar) works independently of whether the target is OpenShell, container, or (future) Kubernetes.
- **FR-015**: The system MUST assume port 443 for all observed domains unless additional port information is available from the DNS log context.

### Key Entities

- **Recording Session**: A single interactive Podman pod session with a workspace container and DNS sidecar. Produces a DNS query log.
- **DNS Query Log**: A structured log file containing all DNS queries made during the recording session, with timestamps and query types.
- **Recorded Component**: A catalog-compatible YAML file generated from observed domains not covered by existing catalog components. Stored in the user-local overrides directory.
- **Catalog Match Result**: The classification of each observed domain as either covered by an existing catalog component or new (requiring a recorded component).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user can discover all external domains their workspace tools contact in a single interactive session, without prior knowledge of which endpoints each tool needs.
- **SC-002**: The recording session adds no more than 30 seconds of overhead beyond normal workspace startup time (sidecar pull and pod creation).
- **SC-003**: Post-session processing (log extraction, deduplication, catalog matching, file generation) completes in under 10 seconds for sessions with up to 500 unique domains.
- **SC-004**: 100% of DNS queries made by tools inside the workspace container are captured by the sidecar (no queries bypass the logger).
- **SC-005**: The generated component files are immediately usable by `cc-deck build refresh` without manual editing.
- **SC-006**: Infrastructure noise (Podman-internal DNS, mDNS, localhost) is filtered from the output with zero false positives (no legitimate egress domains are removed).

## Assumptions

- Podman is installed and available on the user's system (cc-deck already requires Podman for container targets).
- The workspace image has already been built via `cc-deck build run` before running `build record`. If not, the command will error with guidance.
- The CoreDNS (or equivalent DNS logger) container image is publicly available and can be pulled without authentication.
- All meaningful developer tool egress uses DNS resolution before connecting (tools that use hardcoded IP addresses without DNS will not be captured, but this is extremely rare for public services).
- Port 443 (HTTPS) is the correct default for virtually all developer tool egress. Non-443 traffic (e.g., SSH on port 22, custom API ports) is a small minority and can be manually added.
- The user exercises a representative workflow during the recording session (the feature discovers only what is actually accessed, not what could be accessed).
- The catalog component format and the user-local overrides directory (`.cc-deck/setup/openshell/policies/`) are stable and will not change during this feature's development.
