# Feature Specification: Build Skill Iteration Reduction

**Feature Branch**: `071-build-skill-fixes`
**Created**: 2026-06-20
**Status**: Draft
**Input**: Brainstorm 072 - eliminate build iterations by fixing 13 skill instruction gaps

## User Scenarios & Testing *(mandatory)*

### User Story 1 - First-Try OpenShell Build (Priority: P1)

A developer runs `/cc-deck.capture --all` followed by `/cc-deck.build --target openshell` on a project workspace. The build produces a working image on the first attempt without any self-correction iterations. Previously this required 8-9 iterations costing 30-90 seconds each and burning tokens on error diagnosis.

**Why this priority**: This is the core value proposition. Every build iteration wastes time and tokens. Eliminating iterations directly improves the developer experience and reduces cost.

**Independent Test**: Run a full capture-then-build cycle on a test workspace with GitHub release tools (rtk, abtop), Claude Code plugins, custom shell config (starship, fzf, lsd, zsh completions), and a post_install command. The build must succeed on the first podman build invocation.

**Acceptance Scenarios**:

1. **Given** an OpenShell target with a base image running as non-root user, **When** the Containerfile is generated, **Then** `USER root` is inserted before any `RUN` layers that require root privileges
2. **Given** a manifest with GitHub release tools, **When** the Containerfile is generated, **Then** download URLs and tarball extraction commands are verified against the GitHub API before being written
3. **Given** a snippet containing a broken download command, **When** the build skill processes the snippet, **Then** the command is corrected using the GitHub API and documented with a comment
4. **Given** the Claude Code native installer fails with exit 137, **When** the self-correction loop runs, **Then** it falls back to npm install with the correct prefix override for non-root users
5. **Given** cache directories created by root, **When** subsequent tools run as a non-root user, **Then** the parent cache directory is owned by the non-root user so sibling directories can be created
6. **Given** a fresh Claude Code installation, **When** plugin install commands are generated, **Then** the official marketplace is added before the first plugin installation
7. **Given** a tool manifest entry with a `post_install` command, **When** the Containerfile layer is generated, **Then** config directories are pre-created and chowned, the command runs as the non-root user with `|| true`, and root is restored after

---

### User Story 2 - Shell Config Dependency Resolution (Priority: P1)

A developer's captured shell config references tools (starship, lsd, fzf, zoxide, bat) in aliases, eval statements, and init scripts. The build detects these implicit dependencies and installs them even if they are not in the base image. The resulting shell session has a working prompt, correct aliases, and functional tab completion.

**Why this priority**: Runtime failures (broken prompt, missing commands, broken tab completion) are worse than build-time failures because they appear on every session, not just during build. They also lack the self-correction loop.

**Independent Test**: Capture a shell config that aliases `ls` to `lsd`, uses `eval "$(starship init zsh)"`, contains `source <(fzf --zsh)`, and has `zstyle ':completion:*'` directives loaded via a plugin manager. Build an OpenShell image. Verify that all four (lsd, starship, fzf, zsh completions) work in the resulting container.

**Acceptance Scenarios**:

1. **Given** a curated shell config with `eval "$(starship init zsh)"`, **When** shell config dependencies are scanned, **Then** starship is flagged as a required tool and installed from GitHub releases if missing from the base image
2. **Given** a curated shell config with `source <(fzf --zsh)`, **When** the target image has an apt-packaged fzf older than 0.48, **Then** fzf is installed from GitHub releases instead of the package manager
3. **Given** a curated shell config that originally used Antidote/oh-my-zsh for compinit, **When** the plugin manager is stripped during capture, **Then** `autoload -Uz compinit && compinit -C` is added as a preamble if `compdef` or `zstyle ':completion:*'` lines are present
4. **Given** starship init in the shell config, **When** `cc-deck ws new` runs exec commands with TERM=dumb, **Then** starship does not emit errors because its init is guarded with `[[ "$TERM" != "dumb" ]]`

---

### User Story 3 - Capture-Time Verification (Priority: P2)

During `/cc-deck.capture`, GitHub release asset patterns are verified against the API and stored in the manifest. Post-install commands are dry-run validated. This catches mismatches and interactive prompt issues before the build phase, providing early feedback during the interactive wizard.

**Why this priority**: Early detection prevents wasted build cycles. However, the build-time verification (Story 1) catches these issues too, so capture-time verification is a convenience improvement, not a correctness requirement.

**Independent Test**: Run `/cc-deck.capture` with a tool that has an incorrect asset_pattern in a prior manifest. Verify the capture wizard detects the mismatch and writes the correct pattern. Run with a tool that has an interactive post_install command and verify it warns about the interactive prompt.

**Acceptance Scenarios**:

1. **Given** a tool with `install: github-release` and a repo field, **When** the capture wizard processes it, **Then** the actual GitHub release asset names are queried and the correct `asset_pattern` is stored in the manifest
2. **Given** a tool with a `post_install` command that prompts for input, **When** the capture wizard runs dry-run validation, **Then** a warning is displayed about the interactive prompt
3. **Given** `build refresh` is run, **When** snippet download commands are regenerated, **Then** URLs are verified against GitHub APIs

---

### User Story 4 - OpenShell Base Image Documentation (Priority: P3)

The build skill's Key Rules section documents that the OpenShell base image is Ubuntu (not Fedora), runs as `sandbox` (not root), and lacks tools that the cc-deck base image includes. This prevents the LLM from making incorrect assumptions about the build environment.

