# Research: Unified Setup Command

**Feature**: 034-unified-setup-command
**Date**: 2026-04-08

## Research Topics

### RT-1: Existing `cc-deck image` Architecture

**Question**: How is the current image command structured, and what can be reused?

**Findings**:
- The image command lives in `internal/cmd/build.go` with three subcommands: `init`, `verify`, `diff`
- It uses `internal/build/` package for manifest loading, validation, and scaffolding
- Three Claude commands are embedded via `go:embed` in `internal/build/commands/`: `cc-deck.capture.md`, `cc-deck.build.md`, `cc-deck.push.md`
- The manifest struct (`internal/build/manifest.go`) defines `Manifest` with fields: Version, Image, Tools, Sources, Plugins, MCP, GithubTools, Settings, Network
- The init command scaffolds `.cc-deck/image/` with manifest template, Claude commands to `.claude/commands/`, and helper scripts

**Decision**: Rename `internal/build` to `internal/setup`. Extend the manifest with a `Targets` field containing `Container` and `SSH` sub-structs. The existing `ImageConfig` becomes `ContainerTarget`. Keep all existing functionality intact.

**Rationale**: The existing package is well-structured and handles manifest loading, validation, scaffolding, and command embedding. Extending it is less risky than rewriting.

**Alternatives considered**: Creating a separate `internal/ansible` package was rejected because it would fragment the manifest handling. The unified manifest needs a single owner.

---

### RT-2: SSH Pre-flight Bootstrap (Current State)

**Question**: What does the current SSH bootstrap do, and what replaces it?

**Findings**:
- `internal/ssh/bootstrap.go` implements an interactive check sequence with remediation for: SSH connectivity, OS/arch detection, Zellij, Claude Code, cc-deck CLI, cc-deck plugin, credential verification
- Each check has auto-remediation (download and install) with fallback to manual instructions
- Testing (F-001 through F-010) revealed 10 bugs in this approach: wrong URLs, wrong architecture names, missing shell sourcing, absolute local paths in layouts, npm instead of official installer
- The root cause is that procedural bootstrapping is fragile. Each remedy is a one-off script with no idempotency guarantee

**Decision**: Delete `internal/ssh/bootstrap.go`. Replace the `Create()` pre-flight with a lightweight probe: `ssh user@host 'which zellij && which cc-deck && which claude'`. If probe fails, error with "Host appears unprovisioned. Run `cc-deck setup` first."

**Rationale**: Ansible playbooks are idempotent, testable independently, and runnable without Claude Code. The pre-flight bootstrap tried to do too much in a single pass with no correction mechanism.

**Alternatives considered**: Keeping bootstrap as a fallback was rejected because maintaining two provisioning paths doubles the bug surface.

---

### RT-3: Ansible Role Structure and Best Practices

**Question**: How should the Ansible roles be structured for the SSH provisioning backend?

**Findings**:
- Ansible best practice: one role per concern, with `tasks/main.yml`, `defaults/main.yml`, `templates/`, and optional `handlers/main.yml`
- The brainstorm defines 7 roles: `base`, `tools`, `zellij`, `claude`, `cc-deck`, `shell-config`, `mcp`
- Each role maps to a logical concern and can be tested independently
- Roles should use `become: true` for system-level changes and drop privileges for user-level config
- The `site.yml` playbook includes all roles in dependency order

**Decision**: Use the 7-role structure from the brainstorm. Each role gets a skeleton with `tasks/main.yml` and `defaults/main.yml`. The `/cc-deck.build --target ssh` command generates the task content from the manifest; skeletons are just empty structure for init.

**Rationale**: Role-per-concern maps cleanly to the manifest sections (tools, settings, plugins). It also allows selective re-runs (`ansible-playbook site.yml --tags tools`).

**Alternatives considered**: A single monolithic playbook was rejected because it cannot be selectively re-run and is harder to debug.

---

### RT-4: Manifest Schema Evolution

**Question**: How should the manifest evolve from `cc-deck-image.yaml` to `cc-deck-setup.yaml`?

**Findings**:
- Current manifest (`Manifest` struct) has `Image ImageConfig` with fields: Name, Tag, Base
- The brainstorm proposes a `targets` section with `container` and `ssh` sub-sections
- The shared sections (tools, sources, plugins, mcp, github_tools, settings) remain identical
- Container target fields: name, tag, base, registry (new)
- SSH target fields: host, port, identity_file, create_user, user, workspace

**Decision**: Replace `Image ImageConfig` with `Targets TargetsConfig`. The `ContainerTarget` preserves all existing `ImageConfig` fields plus adds `Registry`. The `SSHTarget` is new. The manifest file renames from `cc-deck-image.yaml` to `cc-deck-setup.yaml`.

**Rationale**: The shared sections are already target-agnostic. Only the `Image` field is container-specific. Moving it under `targets.container` is a clean structural change.

**Alternatives considered**: Keeping separate manifests per target was rejected because it defeats the "single capture, dual target" value proposition.

