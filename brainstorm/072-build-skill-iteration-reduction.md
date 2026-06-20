# Brainstorm: Build Skill Iteration Reduction

**Date:** 2026-06-17
**Status:** active
**Depends on:** 064-two-pass-binary-probing, 060-openshell-testing-findings

## Problem Framing

A full OpenShell build run (`/cc-deck.capture --all` followed by `/cc-deck.build --target openshell`) required 8 build iterations before producing a working image. Every failure was caused by a gap in the build skill's instructions that left the LLM guessing about something it could have been told upfront. The self-correction loop works, but each iteration costs 30-90 seconds of build time and burns tokens on error diagnosis.

The goal is to encode the lessons from this run into the skill so that a first-try build succeeds for common configurations.

## Failure Log

Nine distinct failures occurred during the build. Some were fixed together in a single iteration.

### 1. Base image runs as non-root user

**Error:** `apt-get update` returned "Permission denied" on `/var/lib/apt/lists/partial`.

**Root cause:** The OpenShell base image (`ghcr.io/nvidia/openshell-community/sandboxes/base:latest`) defaults to `USER sandbox`. The skill's snippet `01-header.txt` does not set `USER root`, and the generated system packages layer assumed it was already root.

**Fix:** Added `USER root` and `mkdir -p /var/lib/apt/lists/partial` before `apt-get update`.

**Skill gap:** The skill says "use `apt-get` for Debian/Ubuntu" but does not mention that the first generated layer after the header must set `USER root`. Section A (container build) does not need this because the cc-deck base image already runs as root.

### 2. rtk GitHub release asset pattern mismatch

**Error:** `curl` returned 404 for `rtk-aarch64-unknown-linux-gnu.tar.xz`.

**Root cause:** The actual asset is `rtk-aarch64-unknown-linux-gnu.tar.gz` (not `.tar.xz`). For x86_64, the asset uses `musl` instead of `gnu`: `rtk-x86_64-unknown-linux-musl.tar.gz`. The `asset_pattern` field in the manifest was a guess from the capture step, not verified against the actual release.

**Fix:** Queried the GitHub API for actual asset names. Used conditional logic to select the correct suffix per architecture.

**Skill gap:** The skill trusts `asset_pattern` from the manifest without verification. No instruction to probe actual release assets before generating download commands.

### 3. abtop binary nested in subdirectory

**Error:** `chmod: cannot access '/usr/local/bin/abtop': No such file or directory`.

**Root cause:** The tarball `abtop-aarch64-unknown-linux-gnu.tar.xz` contains a nested directory (`abtop-aarch64-unknown-linux-gnu/abtop`), not a flat binary at the top level. `tar -xJf - -C /usr/local/bin/` extracted the directory structure, not the binary.

**Fix:** Used `--strip-components=1` with a path filter to extract the binary directly.

**Skill gap:** The skill assumes all GitHub release tarballs contain a flat binary. No instruction to probe tarball structure before generating extraction commands.

### 4. cc-session and cc-setup download patterns wrong in mandatory snippet

**Error:** `tar: cc-session: Not found in archive` followed by 404 for cc-setup.

**Root cause:** Two separate issues in the mandatory stack snippet `03-mandatory-stack.txt`:
- cc-session's tarball nests the binary in a subdirectory (same issue as abtop)
- cc-setup uses a completely different naming convention: `cc-setup-{version}-linux-{goarch}.tar.gz` (Go-style versioned naming, not Rust-style `{arch}-unknown-linux-{libc}`)

The snippet assumed both tools follow the same pattern.

**Fix:** Replaced the snippet's combined download loop with separate commands, each using the correct URL pattern and extraction method. Used the GitHub API to fetch the cc-setup version dynamically.

**Skill gap:** The skill says "copy snippet content EXACTLY as-is" but the snippet itself was wrong. No escape hatch for when snippets contain broken commands.

### 5. Claude Code native installer OOM-killed

**Error:** `bash: line 158: 32 Killed "$binary_path" install` (exit 137).

**Root cause:** The native Claude Code installer (`curl -fsSL https://claude.ai/install.sh | bash`) downloads and extracts a large binary. The container build environment ran out of memory during extraction.

**Fix:** Switched to npm install (`npm install -g @anthropic-ai/claude-code@latest`) with a user-local npm prefix (`/sandbox/.npm-global`), since the system prefix `/usr` requires root.

