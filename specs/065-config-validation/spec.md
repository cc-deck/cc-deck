# Feature Specification: Config Validation

**Feature Branch**: `065-config-validation`
**Created**: 2026-06-02
**Status**: Draft
**Input**: User description: "Add cc-deck config check command and load-time validation for config.yaml"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Explicit Config Validation (Priority: P1)

A user edits their `~/.config/cc-deck/config.yaml` to add badge icons, configure voice parameters, or set up K8s profiles. After editing, they run `cc-deck config check` to validate their configuration before restarting any workspace or voice session.

**Why this priority**: This is the primary user-facing feature. Users need a way to proactively validate their config and get actionable feedback on problems.

**Independent Test**: Can be fully tested by creating a config file with known issues (wide badge icons, invalid profile fields, out-of-range voice thresholds) and running `cc-deck config check`. Delivers immediate value by surfacing problems before they cause runtime failures.

**Acceptance Scenarios**:

1. **Given** a config file with a Wide badge icon (e.g., U+2630), **When** the user runs `cc-deck config check`, **Then** an error is reported with the icon, its codepoint, the reason it is problematic, and a suggested replacement.
2. **Given** a config file with no issues, **When** the user runs `cc-deck config check`, **Then** a success message is printed and the exit code is 0.
3. **Given** a config file with warnings but no errors, **When** the user runs `cc-deck config check`, **Then** warnings are printed but the exit code is 0.
4. **Given** a config file with at least one error, **When** the user runs `cc-deck config check`, **Then** all findings are printed and the exit code is 1.

---

### User Story 2 - Load-Time Warning Summary (Priority: P2)

A user has a misconfigured config file (e.g., a badge icon that renders as 2 columns). When they run any cc-deck command (e.g., `cc-deck ws voice`), a one-line warning is printed to stderr pointing them to `cc-deck config check` for details.

**Why this priority**: This provides passive discovery of config issues. Users who never run `cc-deck config check` still get notified that something may be wrong.

**Independent Test**: Can be tested by creating a config with a known issue, running any cc-deck command, and verifying the one-line summary appears on stderr without disrupting the command's normal output.

**Acceptance Scenarios**:

1. **Given** a config file with 2 warnings and 1 error, **When** the user runs any cc-deck command, **Then** stderr shows `config: 3 issues found (1 error, 2 warnings), run cc-deck config check for details`.
2. **Given** a config file with no issues, **When** the user runs any cc-deck command, **Then** no validation message appears on stderr.
3. **Given** a config file that does not exist, **When** the user runs any cc-deck command, **Then** no validation message appears (missing config is not an error).

---

### User Story 3 - Badge Icon Width Validation (Priority: P1)

A user adds badge icons to their config for sidebar display. The validator checks each icon for display width problems: emoji icons (always 2 columns) and East Asian Width W characters are reported as errors, East Asian Width Ambiguous characters are reported as warnings.

**Why this priority**: This addresses the immediate trigger for the feature. Badge icon width mismatches break sidebar layout alignment, and users have no way to know which icons are safe.

**Independent Test**: Can be tested by creating badge rules with various icon types (emoji, Wide, Ambiguous, Narrow) and verifying correct classification and suggested replacements.

**Acceptance Scenarios**:

1. **Given** a badge value containing an emoji (e.g., U+1F6A2), **When** validation runs, **Then** an error is reported explaining the emoji is 2 columns wide and suggesting a single-width alternative.
2. **Given** a badge value containing an East Asian Width W character (e.g., U+2630), **When** validation runs, **Then** an error is reported with the same structure.
3. **Given** a badge value containing an East Asian Width Ambiguous character (e.g., U+25B6), **When** validation runs, **Then** a warning is reported noting it may render as 2 columns in some terminals.
4. **Given** a badge value containing only Narrow characters (e.g., U+22EE), **When** validation runs, **Then** no finding is reported for that icon.
5. **Given** a badge value with a color prefix (e.g., `#FFB43C:icon`), **When** validation runs, **Then** only the icon portion after the colon is checked for width.

---

### User Story 4 - Badge Rule Structure Validation (Priority: P2)

A user defines badge rules with missing required fields or invalid format values. The validator catches these structural issues.

**Acceptance Scenarios**:

1. **Given** a badge rule with an empty `name` field, **When** validation runs, **Then** an error is reported.
2. **Given** a badge rule with an empty `file` field, **When** validation runs, **Then** an error is reported.
3. **Given** a badge rule with an empty `extract` field, **When** validation runs, **Then** an error is reported.
4. **Given** a badge rule with `format: xml`, **When** validation runs, **Then** an error is reported listing supported formats.
5. **Given** a badge value with an invalid color prefix (e.g., `#FFB4:icon`), **When** validation runs, **Then** an error is reported about the malformed color code.

---

