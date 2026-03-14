# Task: Add --format json flag

Add a `--format` flag to the CLI that supports `text` (default) and `json` output formats.

## Requirements

- Add a `--format` flag that accepts `text` or `json`
- Default format should be `text` (current behavior, unchanged)
- When `--format json` is used, output each city's weather as a JSON object
- Use the `encoding/json` package from the standard library
- Parse flags before city arguments using the `flag` package