**Skill gap:** The skill only mentions the native installer. No fallback path for memory-constrained builds. No mention that npm global prefix must be overridden for non-root users.

### 6. Cache directory permission denied

**Error:** `EACCES: permission denied, mkdir '/sandbox/.cache/claude'`.

**Root cause:** The cc-deck install step ran `mkdir -p /sandbox/.cache/zellij` as root, which created `/sandbox/.cache/` owned by root. The chown only covered `/sandbox/.cache/zellij`, not the parent. When Claude Code (running as sandbox) tried to create `/sandbox/.cache/claude`, it could not write to the root-owned parent.

**Fix:** Changed `chown -R sandbox:sandbox /sandbox/.cache/zellij` to `chown -R sandbox:sandbox /sandbox/.cache`.

**Skill gap:** The snippet documentation does not call out that chown must cover the parent directory, not just the specific subdirectory.

### 7. Official Claude plugins marketplace not configured

**Error:** `Plugin "clangd-lsp" not found in any configured marketplace`.

**Root cause:** A fresh Claude Code installation has no marketplaces configured. The plugin install commands attempted `claude plugins install clangd-lsp` without first adding the `anthropics/claude-plugins-official` marketplace.

**Fix:** Added `claude plugins marketplace add anthropics/claude-plugins-official` before the first `claude plugins install` call.

**Skill gap:** The skill's plugin handling section says `source: marketplace -> claude plugins install <name>` but does not mention that the official marketplace must be explicitly added first in a fresh installation.

### 8. starship and lsd not installed (runtime failure)

**Error:** After a successful build, the container shell showed `command not found: lsd` and raw prompt escape codes instead of the starship prompt.

**Root cause:** The curated shell config (from `/cc-deck.capture`) aliases `ls` to `lsd` and expects `starship` for the prompt. Neither is in the OpenShell base image (Ubuntu 24.04). The cc-deck base image (Fedora) includes both, so the skill's base-image probing step did not check for them. The probing step only checks a fixed list (`git node python3 npm curl rg`) and does not cross-reference tools referenced in the shell config.

**Fix:** Added GitHub release installs for starship and lsd before the tool installation layers.

**Skill gap:** The base-image probing instructions check for a fixed set of tools but do not scan the curated shell config for implicit dependencies.

## Proposed Skill Changes

### Change 1: USER root after header for OpenShell builds

**Section:** C2, assembly order, between step 1 and step 3.

Add explicit instruction that after copying `01-header.txt`, a `USER root` line must be added before any generated `RUN` layers. The OpenShell base image defaults to `USER sandbox`. This is not needed for container builds (Section A) because the cc-deck base image runs as root.

### Change 2: GitHub release asset verification before Containerfile generation

**Section:** C2 (and A2), under "Tool resolution" for `install: github-release`.

Add a mandatory pre-generation step: before writing any download commands, query the GitHub API (`/repos/<repo>/releases/latest`) to get actual asset names. Then probe the tarball structure (`tar -t`) to determine whether the binary is flat or nested. Record the actual URL, format, and extraction method for each tool. Generate Containerfile commands from these verified values, not from the manifest's `asset_pattern` hint.

This is a one-time cost during Containerfile generation (before the build starts) that prevents multiple build-fail-fix cycles.

### Change 3: Allow snippet modification when commands are broken

**Section:** C2, the note about snippets.

Change the "copy verbatim" rule to: "Copy snippet content as-is unless a download command in the snippet produces a 404 or extraction error. In that case, verify the download URL against the GitHub API and fix the command. Document the modification with a comment."

This is the pragmatic escape hatch. The snippets are generated by `build refresh` and are usually correct, but they encode assumptions about tarball structure and naming conventions that can drift when upstream projects change their release format.

### Change 4: Claude Code npm fallback for OOM scenarios

**Section:** C2, in the mandatory stack documentation.

Add a fallback path: if the native Claude Code installer fails with exit 137 (OOM), switch to npm install. Include the exact commands including npm prefix override for non-root users.

### Change 5: Cache directory ownership

**Section:** C2, mandatory stack snippet documentation.

Add a note that chown must cover `/sandbox/.cache` (not just `/sandbox/.cache/zellij`) so that subsequent tools can create sibling directories.

