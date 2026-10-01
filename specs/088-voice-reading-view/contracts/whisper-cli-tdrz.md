# Contract: TurnTranscriber and whisper-cli Invocation

## Go interface (`internal/voice`)

```go
// TurnTranscriber transcribes one passage and reports speaker turns.
type TurnTranscriber interface {
    // TranscribeTurns returns segments in passage order. Segment offsets are
    // relative to the passage start. TurnStart on segment i+1 is true when
    // the model reported a speaker change after segment i. The first
    // segment's TurnStart is false; the relay applies the pause-break rule.
    TranscribeTurns(ctx context.Context, audio []int16, sampleRate int) ([]Segment, error)
    Close() error
}
```

Behavioral requirements:
1. MUST honor `ctx` cancellation and deadlines (the relay uses a 30 second deadline).
2. MUST return an error (not empty segments) when the tool exits non-zero, the JSON file is missing, or the JSON does not parse.
3. MUST return empty segments without error when the tool succeeds but transcribes nothing.
4. MUST NOT write outside a private temp directory, which it removes before returning.
5. MUST pass the glossary prompt when one is set (`SetPrompt`), mirroring the HTTP transcriber.

## Invocation

```text
whisper-cli -m <ModelPath("small.en-tdrz")> -f <tmp>/passage.wav -tdrz -oj -of <tmp>/out -np [--prompt <glossary>]
```

Output file `<tmp>/out.json` (non-full JSON, relevant fields only):

```json
{
  "transcription": [
    { "offsets": { "from": 0,    "to": 2380 }, "text": " So what about Friday?", "speaker_turn_next": true },
    { "offsets": { "from": 2380, "to": 4100 }, "text": " Friday works for me.",  "speaker_turn_next": false }
  ]
}
```

Mapping: `Start = offsets.from ms`, `End = offsets.to ms`, `Text` sanitized with the relay helpers, `TurnStart(i+1) = speaker_turn_next(i)`. A missing `speaker_turn_next` field is treated as false.

## Availability

`TdrzStatus() (toolErr, modelErr error)`:
- `toolErr` non-nil when `exec.LookPath("whisper-cli")` fails.
- `modelErr` non-nil when `ModelPath("small.en-tdrz")` does not exist.

## Testability

The runner is a package-level variable or struct field (`runCmd func(ctx, name string, args ...string) error`) so tests can substitute a fake that writes a canned `out.json`.
