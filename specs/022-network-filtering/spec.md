# Feature Specification: Network Security and Domain Filtering

**Feature Branch**: `022-network-filtering`
**Created**: 2026-03-16
**Status**: Draft
**Input**: User description: "Network Security and Domain Filtering for containerized AI agent sessions"

## Clarifications

### Session 2026-03-16

- Q: When manifest has no `network` section, what should happen? → A: Skip network filtering (no proxy sidecar, open network). Filtering is opt-in via the `network` section.
- Q: Proxy image: upstream Squid or cc-deck-maintained? → A: Spec should not mandate proxy software. Use "lightweight forward proxy sidecar" and defer proxy choice (Squid, tinyproxy, etc.) to planning phase.
- Q: How does `cc-deck domains blocked` locate the proxy container? → A: Use existing session tracking (compose project name convention). No separate registration needed.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Deploy with Default Network Filtering (Priority: P1)

A user deploys a containerized Claude Code session using `cc-deck deploy --compose`. The system automatically generates a forward proxy sidecar alongside the session container, restricting all outbound traffic to only the domains required by the configured AI backend (Anthropic or Vertex AI) plus common development domains (GitHub). The user does not need to understand proxy configuration or network policy syntax.

**Why this priority**: Without network filtering, YOLO-mode containers can exfiltrate code and secrets to any destination. This is the foundational security capability that all other stories build on.

**Independent Test**: Can be tested by deploying a session with default settings and verifying that HTTP requests to allowed domains succeed while requests to arbitrary domains are blocked.

**Acceptance Scenarios**:

1. **Given** a valid build directory with `cc-deck-build.yaml`, **When** the user runs `cc-deck deploy --compose`, **Then** the generated `compose.yaml` includes a forward proxy sidecar, an internal network, and the session container is only attached to the internal network.
2. **Given** a deployed session with Anthropic backend, **When** the session container attempts to reach `api.anthropic.com`, **Then** the request succeeds through the proxy.
3. **Given** a deployed session with default filtering, **When** the session container attempts to reach `evil-exfil-server.com`, **Then** the request is blocked by the proxy and a log entry is created.
4. **Given** a deployed session, **When** the session container attempts an SSH connection (port 22), **Then** the connection fails because only HTTP/HTTPS traffic is routed through the proxy and all other protocols are blocked by network isolation.

---

### User Story 2 - Configure Domain Groups in Build Manifest (Priority: P1)

A user configuring a custom container image specifies which domain groups to allow in `cc-deck-build.yaml`. During the `/cc-deck.extract` phase, the system auto-detects project ecosystems (Go, Python, Node.js, Rust) and populates the `network.allowedDomains` section with the corresponding group names. The user can add, remove, or override groups before building.

**Why this priority**: Domain groups are the core configuration mechanism. Without them, users would need to list individual domains manually, which is error-prone and tedious.

**Independent Test**: Can be tested by running `/cc-deck.extract` on a project with a `go.mod` and `package.json`, then verifying that `golang` and `nodejs` appear in the manifest's `network.allowedDomains`.

**Acceptance Scenarios**:

1. **Given** a project containing `go.mod`, **When** the user runs `/cc-deck.extract`, **Then** `golang` is added to `network.allowedDomains` in the manifest.
2. **Given** a project containing `pyproject.toml`, **When** the user runs `/cc-deck.extract`, **Then** `python` is added to `network.allowedDomains` in the manifest.
3. **Given** a manifest with `allowedDomains: [python, golang]`, **When** `cc-deck deploy --compose` runs, **Then** the proxy allowlist includes all domains from both the `python` and `golang` groups.
4. **Given** a manifest with `allowedDomains: [python, custom.corp.com]`, **When** domain expansion runs, **Then** `python` is resolved as a group name (no dot) and `custom.corp.com` is passed through as a literal domain.

---

### User Story 3 - User-Defined Domain Groups (Priority: P2)

A user working in a corporate environment needs to allow access to internal package registries and artifact stores across multiple projects. They create custom domain groups in `~/.config/cc-deck/domains.yaml` that can be referenced by name in any build manifest. They can also extend built-in groups with additional domains (e.g., add an internal PyPI mirror to the `python` group).

