# Feature Specification: Egress Recording Mode

**Feature Branch**: `062-egress-recording-mode`  
**Created**: 2026-05-24  
**Status**: Draft  
**Input**: User description: "A mode to start an OpenShell sandbox without protection in a recording mode, extract accessed egress URLs, and feed results into the catalog-driven build process as component files usable by cc-deck build"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Record Egress for a New Project (Priority: P1)

A developer sets up cc-deck for a new project for the first time. They have run `/cc-deck.capture` to detect tools and settings, but they are unsure which network endpoints their tools actually contact at runtime. Instead of manually researching domain lists for each tool, they run `cc-deck build record` to launch a recording session. During the session, they install dependencies, run builds, and use Claude Code normally. When they exit, cc-deck tells them exactly which domains were accessed and generates the appropriate policy files.

**Why this priority**: This is the core value proposition. Without this, users must manually research and maintain domain lists for their workspace tools, which is error-prone and incomplete.

**Independent Test**: Can be fully tested by running `cc-deck build record` on a project with known dependencies (e.g., a Python project with pip install), verifying the DNS log captures the expected domains (pypi.org, files.pythonhosted.org), and confirming they are added to `build.yaml` `network.allowed_domains`.

**Acceptance Scenarios**:

1. **Given** a project with a valid build.yaml manifest and a built workspace image, **When** the user runs `cc-deck build record`, **Then** a Podman pod is created with the workspace container and a DNS logger sidecar, and the user is attached interactively to the workspace container.

2. **Given** an active recording session where the user installs Python packages via pip, **When** the user exits the session, **Then** cc-deck extracts the DNS query log and reports that `pypi.org` and `files.pythonhosted.org` were observed.

3. **Given** a completed recording session with observed domains, **When** cc-deck processes the results, **Then** new domains are appended to `build.yaml` `network.allowed_domains` and the user is shown a summary of discovered vs. already-known domains.

---

### User Story 2 - Catalog Reverse Matching (Priority: P2)

After a recording session completes, cc-deck matches the observed domains against the existing catalog components and `allowed_domains` already in the manifest. Domains that match known components (e.g., `pypi.org` matches the `python` component) or are already listed in `allowed_domains` are reported as "already covered." Only truly new domains are appended to the manifest.

**Why this priority**: Without reverse matching, the recording output would duplicate domains already handled by the catalog or manifest. This deduplication makes the output actionable and tells the user exactly what is new.

**Independent Test**: Can be tested by running a recording session on a project with known tools (Python, Go) and verifying that the output separates catalog-matched domains from novel ones.

**Acceptance Scenarios**:

1. **Given** a recording session that observed `pypi.org`, `crates.io`, and `internal-api.corp.example.com`, **When** cc-deck processes the results with catalog components for Python and Rust loaded, **Then** the report shows `pypi.org` and `crates.io` as "covered by catalog" and `internal-api.corp.example.com` as "new, added to `allowed_domains`."

2. **Given** a recording session where all observed domains are already covered by catalog components or existing `allowed_domains`, **When** cc-deck processes the results, **Then** no manifest changes are made and the user is told "all observed domains are already covered."

---

### User Story 3 - Integrate Recorded Domains into Build Pipeline (Priority: P2)

After reviewing the recording results, the user runs `cc-deck build refresh` and the recorded domains in `allowed_domains` are picked up automatically. For OpenShell targets, the domains are assembled into the policy as network policies. The user does not need to manually edit any policy files.

**Why this priority**: The recording is only useful if the results flow seamlessly into the existing build pipeline. This story ensures the output integrates with the existing `allowed_domains` mechanism.

**Independent Test**: Can be tested by running a recording session, then running `cc-deck build refresh`, and verifying the generated policy includes the recorded domains.

**Acceptance Scenarios**:

1. **Given** recorded domains exist in `build.yaml` `network.allowed_domains`, **When** the user runs `cc-deck build refresh`, **Then** the policy generator includes the recorded domains alongside catalog and embedded components.

2. **Given** a manifest with an OpenShell target and recorded domains in `allowed_domains`, **When** the user runs `cc-deck build run --target openshell`, **Then** the built image contains a policy file that includes endpoints from the recording session.

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

