# Feature Specification: Configurable Sidebar Badges

**Feature Branch**: `057-sidebar-badges`
**Created**: 2026-05-18
**Status**: Draft
**Input**: User description: "Configurable sidebar badges with JSON dot-path extraction"

## User Scenarios & Testing

### User Story 1 - Spex Pipeline Badge (Priority: P1)

A developer using cc-deck with the spex plugin wants to see at a glance which sessions have an active ship or flow pipeline. When `.specify/.spex-state` exists in a session's working directory, a badge emoji appears on line 2 of the sidebar entry, before the git branch icon. The emoji changes based on the pipeline mode (ship vs flow).

**Why this priority**: This is the primary motivating use case. The spex plugin already writes a state file, and users currently have no sidebar visibility into pipeline state.

**Independent Test**: Configure a badge rule pointing to `.specify/.spex-state`, create the file with `{"mode": "ship"}`, trigger a hook event, and verify the sidebar renders the ship emoji on line 2.

**Acceptance Scenarios**:

1. **Given** a badge rule for `.specify/.spex-state` extracting `.mode` with `ship: "🚢"`, **When** `.specify/.spex-state` exists with `{"mode": "ship"}` and a hook event fires, **Then** the sidebar shows 🚢 on line 2 before the branch icon.
2. **Given** the same badge rule with `flow: "🌊"`, **When** the file contains `{"mode": "flow"}`, **Then** the sidebar shows 🌊 instead.
3. **Given** the badge rule has a `default` emoji of "📦", **When** the file exists but the extracted value does not match any configured mapping, **Then** the sidebar shows the default emoji 📦.

---

### User Story 2 - Multiple Simultaneous Badges (Priority: P2)

A developer has multiple badge rules configured (e.g., spex pipeline and a security scan status). When both state files exist, both badges appear on line 2, ordered by their position in the configuration.

**Why this priority**: Multiple badges enable visibility into independent workflow dimensions without requiring users to choose which one to display.

**Independent Test**: Configure two badge rules, create both state files, trigger a hook event, and verify both emojis appear on line 2 in configuration order.

**Acceptance Scenarios**:

1. **Given** two badge rules (spex and security-scan) and both state files exist, **When** a hook event fires, **Then** both emojis appear on line 2 before the branch icon in configuration order.
2. **Given** two badge rules but only one state file exists, **When** a hook event fires, **Then** only the matching badge appears.

---

### User Story 3 - Silent Failure on Missing Files (Priority: P2)

A developer has badge rules configured but the corresponding state files do not exist in a particular session's working directory. No badge should appear, and no errors should be visible.

**Why this priority**: Badge rules are global, but not every project uses every tool. Silent failure prevents noise.

**Independent Test**: Configure a badge rule, ensure the state file does not exist, trigger a hook event, and verify no badge appears and no errors are logged to the user.

**Acceptance Scenarios**:

1. **Given** a badge rule for a file that does not exist, **When** a hook event fires, **Then** no badge is rendered for that rule and no error is shown.
2. **Given** a badge rule pointing to a file with invalid JSON, **When** a hook event fires, **Then** no badge is rendered for that rule and no error is shown.
3. **Given** a badge rule with a dot-path that does not match any key in the JSON, **When** a hook event fires, **Then** the default emoji is shown if configured, otherwise no badge.

---

### User Story 4 - Badge Configuration (Priority: P3)

A developer adds badge rules to their `~/.config/cc-deck/config.yaml` file. Each rule specifies a file to check, a JSON dot-path to extract a value, a mapping of values to emojis, and an optional default emoji.

**Why this priority**: Configuration is the foundation, but it is primarily a setup-time activity rather than a daily interaction.

**Independent Test**: Write a config file with badge rules, run `cc-deck config show` or equivalent, and verify the rules are parsed correctly.

**Acceptance Scenarios**:

1. **Given** a valid `badges:` section in `config.yaml`, **When** the CLI hook command starts, **Then** all badge rules are parsed and available for evaluation.
2. **Given** a badge rule with a nested dot-path (e.g., `.result.outcome`), **When** the target file contains `{"result": {"outcome": "pass"}}`, **Then** the correct emoji is resolved.
3. **Given** a badge rule with missing or invalid fields, **When** the CLI parses the config, **Then** the invalid rule is skipped silently and other rules still work.

---

### Edge Cases

- What happens when `working_dir` is not set for a session? Badge evaluation is skipped for that session.
- What happens when a badge file exists but is empty (0 bytes)? Treated as invalid JSON, badge is skipped.
- What happens when the dot-path points to a non-string value (number, boolean, array, object)? The value is converted to its string representation for matching.
- What happens when the config has no `badges:` section? No badges are evaluated, no overhead added.
- What happens when the extracted value contains characters that would break rendering? Emoji values come from the user's own config, so they are trusted.

## Requirements

### Functional Requirements

- **FR-001**: The system MUST support badge rule definitions in `~/.config/cc-deck/config.yaml` under a `badges:` key.
- **FR-002**: Each badge rule MUST specify: `name` (string), `file` (path relative to working_dir), `format` (must be "json"), `extract` (JSON dot-path), `values` (map of string to emoji), and optionally `default` (fallback emoji).
- **FR-003**: The CLI hook command MUST evaluate all badge rules against the session's `working_dir` on each hook event.
- **FR-004**: The CLI MUST resolve each badge to an emoji string (or skip if unresolvable) and include all resolved badges in the hook payload sent to the plugin.
- **FR-005**: The Rust WASM plugin MUST render resolved badges on line 2 of each session entry, before the git branch icon.
- **FR-006**: Multiple badges MUST be displayed simultaneously, ordered by their position in the configuration.
- **FR-007**: Badge evaluation MUST fail silently when the target file does not exist, contains invalid JSON, or the dot-path does not resolve to a value.
- **FR-008**: The dot-path extractor MUST support nested paths (e.g., `.result.outcome` for `{"result": {"outcome": "pass"}}`).
- **FR-009**: When the extracted value does not match any entry in the `values` map, the system MUST use the `default` emoji if configured, or skip the badge otherwise.
- **FR-010**: Badge evaluation MUST be skipped entirely when no `badges:` section exists in the config or when `working_dir` is not set for a session.

### Key Entities

- **Badge Rule**: A configuration entry defining how to detect and display a workflow badge. Contains name, file path, format, extraction path, value-to-emoji mapping, and optional default.
- **Resolved Badge**: The output of evaluating a badge rule: an emoji string ready for rendering, or nothing if the rule did not match.
- **Hook Payload**: The JSON message sent from the CLI to the plugin on each hook event, extended with a list of resolved badge emojis.

## Success Criteria

### Measurable Outcomes

- **SC-001**: Users can see workflow state badges in the sidebar within 1 second of a hook event firing.
- **SC-002**: Adding a new badge rule requires only editing the config file, with no code changes or restarts.
- **SC-003**: Badge evaluation adds less than 50ms of latency to hook processing per session, even with 5 badge rules configured.
- **SC-004**: Sessions without matching badge files show no visual artifacts or error indicators.

## Assumptions

- The `working_dir` for each session is already tracked and available in the hook processing path.
- The config file (`~/.config/cc-deck/config.yaml`) is already parsed by the CLI for other settings.
- Badge state files are small (under 10KB) and can be read synchronously without performance concerns.
- Only JSON format is supported in this version. YAML or other formats may be added later.
- The dot-path extractor handles simple nested key access only (no array indexing, no wildcards, no filters).
- Badge emojis are single emoji characters or short strings (1-2 characters) that fit in the sidebar width.