### User Story 5 - Profile Validation (Priority: P2)

A user configures profiles for Anthropic or Vertex backends. The validator checks required fields per backend type and verifies that `default_profile` references an existing profile.

**Acceptance Scenarios**:

1. **Given** an Anthropic profile without `api_key_secret`, **When** validation runs, **Then** an error is reported.
2. **Given** a Vertex profile without `project`, **When** validation runs, **Then** an error is reported.
3. **Given** `default_profile` set to a name that does not exist in `profiles`, **When** validation runs, **Then** an error is reported.
4. **Given** `default_profile` is empty and no profiles are defined, **When** validation runs, **Then** no finding is reported.

---

### User Story 6 - Voice Parameter Validation (Priority: P3)

A user configures voice relay defaults with out-of-range or extreme values. The validator catches invalid and unreasonable values.

**Acceptance Scenarios**:

1. **Given** `voice.threshold` set to 150, **When** validation runs, **Then** an error is reported (must be 0-100).
2. **Given** `voice.silence` set to -1, **When** validation runs, **Then** an error is reported (must be positive).
3. **Given** `voice.silence` set to 15, **When** validation runs, **Then** a warning is reported (extreme value).
4. **Given** `voice.hangover` set to 0.3, **When** validation runs, **Then** no finding is reported.

---

### Edge Cases

- What happens when the config file has valid YAML syntax but unknown top-level keys? No finding is reported (forward compatibility).
- What happens when a badge value is an empty string? No width check needed, skip it.
- What happens when the config file is completely empty? No findings (empty config is valid).
- What happens when badge values contain ANSI escape sequences? The icon check should parse past color prefixes (`#RRGGBB:`) but not handle arbitrary ANSI codes in badge values.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide a `Validate()` method on the `Config` struct that returns a slice of findings.
- **FR-002**: Each finding MUST include: severity (Error or Warning), category (badges, profiles, voice, structure), a human-readable message, and a fix suggestion.
- **FR-003**: The `cc-deck config check` command MUST print all findings grouped by category, one line per finding, with severity indicator.
- **FR-004**: The `cc-deck config check` command MUST exit with code 0 when no errors are found (warnings only or clean), and code 1 when any error is found.
- **FR-005**: When any cc-deck command loads the config, validation MUST run and print a one-line summary to stderr if findings exist.
- **FR-006**: Badge icon validation MUST classify emoji as errors (always 2-column width).
- **FR-007**: Badge icon validation MUST classify East Asian Width W characters as errors.
- **FR-008**: Badge icon validation MUST classify East Asian Width Ambiguous characters as warnings.
- **FR-009**: Badge icon findings MUST include a suggested single-width replacement character.
- **FR-010**: Badge rule validation MUST check that `name`, `file`, and `extract` fields are non-empty.
- **FR-011**: Badge rule validation MUST check that `format` is a supported value (currently `json`).
- **FR-012**: Badge color prefix validation MUST check `#RRGGBB:` syntax when present.
- **FR-013**: Profile validation MUST check required fields per backend type using existing `Profile.Validate()`.
- **FR-014**: Profile validation MUST verify `default_profile` references an existing profile name.
- **FR-015**: Voice validation MUST check `threshold` is in the 0-100 range.
- **FR-016**: Voice validation MUST check `silence`, `pre_roll`, and `hangover` are positive when set.
- **FR-017**: Voice validation MUST warn on extreme values (`silence > 10`, `hangover > 5`, `pre_roll > 2`).

### Key Entities

- **Finding**: Represents a single validation finding with severity, category, message, and fix suggestion.
- **Severity**: Error (will break something) or Warning (may cause issues).
- **Category**: badges, profiles, voice, structure.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Running `cc-deck config check` on a config with known issues reports all issues within 100ms.
- **SC-002**: All badge icon width checks correctly classify emoji, Wide, Ambiguous, and Narrow characters per Unicode East Asian Width property.
- **SC-003**: Load-time validation adds less than 5ms overhead to any cc-deck command.
- **SC-004**: Unit test coverage for all validation check categories (badges, profiles, voice, structure) with both passing and failing cases.
- **SC-005**: Users can identify and fix config issues without reading source code, using only the fix suggestions in the output.

## Assumptions

- The `unicode_width` crate (used by the Rust plugin) may disagree with the Go validator on character widths. The Go validator uses Unicode East Asian Width property directly, which is authoritative.
- The config file location follows XDG conventions (`~/.config/cc-deck/config.yaml`).
- Load-time validation runs on the config that was successfully parsed. If YAML parsing fails, the existing error handling surfaces the parse error before validation runs.
- Badge values are short strings (typically 1-3 characters). Performance of unicode property lookups is not a concern.
- The set of supported badge formats may grow beyond `json` in the future. The validator checks against the current supported list.
