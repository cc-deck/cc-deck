# Contract: Policy Component YAML Schema

## Extended Fields

### probe_binaries

- **Type**: list of strings
- **Required**: No
- **Default behavior when absent**: falls back to `match.tools` entries
- **Validation**: entries must not contain `/` (binary names only, not paths)
- **Ignored when**: component has explicit `binaries` field with len > 0

Each entry is passed as an argument to `which <name>` inside the probe container. If `which` fails, `find / -name <name> -type f -executable` runs as fallback.

### runtime_globs

- **Type**: list of strings
- **Required**: No
- **Default behavior when absent**: no glob patterns added
- **Validation**: entries must start with `/` (absolute paths with glob wildcards)
- **Ignored when**: component has explicit `binaries` field with len > 0

Glob patterns are added to the component's `binaries` field alongside probed paths. They cover binaries created at runtime that did not exist when the image was built.

## Precedence Rules

1. Components with explicit `binaries` in YAML are never overwritten by probe results or globs.
2. `probe_binaries` takes precedence over `match.tools` for determining which binaries to search for.
3. Probed paths and `runtime_globs` are combined (deduplicated) into the final `binaries` field.
4. If a binary is not found by probe and has no matching glob, a warning is logged but the build continues.

## Backward Compatibility

- Existing component YAML files without `probe_binaries` or `runtime_globs` continue to work.
- Components with explicit `binaries` are completely unaffected by this change.
- The well-known paths table is removed; components that relied on it must add `probe_binaries` and/or `runtime_globs` to their YAML.
