# Review Guide: Voice Attend Stop Word

**Spec:** [spec.md](spec.md) | **Plan:** [plan.md](plan.md) | **Tasks:** [tasks.md](tasks.md)
**Generated:** 2026-04-30

---

## What This Spec Does

Adds a voice command "next" that cycles to the next attended session, matching the Alt+a keyboard shortcut. This lets users stay hands-free when working across multiple Claude Code sessions instead of reaching for the keyboard to switch. The trigger word is configurable via the same mechanism as the existing "send" stop word.

**In scope:** New "attend" action in DefaultCommands, `[[attend]]` command protocol payload, plugin handler for the new command, unit tests, documentation.

**Out of scope:** Reverse direction ("prev"/"back") stop word, stop words for other actions (pause, mute), changes to the attend cycling algorithm itself. Reverse cycling is intentionally excluded because keyboard Alt+A already covers it and voice commands work best with a small vocabulary.

## Bigger Picture

This builds on spec 045 (voice sidebar integration) which established the `[[command]]` protocol on the `cc-deck:voice` pipe. Spec 045 introduced voice relay, mute toggle, and the `[[enter]]` command. This spec is the second command in that protocol, validating that the protocol is extensible as designed.

If more voice commands are needed in the future (pause, mute toggle, new session), the pattern established here (action in DefaultCommands, case in relay dispatch, match arm in plugin) becomes the template. The brainstorm's open thread about whether more actions will be needed is worth keeping in mind.

---

## Spec Review Guide (30 minutes)

> Focus your time on the design decisions and edge cases. The implementation is small (3 production files, ~20 lines of code).

### Understanding the approach (8 min)

Read [Functional Requirements](spec.md#functional-requirements) (FR-001 through FR-007) and the [Implementation Approach](plan.md#implementation-approach). As you read, consider:

- Does the three-layer approach (DefaultCommands -> relay dispatch -> plugin handler) feel right, or is there a simpler path?
- Is reusing the `cc-deck:voice` pipe for `[[attend]]` the right call, versus sending on the `cc-deck:attend` pipe that the keybinding already uses?

### Key decisions that need your eyes (12 min)

**Default trigger word "next"** ([Edge Cases](spec.md#edge-cases))

"next" is a common English word. The standalone detection (only fires when the entire utterance after filler stripping equals "next") mitigates false positives, but "next" said in isolation between sentences could still trigger unexpectedly.
- Question for reviewer: Is "next" a good default, or would a less common word (like "switch" or "hop") cause fewer accidental triggers in practice?

**Tiered attend behavior** ([FR-004](spec.md#functional-requirements))

The voice command matches Alt+a exactly: waiting sessions first, then done, then idle. This means the voice user gets the same smart cycling behavior.
- Question for reviewer: Should voice "next" always match Alt+a, or could there be cases where a simpler "just go to the next tab" would be more intuitive for voice users?

**Same-word conflict handling** ([Edge Cases](spec.md#edge-cases))

If a user configures the same word for both "submit" and "attend", map key overwrite determines which wins. The spec documents this as last-wins behavior.
- Question for reviewer: Should this be an error or warning instead of silent overwrite? A user might not realize their "send" command stopped working.

### Areas where I'm less certain (5 min)

- [Edge Cases](spec.md#edge-cases): The same-word conflict behavior depends on Go map iteration order during `BuildCommandMap`, which is non-deterministic. The "last-wins" claim may not hold consistently. This might need explicit conflict detection.
- [Assumptions](spec.md#assumptions): The spec assumes `perform_attend_next()` is accessible from `handle_voice_command()`. The research confirms this, but the exact import path and method signature should be verified during implementation.

### Risks and open questions (5 min)

- If a user frequently says "next" as a standalone utterance in natural speech (e.g., dictating a numbered list: "first... next... next..."), will false triggers be frustrating enough to make the feature unusable without reconfiguration?
- The spec depends on spec 045 being fully implemented. If the `[[command]]` protocol handling has gaps, this feature inherits those gaps.

---

*Full context in linked [spec](spec.md) and [plan](plan.md).*