### Change 6: Official marketplace setup before plugin installs

**Section:** C2 (and A2), under "Plugin handling".

Add `claude plugins marketplace add anthropics/claude-plugins-official` as the first step before any plugin installation. A fresh Claude Code installation has no marketplaces.

### Change 7: Shell config dependency scanning during base image probing

**Section:** C2, under "Base image probing".

Add a third probing step: scan the curated shell config (`settings.shell_rc`) for commands used in aliases and eval statements. Cross-reference against the base image probe results. Install any missing tools that the shell config references (starship, lsd, zoxide, bat, etc.) from GitHub releases or the package manager.

### Change 8: OpenShell base image note in Key Rules

**Section:** "Key Rules (all targets)" at the bottom.

Add a note clarifying that the OpenShell base image is Ubuntu (not Fedora), runs as `sandbox` (not root), and does NOT include lsd, starship, zsh, bat, or ripgrep by default. The cc-deck container base image (Fedora) does include these. This distinction is easy to miss and caused multiple failures.

## Impact Assessment

Changes 1 through 8 together would have eliminated all 8 build iterations in this run. The build would have succeeded on the first attempt.

The highest-value changes are #2 (asset verification) and #7 (shell config scanning) because they prevent entire categories of failures rather than individual cases. Change #3 (snippet modification escape hatch) is important for long-term maintenance because snippet accuracy will continue to drift as upstream projects change their release formats.

## Validation Run (2026-06-19)

A second independent build was run on a fresh test workspace (`cc-deck-test`) using `/cc-deck.capture` followed by `/cc-deck.build --target openshell`. This run reproduced 7 of the 8 original failures and surfaced 2 new ones. Total: 9 build iterations before success.

### Reproduced failures

Failures #1 through #7 all recurred exactly as documented above, confirming that the proposed skill changes have not yet been applied. The self-correction loop fixed each one, but at the cost of 9 iterations and significant token spend.

### NOT reproduced: #5 (Claude Code OOM)

The native Claude Code installer succeeded on this run without an OOM kill. This suggests the OOM failure may have been caused by a concurrent memory-intensive layer or a host-specific memory constraint, not an inherent problem with the installer. Change #4 (npm fallback) is still worth keeping as a resilience measure, but it may not be needed for standard builds.

### New failure #9: jq settings merge loses array context

**Error:** `jq: error: Cannot index object with number` when merging Claude settings.

**Root cause:** The merge command used `jq -s '.[0] * .[1] | .hooks = .[0].hooks'`. After the pipe, `.[0]` refers to the merged object (which is not an array), not the original first file. The expression needs a variable binding to preserve the reference.

**Fix:** Changed to `jq -s '.[0] as $orig | $orig * .[1] | .hooks = $orig.hooks'`.

**Skill gap:** The settings merge example in Section C2 (and A2) does not include actual jq commands. The LLM generated a plausible but incorrect expression. The skill should provide the exact jq command for the settings merge rather than leaving it to generation.

### New failure #10: post_install commands need directory setup

**Error:** `rtk: Failed to create directory: /sandbox/.config/rtk: Permission denied (os error 13)` when running `rtk init -g`.

**Root cause:** The `post_install` field (`rtk init -g`) runs as `USER sandbox`, but `/sandbox/.config/rtk/` does not exist and the parent `/sandbox/.config/` is owned by root (created by earlier root-level layers). The sandbox user cannot create new subdirectories.

**Fix:** Added `mkdir -p /sandbox/.config/rtk && chown sandbox:sandbox /sandbox/.config/rtk` before the `USER sandbox` / `RUN rtk init -g` block. Also added `|| true` since rtk's interactive prompt detection can cause partial failures in non-interactive container builds.

**Skill gap:** The skill defines `post_install` as a shell command that runs after the tool binary is installed, but does not mention that:
1. The command runs as `sandbox` user (not root)
2. Any config directories the command needs must be pre-created and chowned
3. Commands that expect interactive input should be guarded with `|| true` or `yes |` wrappers

### New runtime issue #11: starship errors under TERM=dumb

**Error:** `[ERROR] - (starship::print): Under a 'dumb' terminal (TERM=dumb).` printed to stderr during `cc-deck ws new` sandbox exec commands (e.g., git clone, env var injection).

