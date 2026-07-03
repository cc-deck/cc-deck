# Research: Voice Glossary

## R1: Whisper Prompt Parameter

**Decision**: Use `prompt` form field in multipart POST to whisper-server `/inference`.

**Rationale**: whisper.cpp (which powers whisper-server) accepts an `initial_prompt` / `prompt` parameter that biases the decoder. Limited to 224 tokens (~800 chars). Terms placed last in the prompt have higher influence.

**Source**: [Whisper prompting guide](https://cookbook.openai.com/examples/whisper_prompting_guide)

## R2: State Dump Already Includes working_dir

**Decision**: No plugin changes needed. The `DumpStateResponse` already serializes full Session structs via serde, which includes `working_dir: Option<String>`.

**Rationale**: Verified in `cc-zellij-plugin/src/controller/render_broadcast.rs`. The `build_dump_response()` function serializes `state.sessions` which are full `Session` structs.

## R3: Prompt Injection via Setter

**Decision**: Add `SetPrompt(string)` method to `httpTranscriber` rather than changing the `Transcriber` interface.

**Rationale**: The `Transcriber` interface is implemented by both `httpTranscriber` and `cliTranscriber`. The CLI transcriber doesn't support prompts (it spawns whisper-cli which has different argument handling). Changing the interface would require a no-op implementation on the CLI side. A setter is cleaner.