---

### RT-5: Ansible Inventory Generation

**Question**: How should the Ansible inventory be generated from the manifest?

**Findings**:
- Ansible inventory can be static (INI file) or dynamic (script/plugin)
- For a single-host provisioning scenario, a static INI file is simplest
- The manifest's `targets.ssh` section provides: host, port, identity_file
- Ansible needs: inventory host entry, SSH connection variables (`ansible_host`, `ansible_port`, `ansible_ssh_private_key_file`, `ansible_user`)

**Decision**: Generate a static `inventory.ini` from the manifest SSH target section. The `/cc-deck.build --target ssh` command regenerates inventory on each run. Variables from the manifest are placed in `group_vars/all.yml`.

**Rationale**: Static inventory is sufficient for single-host provisioning. Dynamic inventory adds complexity without benefit for this use case.

**Alternatives considered**: Dynamic inventory script was rejected as over-engineering for a single-host target.

---

### RT-6: Package Manager Support

**Question**: Should initial implementation support both dnf and apt?

**Findings**:
- The testing target is Fedora 43 (Hetzner VM). The base image is also Fedora-based.
- The existing Containerfile generation uses `dnf` exclusively
- Supporting apt requires conditional logic in every role that installs packages
- Ansible's `package` module abstracts package managers, but package names differ between distros (e.g., `python3-devel` vs `python3-dev`)

**Decision**: Start with dnf (Fedora/RHEL) only. The `base` role detects OS family and stores it as a fact. Package name mappings can be added later via role variables. The Claude command's self-correction loop can handle package name mismatches at generation time.

**Rationale**: Supporting one package manager well is better than supporting two poorly. The self-correction loop already handles package name mismatches for containers; the same approach works for SSH.

**Alternatives considered**: Ansible's `package` module was considered but rejected because package names still differ. The abstraction hides the package manager but not the naming differences.

---

### RT-7: Version Pinning Strategy

**Question**: How should tool versions be pinned in the Ansible playbooks?

**Findings**:
- Zellij: downloaded from GitHub Releases. Version can be pinned to match the development machine.
- cc-deck: downloaded from GitHub Releases. Version should match the local `cc-deck version` to ensure remote and local are compatible.
- Claude Code: installs via `curl -fsSL https://claude.ai/install.sh | bash`. Auto-updates itself. No pinning needed.
- System packages (via dnf): latest from repo by default. Pinning requires explicit version strings.

**Decision**: Pin cc-deck to the locally installed version (from `cc-deck version -o json`). Pin Zellij to a known-good version stored in the manifest or role defaults. Do not pin Claude Code (it auto-updates). Do not pin system packages (latest is fine for dev environments).

**Rationale**: cc-deck version matching between local and remote prevents protocol mismatches. Zellij pinning prevents plugin SDK incompatibilities. Claude Code handles its own updates.

---

### RT-8: Migration from `cc-deck image`

**Question**: Should the init command detect old-style `.cc-deck/image/` directories?

**Decision**: No automatic migration. The spec explicitly states "no backwards compatibility needed." If an old directory exists, the init command creates `.cc-deck/setup/` alongside it. Users can manually copy their manifest.

**Rationale**: There are no external users. The developer (single user) can handle the migration manually. Adding migration code for a one-time operation violates YAGNI (Constitution Principle VIII).

---

### RT-9: Claude Command Target Dispatch

**Question**: How should `/cc-deck.build` dispatch between container and SSH targets?

**Findings**:
- The current `/cc-deck.build` command is a single Markdown file with step-by-step instructions for Containerfile generation and image building
- Adding `--target ssh` requires either: (a) a single command with conditional sections, or (b) two separate command files
- Claude Code commands receive `$ARGUMENTS` which includes the `--target` flag

**Decision**: Use a single `/cc-deck.build` command file with a dispatch section at the top. The command reads `--target` from `$ARGUMENTS`, then follows the appropriate section (container or ssh). Common steps (manifest reading, validation) are shared.

**Rationale**: A single command file keeps the entry point unified. The dispatch pattern is clear and matches the CLI's unified design.

**Alternatives considered**: Two separate command files (`cc-deck.build-container.md`, `cc-deck.build-ssh.md`) were considered but rejected because they break the unified design and require the user to know which command to invoke.

---

### RT-10: Verify Command for SSH Targets

**Question**: Should `cc-deck setup verify --target ssh` use Ansible or direct SSH?

**Decision**: Use direct SSH commands (reuse `internal/ssh/client.go`). The verify command runs the same checks as the container verify (cc-deck version, Claude Code, tool availability) but via SSH instead of `podman exec`.

**Rationale**: Verification should not require Ansible to be installed. It is a lightweight check, not a provisioning operation. The SSH client is already available in the codebase.

**Alternatives considered**: An Ansible `verify.yml` playbook was considered but rejected because it adds an Ansible dependency to a read-only check operation.