**Root cause:** The `05-shell-finalize.txt` snippet appends `eval "$(starship init zsh)"` unconditionally to `.zshrc`. When `cc-deck ws new` runs exec commands inside the sandbox, it sets `TERM=dumb` to suppress escape sequences. But starship init still loads and prints an error to stderr. This error pollutes command output and can confuse log parsing.

This is not a build-time failure (the image builds fine), but a runtime failure that appears on first use of every sandbox with starship configured.

**Fix:** Guard starship init with a TERM check in the `05-shell-finalize.txt` snippet:
```zsh
[[ "$TERM" != "dumb" ]] && eval "$(starship init zsh)"
```

**Skill gap:** The shell-finalize snippet and the skill's starship init instructions do not account for non-interactive exec contexts where TERM=dumb. The skill mentions guarding with `[[ $- == *i* ]]` for interactive checks, but TERM=dumb is a separate condition that can occur even in interactive shells (e.g., when the terminal emulator sets it explicitly).

### New runtime issue #12: fzf --zsh unsupported on Ubuntu 24.04

**Error:** `unknown option: --zsh` printed on shell startup.

**Root cause:** The curated zshrc contains `source <(fzf --zsh)`, which requires fzf 0.48+. Ubuntu 24.04's apt package ships fzf 0.44, which does not support the `--zsh` flag. The older init method uses `source /usr/share/doc/fzf/examples/key-bindings.zsh`.

The capture step copied the `source <(fzf --zsh)` line from the host's macOS config (Homebrew fzf 0.56+) without checking whether the target image's fzf version supports it.

**Fix (preferred):** Install fzf from GitHub releases instead of apt, same pattern as other tools:
```dockerfile
RUN FZF_VERSION=$(curl -fsSL https://api.github.com/repos/junegunn/fzf/releases/latest | jq -r '.tag_name' | sed 's/^v//') && \
    curl -fsSL "https://github.com/junegunn/fzf/releases/download/v${FZF_VERSION}/fzf-${FZF_VERSION}-linux_$(dpkg --print-architecture).tar.gz" \
      | tar -xzf - -C /usr/local/bin fzf
```

**Fix (alternative):** Version-aware guard in the curated zshrc:
```zsh
if fzf --zsh &>/dev/null; then
  source <(fzf --zsh)
elif [ -f /usr/share/doc/fzf/examples/key-bindings.zsh ]; then
  source /usr/share/doc/fzf/examples/key-bindings.zsh
  source /usr/share/doc/fzf/examples/completion.zsh
fi
```

**Skill gap:** The capture step's shell config curation does not check whether commands used in the curated config are version-compatible with the target image. The `source <(fzf --zsh)` pattern works on macOS (Homebrew) but not on Ubuntu's older apt package.

### New runtime issue #13: zsh tab completion broken (missing compinit)

**Error:** Tab completion on `ls` completes binaries in `/usr/local/bin` instead of files in the current directory. zstyle and compdef calls in the curated zshrc have no effect.

**Root cause:** The original `.zshrc` loaded `compinit` through the Antidote plugin manager (which sources `ohmyzsh/ohmyzsh path:lib`, which calls `compinit`). The capture step correctly stripped Antidote as macOS/Homebrew-specific, but did not notice that `compinit` was a transitive dependency loaded through Antidote. Without `compinit`, the zsh completion system is not initialized, so `zstyle`, `compdef`, and tab completion all fall back to basic file globbing.

**Fix:** Add `autoload -Uz compinit && compinit -C` to the curated zshrc, before any `zstyle` or `compdef` lines.

**Skill gap:** The capture step's "Strip out" rules remove plugin managers (Antidote, oh-my-zsh) but do not identify essential services those managers provide. `compinit` is the most critical transitive dependency: without it, the entire zsh completion system is non-functional. The skill should list `compinit` as a required preamble whenever `zstyle` or `compdef` lines are present in the curated config.

## Proposed Skill Changes (updated)

### Change 1-8: (unchanged from original analysis)

See above. All remain valid and unimplemented.

### Change 9: Provide exact jq merge command for settings.json

**Section:** C2 (and A2), under "Settings handling", the merge strategy documentation.

Add the exact jq command:
```bash
jq -s '.[0] as $orig | $orig * .[1] | .hooks = $orig.hooks' \
  /sandbox/.claude/settings.json /tmp/user-settings.json > /tmp/merged.json
```

