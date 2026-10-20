---
runbook_version: 1
feature: 091-harness-scoped-profile-names
status: passed
mode: light
steps_total: 8
step_results: passed,passed,passed,passed,passed,passed,passed,passed
drafted_at: 0cc5c00
drafted_fingerprint: e193e6f8d0f3b753c03c81f964c2d6755a0dac48ab489c452ea787bb5905590a
tested_at: 0cc5c00
tested_fingerprint: e5d11cc632cc842570103e67800999d5169e02fcebae0a2ac5fa35dee89ae97f
updated: 2026-10-20
---

# Try it: Harness-scoped profile names

<!-- runbook-status -->
> **Status:** passed · 8 of 8 steps (8 passed) · Mode: light · Tested at 0cc5c00 · 2026-10-20
<!-- /runbook-status -->



















## What you're testing

This journey adds a Codex profile with the same short name as an existing Claude profile. It checks that both qualified selectors work, the bare name becomes ambiguous, the default selects one exact pair, and cleanup preserves the original profile.

## Test environment

No fixtures are needed. Build the CLI from this worktree first; each command then invokes `./cc-deck/cc-deck` so it exercises the code being finished. Run the commands from this feature worktree's repository root. While the feature is in a worktree, a shell opened at the main checkout must first enter `.claude/worktrees/091-harness-scoped-profile-names`; after merge, run from the checkout root. The journey uses the local cc-deck configuration and requires an existing `claude/private` profile. Set `OPENAI_API_KEY` in the environment before adding the Codex profile if you want to launch it with real credentials; the commands below only inspect profile configuration and wrappers.

## Steps

### Step 1 of 8: Build the feature CLI (US1)

**Why this step:** This ensures the commands below run the schema implementation from this worktree.

**Needs:** The repository build prerequisites.

**Run:**

```sh
make build-cli
```

**What you should see:** The CLI build completes and writes `cc-deck/cc-deck`.

**If you see something else:** Check the build output for the first failing prerequisite or compilation error.

**Result:**

Passed (agent). The first attempt from the main checkout used its older CLI and showed incorrect rows. Retried from this feature worktree with the built CLI; it listed `claude-private` (harness `claude`, default) and `codex-private` (harness `codex`). Exit code 0.

Passed (agent). `make build-cli` exited 0; output showed `go build -o cc-deck ./cmd/cc-deck` creating the worktree binary.

### Step 2 of 8: Add the same name under Codex (US1)

**Why this step:** This creates the second identity that the feature enables.

**Needs:** Step 1 and an existing `claude/private` profile.

**Run:**

```sh
./cc-deck/cc-deck config profile add private --harness codex --api-key-env OPENAI_API_KEY
```

**What you should see:** The command adds `codex/private` and reports that the profile was saved.

**If you see something else:** An existing-pair error means `codex/private` is already configured; inspect it with `./cc-deck/cc-deck config profile show codex/private` before continuing.

**Result:**

Passed (agent). Output: `Profile "private" added.`

### Step 3 of 8: Confirm both qualified profiles (US1)

**Why this step:** The list should show the two distinct profile identities.

**Needs:** Step 2.

**Run:**

```sh
./cc-deck/cc-deck config profile list
```

**What you should see:** `claude-private` and `codex-private` rows, with `claude-private` marked as default. The HARNESS column identifies `claude` and `codex`.

**If you see something else:** If one row is missing, validate the config and inspect the corresponding qualified selector.

**Result:**


### Step 4 of 8: Inspect each qualified profile (US2)

**Why this step:** Qualified selectors must pick exactly one harness/name pair.

**Needs:** Step 2.

**Run:**

```sh
./cc-deck/cc-deck config profile show claude/private && ./cc-deck/cc-deck config profile show codex/private
```

**What you should see:** Each command shows its own harness and wrapper command, `claude-private` or `codex-private`.

**If you see something else:** Check that the harness is correct in each output and that neither selector resolves to the other profile.

**Result:**

Passed (user). `claude/private` showed harness `claude` and wrapper `claude-private`; `codex/private` showed harness `codex` and wrapper `codex-private`.


### Step 5 of 8: Confirm the bare name is rejected (US2)

**Why this step:** An ambiguous short name must never silently choose an account.

**Needs:** Step 2.

**Run:**

```sh
./cc-deck/cc-deck config profile show private
```

**What you should see:** An ambiguity error listing `claude/private` and `codex/private` as choices.

**If you see something else:** If a profile is shown, stop and check whether both profiles are present in the config.

**Result:**

Passed (agent). Exit code 1 with the expected error: `profile "private" is ambiguous; use claude/private or codex/private`.


### Step 6 of 8: Select the Codex default (US2)

**Why this step:** This confirms the saved default retains the harness as part of its identity.

**Needs:** Step 2.

**Run:**

```sh
./cc-deck/cc-deck config profile use codex/private && ./cc-deck/cc-deck config profile list
```

**What you should see:** The command succeeds and marks only `codex/private` as the default.

**If you see something else:** Inspect `default_profile` with `./cc-deck/cc-deck config profile list -o yaml` and check for the qualified value `codex/private`.

**Result:**

Passed (agent). The CLI confirmed `codex/private` as the default; the list marked only `codex-private` with `*`.


### Step 7 of 8: Restore the original default (US2)

**Why this step:** This returns the installation to its starting selection before cleanup.

**Needs:** Step 6.

**Run:**

```sh
./cc-deck/cc-deck config profile use claude/private
```

**What you should see:** The command confirms `claude/private` as the selected default.

**If you see something else:** Stop before deleting anything and inspect the profile list.

**Result:**

Passed (agent). The CLI confirmed `claude/private` as the default.


### Step 8 of 8: Remove the test profile and refresh wrappers (US2)

**Why this step:** This verifies deleting one pair leaves the same-named Claude profile intact.

**Needs:** Step 7.

**Run:**

```sh
./cc-deck/cc-deck config profile delete codex/private && ./cc-deck/cc-deck config profile sync && ./cc-deck/cc-deck config profile list
```

**What you should see:** The Codex profile is removed, sync completes, and `claude/private` remains listed as the default.

**If you see something else:** Check the delete message and sync output, then verify the qualified list before retrying.

**Result:**

Passed (agent). `codex/private` was deleted, sync exited 0, and the final list showed `claude/private` (displayed as `claude-private`) still default. Sync reported several cache files were not symlinks and left them unchanged.


## Coverage

- US1: covered by adding a same-named profile under another harness and listing both qualified identities.
- US2: covered by qualified show, ambiguous bare show, exact default selection, and pair-specific deletion.
- US3: the current installation was migrated separately; see [VERIFY.md](VERIFY.md).
- US4: snapshot identity is covered by automated round-trip tests; this runbook does not launch interactive Claude and Codex sessions.

## Results

The interactive walkthrough completed successfully. Both harness-scoped profiles were distinguishable, bare-name ambiguity was reported, defaults selected and restored by qualified identity, and deleting the Codex test profile preserved the Claude profile.
