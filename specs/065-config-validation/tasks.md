# Tasks: Config Validation

**Input**: Design documents from `specs/065-config-validation/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US3)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: Create validation types and foundational structures

- [ ] T001 [P] Define Finding, Severity, and Category types in cc-deck/internal/config/validate.go
- [ ] T002 [P] Create cc-deck/internal/config/validate_test.go with test helpers and table-driven test scaffold

---

## Phase 2: Foundational (Badge Icon Width Logic)

**Purpose**: Core Unicode width analysis that badge validation depends on

- [ ] T003 Implement icon width classification functions (isEmoji, isEastAsianWide, isEastAsianAmbiguous) using Go unicode stdlib in cc-deck/internal/config/validate.go
- [ ] T004 Implement parseBadgeColor to extract icon from `#RRGGBB:icon` format in cc-deck/internal/config/validate.go
- [ ] T005 [P] Add unit tests for icon width classification (emoji, Wide, Ambiguous, Narrow codepoints) in cc-deck/internal/config/validate_test.go
- [ ] T006 [P] Add unit tests for parseBadgeColor with valid/invalid color prefixes in cc-deck/internal/config/validate_test.go

**Checkpoint**: Icon width analysis is tested and ready for badge validation

---

## Phase 3: User Story 1 + 3 - Explicit Config Validation + Badge Icon Width (Priority: P1) MVP

**Goal**: `Config.Validate()` returns findings for badge icon width issues, and `cc-deck config check` prints them

**Independent Test**: Create a config with wide badge icons, run `cc-deck config check`, verify error output and exit code

### Implementation

- [ ] T007 [US1] Implement validateBadges() checking icon width (emoji=error, Wide=error, Ambiguous=warning) with suggested replacements in cc-deck/internal/config/validate.go
- [ ] T008 [US1] Implement Config.Validate() calling validateBadges and returning []Finding in cc-deck/internal/config/validate.go
- [ ] T009 [US1] Add unit tests for validateBadges with emoji, Wide, Ambiguous, Narrow, and color-prefixed badge values in cc-deck/internal/config/validate_test.go
- [ ] T010 [US1] Create cc-deck config check command in cc-deck/internal/cmd/config_check.go with formatted output and exit code logic
- [ ] T011 [US1] Register NewCheckCmd in cc-deck/internal/cmd/config.go

**Checkpoint**: `cc-deck config check` reports badge icon width issues with fix suggestions

---

## Phase 4: User Story 4 - Badge Rule Structure Validation (Priority: P2)

**Goal**: Validate badge rule required fields and format value

**Independent Test**: Create badge rules with missing name/file/extract or invalid format, verify errors

### Implementation

- [ ] T012 [P] [US4] Add badge structure validation (name, file, extract required; format must be "json") to validateBadges() in cc-deck/internal/config/validate.go
- [ ] T013 [P] [US4] Add color prefix syntax validation (hex must be 6 valid hex chars) to validateBadges() in cc-deck/internal/config/validate.go
- [ ] T014 [US4] Add unit tests for badge structure and color prefix validation in cc-deck/internal/config/validate_test.go

**Checkpoint**: Badge rules with structural issues are caught

---

## Phase 5: User Story 5 - Profile Validation (Priority: P2)

**Goal**: Validate profile required fields and default_profile reference

**Independent Test**: Create profiles with missing fields or dangling default_profile, verify errors

### Implementation

- [ ] T015 [US5] Implement validateProfiles() wrapping Profile.Validate() errors as Findings and checking default_profile in cc-deck/internal/config/validate.go
- [ ] T016 [US5] Add validateProfiles call to Config.Validate() in cc-deck/internal/config/validate.go
- [ ] T017 [US5] Add unit tests for profile validation (missing api_key_secret, missing project, dangling default_profile) in cc-deck/internal/config/validate_test.go

**Checkpoint**: Profile misconfigurations are caught

---

## Phase 6: User Story 6 - Voice Parameter Validation (Priority: P3)

**Goal**: Validate voice parameter ranges and warn on extreme values

**Independent Test**: Set voice threshold to 150, silence to -1, verify errors and warnings

### Implementation

- [ ] T018 [US6] Implement validateVoice() checking threshold range, positivity, and extreme values in cc-deck/internal/config/validate.go
- [ ] T019 [US6] Add validateVoice call to Config.Validate() in cc-deck/internal/config/validate.go
- [ ] T020 [US6] Add unit tests for voice validation (out-of-range, negative, extreme, valid values) in cc-deck/internal/config/validate_test.go

**Checkpoint**: Voice parameter issues are caught

---

## Phase 7: User Story 2 - Load-Time Warning Summary (Priority: P2)

**Goal**: Any cc-deck command that loads config prints a one-line summary to stderr when issues exist

**Independent Test**: Create config with issues, run `cc-deck ws list`, verify one-line stderr summary

### Implementation

- [ ] T021 [US2] Implement ValidateAndWarn() helper that calls Validate() and prints one-line summary to stderr in cc-deck/internal/config/validate.go
- [ ] T022 [US2] Integrate ValidateAndWarn() into config loading path used by ws, voice, and other config-dependent commands in cc-deck/internal/cmd/ws.go
- [ ] T023 [US2] Add unit test for ValidateAndWarn output formatting (error+warning counts, no output when clean) in cc-deck/internal/config/validate_test.go

**Checkpoint**: Users are passively notified of config issues during normal usage

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Documentation and cleanup

- [ ] T024 [P] Update CLI reference with config check command documentation in docs/modules/reference/pages/cli.adoc
- [ ] T025 [P] Update configuration reference with validation behavior in docs/modules/reference/pages/configuration.adoc
- [ ] T026 Run make test and make lint to verify all tests pass and code is clean

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies
- **Foundational (Phase 2)**: Depends on Phase 1 (types defined)
- **US1+US3 (Phase 3)**: Depends on Phase 2 (icon width logic)
- **US4 (Phase 4)**: Depends on Phase 3 (validateBadges exists)
- **US5 (Phase 5)**: Depends on Phase 3 (Config.Validate exists)
- **US6 (Phase 6)**: Depends on Phase 3 (Config.Validate exists)
- **US2 (Phase 7)**: Depends on Phase 3 (Config.Validate exists, needs findings to summarize)
- **Polish (Phase 8)**: Depends on all implementation phases

### Parallel Opportunities

- T001 and T002 can run in parallel (types vs test scaffold)
- T005 and T006 can run in parallel (different test functions)
- T012 and T013 can run in parallel (different validation aspects)
- T024 and T025 can run in parallel (different doc files)
- Phases 4, 5, and 6 can run in parallel after Phase 3 (different validation categories, same file but different functions)

---

## Implementation Strategy

### MVP First (Phase 3 Only)

1. Complete Phase 1: Types
2. Complete Phase 2: Icon width logic
3. Complete Phase 3: Badge icon validation + config check command
4. **STOP and VALIDATE**: Test with real config file
5. Ship if ready

### Full Delivery

1. Setup + Foundational (Phases 1-2)
2. Badge icon validation + CLI command (Phase 3, MVP)
3. Badge structure + Profile + Voice validation (Phases 4-6, can parallelize)
4. Load-time integration (Phase 7)
5. Documentation (Phase 8)

---

## Notes

- All new code goes in cc-deck/internal/config/validate.go and validate_test.go
- CLI command in cc-deck/internal/cmd/config_check.go
- No new dependencies needed (Go unicode stdlib is sufficient)
- Constitution requires docs update in same branch
