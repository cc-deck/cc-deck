# Review Guide: 026 - Project-Local Environment Configuration

## For Reviewers

This feature moves environment definitions, image build artifacts, and runtime state into a project-scoped `.cc-deck/` directory at the git root. It adds implicit name resolution via git-boundary walk and a global project registry.

### Quick Review Checklist

- [ ] **spec.md**: 30 FRs, 8 SCs, 7 user stories, 7 edge cases. Check FR-025 through FR-030 (added during spec review to close gaps).
- [ ] **plan.md**: Technical context, constitution check (all pass), 5-phase implementation approach, source code structure.
- [ ] **research.md**: 6 research decisions with rationale and alternatives. Key: R1 (compose paths), R3 (git walk), R5 (status store).
- [ ] **data-model.md**: 4 entities (2 new, 2 modified). Check ProjectStatusFile state transitions and precedence chain.
- [ ] **contracts/project-discovery.md**: New `project` package, `ProjectStatusStore`, registry extensions, `env prune` CLI contract.
- [ ] **quickstart.md**: 12-step implementation order across 5 phases. Verify dependency ordering.

### Key Design Decisions to Validate

1. **Single env per project** (D1 in brainstorm): Is this sufficient? The variant mechanism (FR-010) handles multi-instance needs.
2. **`git rev-parse --show-toplevel`** for git root detection: Proven pattern in codebase. Alternative (manual walk) was rejected.
3. **Project-local `status.yaml` separate from global state**: Avoids polluting global state with per-project data. Trade-off: two state locations to reconcile.
4. **`.cc-deck/.gitignore` exception to Principle XIV**: Documented in FR-024. The only dotfile inside `.cc-deck/`.
5. **`env create` auto-scaffolds when no definition exists** (FR-025): Scaffolds `.cc-deck/environment.yaml` from CLI flags in a git repo, then provisions. No separate init command.

### Areas Requiring Careful Review

- **State split**: Project-local `status.yaml` vs global `state.yaml`. Ensure `env list` correctly merges both sources and no dual-write bugs occur.
- **Compose artifact relocation**: Moving from `.cc-deck/` to `.cc-deck/run/` changes proxy volume paths in `generate.go`. Verify paths are consistent.
- **Delete behavior (FR-027)**: Must remove `status.yaml` and `run/` but preserve `environment.yaml` and `image/`. This is a partial directory cleanup, not a full `rm -rf .cc-deck/`.

### Spec Review Summary

Initial review: 4.0/5. Three blocking issues (B1-B3) and six non-blocking issues (N1-N6) identified and resolved:
- B1: Added FR-025 (create behavior without definition)
- B2: Added FR-026 (project-local vs global precedence)
- B3: Added FR-027 (delete behavior for project-local)
- N1-N6: Added FR-028 (env vars), FR-029 (version field), FR-030 (gitignore self-healing), error scenarios for --branch, recreate scope, container name collision edge case

### Plan Review Summary

Score: 4.5/5. Full 30/30 FR coverage. No blocking issues. Two medium-risk areas identified (state split coordination, compose path changes) with mitigations in place. Phasing strategy is sound: foundation first, breaking changes last.
