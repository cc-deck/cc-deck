# Review Guide: 031-session-tui

## Feature Summary

Full-screen terminal UI (`cc-deck tui`) that serves as a control plane dashboard for all cc-deck environments. P1 delivers: environment list with auto-refresh, attach via suspend/resume, create wizard (local + container), lifecycle management (start/stop/delete), and help overlay. Built with bubbletea v0.25+ on the existing Go CLI.

## Artifacts to Review

| File | Focus Areas |
|------|-------------|
| `spec.md` | P1 scope boundaries, phased delivery model, acceptance scenarios |
| `plan.md` | Architecture (direct polling, no daemon), bubbletea model pattern, constitution compliance |
| `tasks.md` | Task completeness for P1, dependency graph, parallel opportunities |
| `research.md` | Framework decision (bubbletea), session data access path, attach model |
| `data-model.md` | envRow/sessionRow models, Activity enum parsing, view state machine |
| `contracts/tui-subcommand.md` | CLI interface, key bindings, plugin data contract |

## Key Review Questions

### Spec
- [ ] Are P1/P2/P3 scope boundaries clear and consistent across all sections?
- [ ] Do acceptance scenarios cover the main user journeys?
- [ ] Are edge cases adequately addressed (empty state, resize, stopped env attach)?

### Architecture
- [ ] Is direct polling (no daemon) appropriate for P1? Are polling intervals reasonable?
- [ ] Does the bubbletea suspend/resume model work for attach? (Note: uses tea.ExecProcess, not syscall.Exec)
- [ ] Is the session data path (`~/.config/zellij/plugins/cc_deck.wasm/cache/sessions.json`) correct for all platforms?

### Data Model
- [ ] Does the Activity enum deserialization handle both string and object forms?
- [ ] Is envRow sufficient for the list view columns specified in FR-001?
- [ ] Does the view state machine cover all P1 navigation flows?

### Tasks
- [ ] Are task dependencies correct? (US2/US4 depend on US1, US3 independent)
- [ ] Is the parallel execution strategy valid? (Phase 2 parallelism, post-Phase 2 story parallelism)
- [ ] Are test tasks sufficient? (Unit tests for data parsing, integration test for list rendering, manual E2E)

### Constitution Compliance
- [ ] Build via Makefile only (Principle VI)
- [ ] Documentation updates included (Principle IX): README, CLI reference, spec table
- [ ] No direct go build or cargo build commands
- [ ] XDG paths via internal/xdg (Principle XIII)

## Risks and Mitigations

| Risk | Mitigation |
|------|-----------|
| bubbletea v0.25+ API changes | Pin exact version in go.mod |
| Polling overhead with many environments | Configurable intervals, reconciliation is lightweight |
| model.go becoming large | Natural for bubbletea; split into sub-models if needed post-P1 |
| Session data file not found (no Zellij running) | Graceful fallback: show dash in sessions column |