**Why this priority**: Documentation clarity. The other stories fix the actual failures; this story prevents future regressions by making the distinctions explicit.

**Independent Test**: Read the Key Rules section and verify it explicitly states the OS, default user, and tool availability differences between cc-deck and OpenShell base images.

**Acceptance Scenarios**:

1. **Given** the build skill's Key Rules section, **When** a developer reads it, **Then** it clearly states that the OpenShell base image is Ubuntu, runs as user `sandbox`, and does not include lsd, starship, zsh, bat, or ripgrep
2. **Given** the Key Rules section, **When** compared to the cc-deck base image notes, **Then** the differences are explicit and cannot be confused

---

### Edge Cases

- What happens when a GitHub API rate limit is hit during asset verification? The build should fall back to the manifest's `asset_pattern` with a warning, not fail the entire build.
- What happens when a tool's GitHub release has no matching asset for the target architecture? The build should emit a clear error identifying the tool and available architectures.
- What happens when a curated shell config references a tool that has no GitHub release and is not in the package manager? The build should warn but not fail, and the shell config line should be commented out.
- What happens when `post_install` dry-run produces a non-zero exit? The capture wizard should warn but still include the tool, noting that post_install may need manual intervention.

## Requirements *(mandatory)*

### Functional Requirements

**Build Skill (cc-deck.build.md)**

- **FR-001**: Build skill MUST insert `USER root` after the header snippet before any `RUN` layers when the base image runs as a non-root user (OpenShell target)
- **FR-002**: Build skill MUST query the GitHub API for actual release asset names before generating download commands for `install: github-release` tools
- **FR-003**: Build skill MUST probe tarball structure (flat binary vs nested directory) before generating extraction commands
- **FR-004**: Build skill MUST allow snippet modification when download commands produce 404 or extraction errors, verifying against the GitHub API and documenting changes with comments
- **FR-005**: Build skill MUST provide a Claude Code npm fallback path when the native installer fails with exit 137 (OOM), including npm prefix override for non-root users
- **FR-006**: Build skill MUST set cache directory ownership on the parent directory (`/sandbox/.cache`) rather than individual subdirectories
- **FR-007**: Build skill MUST add `claude plugins marketplace add anthropics/claude-plugins-official` before the first plugin installation command
- **FR-008**: Build skill MUST scan the curated shell config for commands used in aliases and eval statements, cross-referencing against the base image probe to identify missing tools
- **FR-009**: Build skill MUST provide the exact jq merge command for settings.json: `jq -s '.[0] as $orig | $orig * .[1] | .hooks = $orig.hooks'`
- **FR-010**: Build skill MUST follow the post_install sandboxing protocol: pre-create config directories as root, run the command as the non-root user with `|| true`, restore root after
- **FR-011**: Build skill MUST document OpenShell base image characteristics (Ubuntu, sandbox user, missing tools) in the Key Rules section

**Capture Skill (cc-deck.capture.md)**

- **FR-012**: Capture skill MUST scan curated shell config for implicit tool dependencies (commands in aliases, eval, source statements)
- **FR-013**: Capture skill MUST detect `source <(fzf --zsh)` and flag fzf for GitHub release install rather than package manager install
- **FR-014**: Capture skill MUST add `autoload -Uz compinit && compinit -C` preamble when stripping plugin managers if the curated config contains `compdef` or `zstyle ':completion:*'` directives
- **FR-015**: Capture skill MUST verify GitHub release asset patterns against the API at capture time and store verified patterns in the manifest
- **FR-016**: Capture skill MUST run post_install commands with `--dry-run` or `--help` flags to detect interactive prompts and missing directory requirements

**Template Changes**

- **FR-017**: The `05-shell-finalize.tmpl` template MUST guard starship init with `[[ "$TERM" != "dumb" ]]` to prevent errors during TERM=dumb exec contexts
- **FR-018**: The `03-mandatory-stack.tmpl` template MUST set cache directory ownership on the parent directory
- **FR-019**: The `03-mandatory-stack.tmpl` template MUST add the official Claude plugins marketplace before plugin installations

**Build Refresh**

- **FR-020**: `build refresh` MUST verify download URLs in regenerated snippets against GitHub APIs

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A fresh OpenShell build (`/cc-deck.capture --all` then `/cc-deck.build --target openshell`) succeeds on the first podman build invocation without self-correction iterations for any of the 13 documented failure categories
- **SC-002**: Shell sessions in the built image have a working starship prompt, functional `lsd` alias, `fzf` key bindings, and zsh tab completion
- **SC-003**: No starship errors appear in stderr when running exec commands with TERM=dumb
- **SC-004**: Capture-time asset verification detects at least one pattern mismatch when given a manifest with an incorrect asset_pattern (regression test)
- **SC-005**: Build iteration count drops from 8-9 to 0-1 for standard OpenShell builds (measured by counting self-correction loop invocations)

## Assumptions

- The GitHub API is accessible during both capture and build phases (network connectivity required)
- GitHub API rate limits are sufficient for the number of tools in a typical manifest (usually under 20 tools, well within unauthenticated rate limits)
- The OpenShell base image continues to use Ubuntu and default to the `sandbox` user
- The cc-deck base image continues to use Fedora and run as root
- The `build-learnings.md` mechanism continues to work as a secondary safety net for failures not covered by these changes
- Tools referenced in shell configs have GitHub releases available (starship, lsd, fzf, bat, zoxide all do)
- The `--dry-run` or `--help` flag is a reliable indicator for detecting interactive prompts in post_install commands (may not work for all tools)
