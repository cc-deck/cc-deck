# Data Model: Config Validation

## Entities

### Finding

Represents a single validation issue found in the config file.

| Field | Type | Description |
|-------|------|-------------|
| Severity | Severity (string const) | "error" or "warning" |
| Category | Category (string const) | "badges", "profiles", "voice", or "structure" |
| Message | string | Human-readable description of the issue |
| Suggestion | string | Actionable fix recommendation |

### Severity

| Value | Meaning | Exit code impact |
|-------|---------|-----------------|
| error | Will break something (layout, deploy, functionality) | Causes exit code 1 |
| warning | May cause issues in some environments | Does not affect exit code |

### Category

| Value | Checks covered |
|-------|---------------|
| badges | Icon width (emoji, Wide, Ambiguous), color prefix syntax, required fields, format value |
| profiles | Required fields per backend type, default_profile reference |
| voice | Threshold range, silence/pre_roll/hangover positivity, extreme values |
| structure | YAML parse errors (handled before validation, included for completeness) |

## Relationships

```
Config (1) --validates--> (0..*) Finding
Config (1) --contains--> (0..*) BadgeRule --icon-check--> (0..*) Finding
Config (1) --contains--> (0..*) Profile --field-check--> (0..*) Finding
Config (1) --contains--> (1) VoiceDefaults --range-check--> (0..*) Finding
```

## State Transitions

None. Findings are computed on demand (pure function), not persisted.

## Validation Flow

1. Parse config YAML (existing `Load()` function handles this)
2. Call `Config.Validate()` which runs checks in order:
   a. Badge rules: structure, format, icon width for each value
   b. Profiles: delegate to `Profile.Validate()`, check default_profile reference
   c. Voice: range checks on all numeric parameters
3. Return accumulated findings slice
4. Caller decides what to do (print full report or one-line summary)