Do not leave the merge implementation to the LLM. jq's `-s` slurp mode has a non-obvious scoping behavior that causes incorrect code on first attempts.

### Change 10: post_install sandboxing protocol

**Section:** C2 (and A2), under "GitHub release tools layer" where `post_install` is processed.

Add explicit instructions for post_install handling:
1. Before running post_install, switch to `USER root` and create any config directories the command might need: `mkdir -p /sandbox/.config/<tool> && chown sandbox:sandbox /sandbox/.config/<tool>`
2. Switch to `USER sandbox` for the actual command
3. Append `|| true` to the command unless the tool is critical to the build
4. Switch back to `USER root` after
5. Non-interactive builds: if the command prompts for input (detected by phrases like "Y/N", "Patch existing"), it will fail. The `|| true` guard handles this.

### Change 11: Guard starship init against TERM=dumb

**Section:** C2, in the shell-finalize snippet documentation, and the `05-shell-finalize.txt` snippet itself.

Change the starship init line from:
```bash
echo 'eval "$(starship init '"$SHELL_NAME"')"' >> "$RC"
```
to:
```bash
echo '[[ "$TERM" != "dumb" ]] && eval "$(starship init '"$SHELL_NAME"')"' >> "$RC"
```

This prevents starship from emitting errors when `cc-deck ws new` runs sandbox exec commands with `TERM=dumb`. The same guard should be applied in the curated shell config generated by `/cc-deck.capture` if starship init is included there.

Also applies to `build refresh` when regenerating the `05-shell-finalize.txt` snippet.

### Change 12: Install fzf from GitHub releases (not apt)

