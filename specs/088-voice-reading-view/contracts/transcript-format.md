# Contract: Transcript File Format

Unchanged:
- File location and naming (`~/.local/share/cc-deck/transcripts/<name>.txt` or an absolute path).
- Append mode; one line per written segment.
- With timestamps enabled, each line is prefixed `[HH:MM:SS] ` using the time the line is written (current format, FR-031).
- Text written while paused is discarded.

New:
- Before writing a segment with `TurnStart = true`, the writer emits exactly one empty line, unless the file has no text from this recording yet.
- A passage with a turn change is written as separate lines, one per segment, with the blank line between the turns.
- Consecutive segments without `TurnStart` are written as separate lines without blank lines (same as today's one line per passage).

Example (timestamps off):

```text
So the first thing I wanted to cover is the release timeline.
And also the blockers.

Right, we still have the two blockers from the dependency update.

Okay, sounds good.
```

Example (timestamps on):

```text
[14:03:12] So the first thing I wanted to cover is the release timeline.

[14:03:25] Right, we still have the two blockers from the dependency update.
```