**Why this priority**: Built-in groups cover open-source ecosystems, but enterprise environments need custom groups for internal infrastructure. This avoids repetitive domain listing across manifests.

**Independent Test**: Can be tested by creating a `domains.yaml` with a custom group, referencing it in a manifest, and verifying the expanded domain list includes both built-in and custom entries.

**Acceptance Scenarios**:

1. **Given** a `~/.config/cc-deck/domains.yaml` with a custom group `company` listing three domains, **When** the user references `company` in a manifest's `allowedDomains`, **Then** the three domains are included in the expanded domain list.
2. **Given** a `domains.yaml` entry for `python` with `extends: builtin` and one additional domain, **When** the `python` group is expanded, **Then** the result includes all built-in `python` domains plus the user-added domain.
3. **Given** a `domains.yaml` entry for `nodejs` without `extends`, **When** the `nodejs` group is expanded, **Then** only the user-defined domains are used (built-in `nodejs` domains are replaced).
4. **Given** a `domains.yaml` with a `dev-stack` group that includes `python`, `golang`, and `company`, **When** `dev-stack` is expanded, **Then** all domains from all three referenced groups are merged and deduplicated.
5. **Given** a `domains.yaml` with circular includes (group A includes group B, group B includes group A), **When** expansion runs, **Then** the system detects the cycle and reports an error instead of looping infinitely.

---

### User Story 4 - Seed and Explore Domain Definitions (Priority: P2)

A user wants to see what built-in domain groups are available and what domains they contain. They run `cc-deck domains init` to export built-in definitions to their config file as commented YAML, then `cc-deck domains list` and `cc-deck domains show <group>` to explore.

**Why this priority**: Discoverability is essential for users to understand and configure domain groups effectively. Without it, the feature is opaque.

**Independent Test**: Can be tested by running `cc-deck domains init` and verifying the output file exists with commented group definitions, then running `cc-deck domains show python` and verifying the domain list.

**Acceptance Scenarios**:

1. **Given** no existing `~/.config/cc-deck/domains.yaml`, **When** the user runs `cc-deck domains init`, **Then** the file is created with all built-in group definitions as commented YAML.
2. **Given** an existing `domains.yaml` with user modifications, **When** the user runs `cc-deck domains init`, **Then** user modifications are preserved and only missing built-in groups are appended (as comments).
3. **Given** built-in groups and user overrides, **When** the user runs `cc-deck domains list`, **Then** all available groups are listed with their source (built-in, user-defined, or extended).
4. **Given** a user-extended `python` group, **When** the user runs `cc-deck domains show python`, **Then** the output shows the merged domain list with annotations indicating which domains come from built-in vs user config.

---

### User Story 5 - Deploy-Time Domain Overrides (Priority: P2)

A user deploying a session needs to add or remove domain groups relative to the manifest defaults for a specific session, without modifying the manifest. They use CLI flags with `+` (add) and `-` (remove) syntax.

**Why this priority**: Deploy-time flexibility allows quick adjustments without editing the manifest, which is important for ad-hoc sessions and debugging.

**Independent Test**: Can be tested by deploying with `--allowed-domains +rust` and verifying Rust package registry domains are included in the proxy config alongside manifest defaults.

**Acceptance Scenarios**:

1. **Given** a manifest with `allowedDomains: [python, github]`, **When** the user deploys with `--allowed-domains +rust`, **Then** the session allows `python`, `github`, and `rust` domains.
2. **Given** a manifest with `allowedDomains: [python, github, nodejs]`, **When** the user deploys with `--allowed-domains -nodejs`, **Then** the session allows `python` and `github` but not `nodejs` domains.
3. **Given** a manifest with `allowedDomains: [python, github]`, **When** the user deploys with `--allowed-domains vertexai,rust` (no + prefix), **Then** the manifest defaults are replaced entirely with `vertexai` and `rust` only.

---

### User Story 6 - Kubernetes NetworkPolicy Generation (Priority: P3)