- What happens when the workspace image has not been built yet? The command MUST error with a clear message: "workspace image not found, run `cc-deck build run --target openshell` first." It MUST NOT auto-build.
- What happens when Podman is not installed or not running? The command should fail with a clear error message.
- What happens when the DNS sidecar fails to start? The recording session should not proceed; the user should see the sidecar error.
- What happens when the user exits immediately without performing any work? The DNS log will be empty or contain only infrastructure noise. The command should report "no meaningful egress domains observed" and not modify the manifest.
- What happens when the DNS sidecar image is not available locally? The command should pull it automatically before starting the pod.
- What happens when the user runs `build record` multiple times? Each session appends only new domains to `allowed_domains`. Domains already present (from prior sessions or manual additions) are not duplicated.

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
- **FR-009**: The system MUST deduplicate observed domains and filter out infrastructure noise using a hardcoded deny-list of known noise patterns: domains ending in `.local`, `.internal`, or `.podman`; the literal `localhost`; and AAAA-only queries that have no corresponding A record. The deny-list is extensible in future versions but not user-configurable in this feature.
- **FR-010**: The system MUST match observed domains against loaded catalog components (embedded, cached, and user-local) and existing `network.allowed_domains` entries, classifying each domain as "already covered" or "new."
- **FR-011**: For domains not already covered, the system MUST append them to the `network.allowed_domains` list in `build.yaml`.
- **FR-012**: The system MUST NOT duplicate domains that are already present in `allowed_domains` or covered by catalog component endpoints.
- **FR-013**: The system MUST present a summary report to the user showing: total domains observed, domains already covered (by catalog or manifest), new domains added to `allowed_domains`, and a reminder to run `cc-deck build refresh`.
- **FR-014**: The recording mechanism (Podman pod + DNS sidecar) MUST be target-agnostic. The output (`allowed_domains` entries) is consumed by the policy assembly pipeline for all target types that support it.
- **FR-015**: The system MUST use port 443 for all generated policy endpoints derived from recorded domains. DNS queries do not contain port information.

### Key Entities

- **Recording Session**: A single interactive Podman pod session with a workspace container and DNS sidecar. Produces a DNS query log.
- **DNS Query Log**: A structured log file containing all DNS queries made during the recording session, with timestamps and query types.
- **Catalog Match Result**: The classification of each observed domain as either already covered (by catalog component endpoints or existing `allowed_domains`) or new (to be appended to `allowed_domains`).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user can discover all external domains their workspace tools contact in a single interactive session, without prior knowledge of which endpoints each tool needs.
- **SC-002**: The recording session adds no more than 30 seconds of overhead beyond normal workspace startup time (sidecar pull and pod creation).
- **SC-003**: Post-session processing (log extraction, deduplication, catalog matching, file generation) completes in under 10 seconds for sessions with up to 500 unique domains.
- **SC-004**: The Podman pod MUST be configured so that all DNS resolution routes through the sidecar (shared network namespace, `--dns` set to the sidecar's listener address). This architectural constraint ensures no queries bypass the logger.
- **SC-005**: The updated `allowed_domains` entries are immediately usable by `cc-deck build refresh` without manual editing.
- **SC-006**: Infrastructure noise (Podman-internal DNS, mDNS, localhost) is filtered from the output with zero false positives (no legitimate egress domains are removed).

## Out of Scope

- Non-interactive (CI-driven) recording mode
- Binary-to-endpoint mapping from recording data (handled by catalog components and two-pass probing)
- User-configurable noise filter lists (hardcoded deny-list only in this feature)
- Generating component YAML files from recorded data (domains go to `allowed_domains` instead)

## Clarifications

### Session 2026-05-24

- Q: How should repeated recording sessions interact with prior results? → A: Append-only. Each session adds new domains to `allowed_domains`; existing entries are not duplicated or removed.
- Q: Where should recorded domains be stored? → A: Directly in `build.yaml` `network.allowed_domains`. No component YAML files generated. Simpler, user-visible, target-agnostic, and already integrated with `build refresh`.
- Q: What constitutes the DNS noise filter boundary? → A: Hardcoded deny-list of known noise patterns (`.local`, `localhost`, `dns.podman`, `*.internal`) plus AAAA-only dedup.
- Q: What should happen when the workspace image does not exist? → A: Error with guidance message, never auto-build.
- Q: How should SC-004 (DNS capture completeness) be verified? → A: Reframe as architectural constraint (pod config with shared namespace + `--dns`), not an empirical 100% claim.

## Assumptions

- Podman is installed and available on the user's system (cc-deck already requires Podman for container targets).
- The workspace image has already been built via `cc-deck build run` before running `build record`. If not, the command will error with guidance.
- The CoreDNS (or equivalent DNS logger) container image is publicly available and can be pulled without authentication.
- All meaningful developer tool egress uses DNS resolution before connecting (tools that use hardcoded IP addresses without DNS will not be captured, but this is extremely rare for public services).
- Port 443 (HTTPS) is the correct default for virtually all developer tool egress. Non-443 traffic (e.g., SSH on port 22, custom API ports) is a small minority and can be manually added.
- The user exercises a representative workflow during the recording session (the feature discovers only what is actually accessed, not what could be accessed).
- The `build.yaml` manifest format and `network.allowed_domains` field are stable and will not change during this feature's development.