**Section:** C2, under "System packages layer" and the shell config dependency scanning (Change #7).

When fzf is detected as a shell config dependency (via `source <(fzf --zsh)` or similar), install it from GitHub releases instead of the distro package manager. Ubuntu 24.04 ships fzf 0.44, which lacks the `--zsh` flag added in 0.48. The GitHub release is a single static binary and follows the same install pattern as other tools.

This is a specific instance of a broader pattern: the capture step copies macOS shell config verbatim, but distro-packaged tool versions may be too old for the syntax used. Change #7 (shell config scanning) should detect `fzf --zsh` and flag fzf for GitHub release install rather than apt.

### Change 13: Add compinit preamble when stripping plugin managers

**Section:** The capture skill (`/cc-deck.capture`), Step 5c, under "Strip out" rules.

When the capture step strips a plugin manager (Antidote, oh-my-zsh, zinit, etc.), it must check whether the curated config contains `compdef`, `zstyle ':completion:*'`, or other completion-dependent directives. If so, add `autoload -Uz compinit && compinit -C` as a preamble to the curated config.

This is a category-level fix: any plugin manager that loads `compinit` transitively will cause the same breakage when stripped. The rule is simple: if the curated config uses completion features, `compinit` must be present.

## Updated Impact Assessment

Changes 1-13 together would have eliminated all build iterations in both the original run (8 iterations) and the validation run (9 iterations), plus all three runtime issues observed during workspace testing (starship TERM=dumb, fzf --zsh version mismatch, broken tab completion). The two runs had significant overlap (7 shared failures) confirming that the failure categories are stable and predictable.

Priority ordering by value:
1. **Change #2** (asset verification) - prevents 2 failures per run, category-level fix
2. **Change #7** (shell config scanning) - prevents runtime failures and implicit dependency misses
3. **Change #13** (compinit preamble) - broken tab completion affects every interactive session
4. **Change #1** (USER root) - trivial to implement, prevents first failure in every OpenShell build
5. **Change #6** (marketplace setup) - simple addition, prevents failure in every build with plugins
6. **Change #9** (exact jq command) - prevents subtle logic bugs in settings merge
7. **Change #11** (starship TERM guard) - prevents runtime errors on every ws new/exec
8. **Change #12** (fzf from GitHub) - prevents fzf init errors on every shell startup
9. **Change #3** (snippet escape hatch) - structural change for long-term maintenance
10. **Change #10** (post_install protocol) - prevents failures for any tool with post-install hooks
11. **Change #5** (cache ownership) - one-line fix in snippet
12. **Change #4** (Claude Code npm fallback) - resilience measure, may not trigger in practice
13. **Change #8** (base image documentation) - documentation clarity, no code change

## Open Questions

- Should the capture step (`/cc-deck.capture`) verify GitHub release asset patterns at capture time rather than deferring to build time? This would catch mismatches earlier but adds network calls to the capture wizard.
- Should `build refresh` also verify snippet download commands when regenerating snippets? This would keep snippets accurate between builds.
- Should the skill mandate npm install for Claude Code in all OpenShell builds (not just as a fallback)? The npm approach uses less memory and produces a simpler PATH setup. The native installer's advantage is bundling its own Node.js, but the OpenShell base image already has Node.js.
- The validation run showed the native installer working without OOM. Should Change #4 be downgraded from "proposed" to "optional resilience measure"?
- Should `post_install` commands be verified at capture time (e.g., run them locally in a dry-run mode) to catch interactive prompt issues before build time?

---

## Revisit: 2026-06-20

### Open Questions Resolved

All five open questions from the original analysis are now decided:

1. **Asset verification timing:** Both capture and build time. Capture stores verified patterns in the manifest for fast feedback. Build re-verifies before generating Containerfile commands to catch drift.

2. **`build refresh` verification:** Yes. `build refresh` probes GitHub APIs and validates URLs in regenerated snippets. Catches upstream naming changes between builds.

3. **Claude Code installation strategy:** Native installer primary, npm fallback on exit 137 (OOM). The validation run showed the native installer succeeding, but npm fallback is kept as a resilience measure.

4. **`post_install` dry-run validation:** Yes, at capture time. The capture wizard runs post_install commands with --dry-run or --help flags to detect interactive prompt issues and missing directory requirements before build time.

5. **Spec structure:** Single spec covering all 13 changes. The changes are tightly related and span the same two skill files.

### Updated Problem Framing

The original analysis documented 13 proposed skill changes from two validation runs (8 + 9 iterations). This revisit confirms the full set for implementation and resolves the open questions that blocked specification.

### Approach Decision

**Approach A: Skill-First** was chosen over Code+Skill Hybrid and Build Pipeline Redesign.

The root cause of all 13 failures is insufficient instruction in the skill markdowns, not missing Go code. The LLM's self-correction loop works but should not need to fire for predictable failures. The fix is to encode the lessons directly into the skill instructions and templates.

**Approaches considered:**

**A: Skill-First (chosen)** - Edit `cc-deck.build.md`, `cc-deck.capture.md`, and Go template files directly. All 13 changes as instruction improvements.
- Pros: Fastest to implement. Directly addresses every failure. No new Go code.
- Cons: Relies on LLM following more complex instructions. Asset verification is procedural, not code-enforced.

**B: Code + Skill Hybrid** - Implement asset verification and shell config scanning as Go code, rest as skill edits.
- Pros: High-value changes enforced by code. `build refresh --verify` is reusable.
- Cons: More implementation effort. May overlap with spec 070 (base image probe).

**C: Build Pipeline Redesign** - New `cc-deck build preflight` subcommand with structured verification.
- Pros: Clean architecture. Single point of truth for verification.
- Cons: Largest scope. Adds new subcommand and data flow. Over-engineering risk.

### Files to Modify

- `internal/build/commands/cc-deck.build.md` (A2/C2: asset verification, USER root, snippet escape hatch, jq merge, post_install protocol, marketplace, Claude Code fallback, starship guard, base image docs)
- `internal/build/commands/cc-deck.capture.md` (Step 5: shell config scanning, compinit preamble, fzf detection; Step 11: asset verification, post_install dry-run)
- `internal/build/templates/containerfile/05-shell-finalize.tmpl` (TERM=dumb guard for starship)
- `internal/build/templates/containerfile/03-mandatory-stack.tmpl` (cache ownership, marketplace setup)

### Success Criteria

A fresh OpenShell build (`/cc-deck.capture --all` followed by `/cc-deck.build --target openshell`) succeeds on the first attempt without self-correction iterations for any of the 13 documented failure categories.

### Scope Boundaries

**In scope:** All 13 changes, asset verification at both capture and build, snippet verification on refresh, post_install dry-run at capture.

**Out of scope:** Go code for asset verification (follow-up if skill instructions prove insufficient), new CLI subcommands, multi-agent build support (separate brainstorm 070).