A user deploying to Kubernetes uses `cc-deck deploy --k8s`. The system generates NetworkPolicy resources that restrict Pod egress to allowed domains. On clusters with DNS-aware CNIs (Cilium, Calico), FQDN-based rules are generated. On standard K8s clusters, the existing CIDR-based approach is used as fallback.

**Why this priority**: K8s deployment is the secondary target after Podman. The existing `BuildNetworkPolicy()` code provides a foundation to build on.

**Independent Test**: Can be tested by generating K8s manifests and verifying the NetworkPolicy contains the expected egress rules for the configured domain groups.

**Acceptance Scenarios**:

1. **Given** a standard K8s cluster, **When** `cc-deck deploy --k8s` runs with domain groups, **Then** a NetworkPolicy with CIDR-based egress rules is generated (using DNS resolution at creation time).
2. **Given** a K8s cluster with Cilium CNI, **When** `cc-deck deploy --k8s` runs, **Then** a CiliumNetworkPolicy with FQDN-based egress rules is generated instead.

---

### User Story 7 - OpenShift EgressFirewall Generation (Priority: P3)

A user deploying to OpenShift uses `cc-deck deploy --k8s --openshift`. The system generates an EgressFirewall CRD with FQDN-based allowlists, leveraging the existing `BuildEgressFirewall()` function but now fed by the domain group expansion system instead of hardcoded backend host lists.

**Why this priority**: OpenShift is a secondary target, but already has partial implementation. This story refactors existing code to use the new domain group system.

**Independent Test**: Can be tested by generating OpenShift manifests and verifying the EgressFirewall contains FQDN rules for all expanded domain groups.

**Acceptance Scenarios**:

1. **Given** an OpenShift deployment with `allowedDomains: [python, github]`, **When** manifests are generated, **Then** the EgressFirewall contains Allow rules for all `python` and `github` domains plus backend domains, followed by a Deny-all rule.
2. **Given** the existing `backendDNSNames()` function, **When** this feature is implemented, **Then** backend DNS names are resolved through the domain group system instead of hardcoded switch statements.

---

### User Story 8 - Audit Blocked Requests (Priority: P3)

A user's agent session is failing because a legitimate domain is being blocked. The user runs `cc-deck domains blocked <session>` to see which requests were denied by the proxy. They then add the missing domain and redeploy or modify the running session.

**Why this priority**: Without audit capability, debugging blocked domains requires manual proxy log inspection. This is a quality-of-life feature that makes filtering practical.

**Independent Test**: Can be tested by making a request to a blocked domain in a session, then running the blocked command and verifying the domain appears in the output.

**Acceptance Scenarios**:

1. **Given** a running Podman session with proxy filtering, **When** the user runs `cc-deck domains blocked <session>`, **Then** a list of blocked domain requests is displayed with timestamps.
2. **Given** a blocked domain identified by the audit command, **When** the user adds it via `cc-deck domains add <session> <domain>`, **Then** the proxy is reconfigured and subsequent requests to that domain succeed.

---

### Edge Cases

