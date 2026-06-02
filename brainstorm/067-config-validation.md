# Brainstorm: Config Validation

**Date:** 2026-06-02
**Status:** active

## Problem Framing

Users configure cc-deck through `~/.config/cc-deck/config.yaml`, including badge icons, voice parameters, and K8s profiles. Misconfigurations are silent until they break something visually (badge icons breaking sidebar layout) or functionally (invalid profile fields causing deploy failures). There is no way to validate the config before something goes wrong.

The immediate trigger: a user chose `☰` (U+2630, East Asian Width W) as a badge icon. The `unicode_width` crate reported it as width 1, but the terminal rendered it as 2 columns, breaking sidebar alignment. The user had no way to know the icon was problematic.

## Approaches Considered

### A: Validation in the config package (chosen)

Add `Config.Validate() []Finding` in `internal/config/`. Each finding carries severity (Error/Warning), message, and fix suggestion. The `cc-deck config check` command calls this and formats output. Other commands call it at load time and print a one-line summary to stderr.

- Pros: Validation logic lives next to data structures. Easy to unit test. Load-time integration is trivial. Follows existing `Profile.Validate()` pattern.
- Cons: Config package gains unicode width analysis logic (acceptable, can use a lookup table).

### B: Separate validator package

Create `internal/configcheck/` with validation logic that accepts `*config.Config`.

- Pros: Config package stays dependency-free. No circular deps.
- Cons: One more package. More wiring in the CLI.

### C: CLI-only validation

All validation in the command handler. No reusable function.

- Pros: Simplest initial implementation.
- Cons: Untestable without CLI. Load-time integration is awkward. Grows messy.

## Decision

Approach A. The config package already has `Profile.Validate()`, so `Config.Validate()` follows the established pattern.

## Key Requirements

### CLI Command

- `cc-deck config check` runs full validation and prints findings.
- Output format: one line per finding with severity, location, message, and fix suggestion.
- Exit code 0 if no errors, 1 if errors found (warnings alone are exit 0).

### Load-time Integration

- When any cc-deck command loads the config, run validation.
- Print a one-line summary to stderr if findings exist: `config: 2 warnings found, run cc-deck config check for details`.
- Errors and warnings both trigger the summary. No suppression mechanism in v1.

### Validation Checks

**Badges:**
- Flag emoji icons as errors (always 2 columns wide, breaks sidebar alignment).
- Flag East Asian Width W characters as errors (always 2 columns, `unicode_width` may undercount).
- Warn on East Asian Width Ambiguous characters (may render as 1 or 2 columns depending on terminal).
- Each icon finding includes a suggested single-width replacement.
- Check color prefix syntax is valid `#RRGGBB:icon` when present.
- Check required fields: `name`, `file`, `extract` must be non-empty.
- Check `format` is a supported value (currently `json`).

**Profiles:**
- Run existing `Profile.Validate()` for required fields per backend type.
- Verify `default_profile` references an existing profile name (if set).

**Voice:**
- `threshold` must be in 0-100 range.
- `silence`, `pre_roll`, `hangover` must be positive.
- Warn on extreme values (e.g., `silence > 10`, `hangover > 5`).

**Structure:**
- Config file parses as valid YAML (friendlier message than raw YAML error).

### Severity Levels

- **Error**: Will break something (wide icons, missing required profile fields, invalid YAML).
- **Warning**: May cause issues (Ambiguous-width icons, extreme voice parameter values).

### Finding Format

Each finding is a struct with: severity, category (badges/profiles/voice/structure), message, and fix suggestion.

### Out of Scope

- Interactive config editing or wizard.
- Auto-fix mode (suggestions only, user edits manually).
- Validating K8s resource quantities (`storage_size` format).
- Badge icon preview or rendering test.

## Open Questions

- Should load-time warnings be suppressible via a `--quiet` flag or config key? Deferred to v2 if users find them noisy.
- Should the command suggest safe icon alternatives from a curated list, or just describe the constraint? Start with constraint description and a few known-good alternatives per semantic category.