- What happens when DNS resolution fails for a domain during K8s CIDR-based policy creation? (Fallback: allow all HTTPS, as current code does)
- What happens when a user specifies a group name that does not exist in built-in or user config? (Error with suggestion of available groups)
- What happens when `domains.yaml` has invalid YAML syntax? (Clear error message with file path and line number)
- What happens when wildcard `.example.com` and explicit `example.com` both appear? (Wildcard dedup removes the explicit entry, as Squid treats `.example.com` as covering both)
- What happens when `--allowed-domains all` is specified? (Network filtering is disabled entirely, with a warning about security implications)
- What happens when a name without dots does not match any known group? (Error with list of available groups, per FR-019)
- What happens when a user tries `cc-deck domains add` on a K8s/OpenShift session? (Not supported in v1; runtime modification is Podman-only via proxy reconfiguration)
- What happens when the manifest has no `network` section? (No proxy sidecar is generated; network is open. Filtering is opt-in.)

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST ship with built-in domain group definitions embedded in the binary (python, nodejs, rust, golang, github, gitlab, docker, quay)
- **FR-002**: System MUST automatically include backend-specific domains (Anthropic or Vertex AI) based on the configured backend, without user specification
- **FR-003**: System MUST load user-defined domain groups from `~/.config/cc-deck/domains.yaml`, merging with or overriding built-in groups
- **FR-004**: System MUST support `extends: builtin` in user domain config to merge user domains with built-in group definitions
- **FR-005**: System MUST support `includes` in domain groups for referencing other groups, with recursive expansion and cycle detection
- **FR-006**: System MUST distinguish group names from literal domains by convention (no dot = group name, contains dot = domain)
- **FR-007**: System MUST generate a forward proxy sidecar configuration in `compose.yaml` output with internal network isolation for Podman deployments
- **FR-008**: System MUST perform wildcard deduplication before generating proxy config (`.example.com` covers both exact and subdomain matches). Specific proxy software is a planning-phase decision.
- **FR-009**: System MUST generate Kubernetes NetworkPolicy with CIDR-based egress rules as the default K8s strategy
- **FR-010**: System MUST generate OpenShift EgressFirewall CRD with FQDN-based rules, replacing the current hardcoded `backendDNSNames()` with domain group expansion
- **FR-011**: System MUST support deploy-time domain overrides via `--allowed-domains` with `+` (add), `-` (remove), and bare (replace) syntax
- **FR-012**: System MUST provide `cc-deck domains init` to seed `~/.config/cc-deck/domains.yaml` with commented built-in definitions
- **FR-013**: System MUST provide `cc-deck domains list` to show all available groups with their source
- **FR-014**: System MUST provide `cc-deck domains show <group>` to display expanded domains for a group
- **FR-015**: System MUST provide `cc-deck domains blocked <session>` to display blocked requests from proxy logs (Podman only)
- **FR-016**: `/cc-deck.extract` MUST map detected project artifacts to domain groups (go.mod to golang, pyproject.toml to python, package.json to nodejs, Cargo.toml to rust)
- **FR-017**: System MUST support `--allowed-domains all` to disable network filtering entirely, with a visible security warning printed to stderr
- **FR-018**: System MUST support adding and removing domains on running Podman sessions by regenerating the proxy configuration without restarting the session container
- **FR-019**: When a name without dots is used in `allowedDomains` or `--allowed-domains` and it does not match any known group (built-in or user-defined), the system MUST report an error listing available groups rather than silently treating it as a literal domain

### Key Entities

- **Domain Group**: A named collection of domain patterns. Has a name (string without dots), a list of domain patterns, optional `extends` reference, and optional `includes` references to other groups. Source is either built-in (embedded) or user-defined (`domains.yaml`).
- **Domain Pattern**: A string representing an allowed domain. Can be exact (`api.anthropic.com`) or wildcard (`.github.com` covering all subdomains). Regex patterns (as used by paude with `~` prefix) are deferred to a future iteration.
- **Build Manifest Network Section**: The `network.allowedDomains` list in `cc-deck-build.yaml`. Contains group names and/or literal domain patterns. Resolved at deploy time.
- **Proxy Configuration**: Generated forward proxy allowlist rules for Podman deployments. Derived from expanded domain list after wildcard deduplication. Specific proxy software (Squid, tinyproxy, etc.) is a planning-phase decision.
- **Network Policy**: Generated Kubernetes/OpenShift resources (NetworkPolicy, CiliumNetworkPolicy, or EgressFirewall) for cluster deployments. Derived from expanded domain list.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A Podman deployment with a `network` section in the manifest blocks outbound requests to unauthorized domains within the first deploy
- **SC-002**: Users can configure domain allowlists using group names, reducing configuration from dozens of individual domains to 2-5 group references
- **SC-003**: User-defined domain groups in `domains.yaml` are resolvable and deployable without modifying the cc-deck binary
- **SC-004**: `/cc-deck.extract` correctly identifies at least 4 project ecosystems (Go, Python, Node.js, Rust) and maps them to domain groups
- **SC-005**: Existing K8s and OpenShift egress filtering continues to work after refactoring `backendDNSNames()` to use the domain group system
- **SC-006**: Users can identify and resolve blocked domain issues within 2 minutes using `cc-deck domains blocked`
- **SC-007**: Domain group expansion (including recursive includes and deduplication) completes in under 100ms for typical configurations
