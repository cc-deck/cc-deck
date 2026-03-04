# Research: Plugin Lifecycle Management

**Date**: 2026-03-04
**Feature**: 009-plugin-lifecycle

## Decision: Go embed for WASM binary

**Chosen**: Use `//go:embed` directive to embed the compiled WASM binary in the Go CLI binary.

**Rationale**: The Go project currently has no embedded files, but `//go:embed` is the standard approach for bundling static assets in Go. It produces a single self-contained binary with no runtime dependencies. The WASM binary is approximately 1.2 MB, which is acceptable overhead for the CLI binary.

**Alternatives considered**:
- Download on install: Requires network access, release management, and URL stability. Rejected per spec constraint SC-002 (offline installation).
- Ship as separate file: Requires users to manage file placement. Rejected because it defeats the single-binary goal.

## Decision: Zellij version detection via CLI

**Chosen**: Run `zellij --version` and parse the output (`zellij 0.43.1` format).

**Rationale**: Simple and reliable. The output format is stable (`<name> <semver>`). Use `exec.LookPath("zellij")` for installation detection.

**Alternatives considered**:
- Check for Zellij binary at known paths (/usr/bin, /usr/local/bin, /opt/homebrew/bin): Fragile, platform-specific. Rejected.
- Parse Zellij config for version hints: Config doesn't contain version info. Rejected.

## Decision: String-level layout injection (no KDL parser)

**Chosen**: Inject the cc-deck plugin pane block as a string append to the default layout, wrapped in sentinel comments for detection and removal.

**Rationale**: Go has no mature KDL parser library. String-level append is safe because the plugin pane block is always a top-level child in a `layout {}` block. Sentinel comments (`// cc-deck-plugin-start` / `// cc-deck-plugin-end`) enable reliable detection and clean removal.

**Alternatives considered**:
- Full KDL parsing in Go: No stable Go KDL library exists. Writing one is out of scope. Rejected.
- Regex-based injection: Fragile with nested KDL structures. Rejected.
- Require users to manually edit layouts: Defeats the purpose of automatic setup. Rejected.

## Decision: Atomic writes via temp file + rename

**Chosen**: Write plugin binary to a temporary file in the same directory, then `os.Rename()` to the final path.

**Rationale**: `os.Rename()` on the same filesystem is atomic on POSIX systems. Prevents corrupted binaries from interrupted writes. Standard Go pattern.

## Decision: Cobra subcommand structure

**Chosen**: Follow the existing cc-deck pattern with a parent `plugin` command and three subcommands: `install`, `status`, `remove`.

**Rationale**: Matches the existing `profile` command pattern which also has subcommands (`add`, `list`, `use`, `show`). Consistent user experience.

**Implementation pattern** (from existing codebase):
- Flags struct per command
- `NewPluginCmd()` constructor returns `*cobra.Command` with subcommands
- `runPluginInstall()` / `runPluginStatus()` / `runPluginRemove()` for business logic
- Register in `main.go` via `rootCmd.AddCommand()`

## Decision: New `internal/plugin/` package

**Chosen**: Create a new `internal/plugin/` package for all plugin lifecycle logic, mirroring the existing `internal/session/` pattern.

**Rationale**: The existing codebase separates domain logic from cobra commands. `internal/cmd/plugin.go` defines the command; `internal/plugin/` contains install, status, remove, layout, and zellij detection logic.

## Finding: Zellij directory structure

Standard paths (confirmed from local installation):

```
~/.config/zellij/              # Config root (overridable via ZELLIJ_CONFIG_DIR)
├── config.kdl                 # Main config
├── plugins/                   # Plugin binaries
│   └── cc_deck.wasm          # Target install location
├── layouts/                   # Layout files
│   └── cc-deck.kdl           # cc-deck layout (to be installed)
└── themes/                    # Theme files (not touched)
```

## Finding: Plugin pane KDL block

The exact KDL block to inject into layouts:

```kdl
// cc-deck-plugin-start (managed by cc-deck, do not edit)
pane size=1 borderless=true {
    plugin location="file:~/.config/zellij/plugins/cc_deck.wasm"
}
// cc-deck-plugin-end
```

For the minimal layout template:
```kdl
layout {
    pane
    pane size=1 borderless=true {
        plugin location="file:~/.config/zellij/plugins/cc_deck.wasm"
    }
}
```

For the full layout template:
```kdl
layout {
    default_tab_template {
        children
        pane size=1 borderless=true {
            plugin location="file:~/.config/zellij/plugins/cc_deck.wasm" {
                idle_timeout "300"
            }
        }
    }
    tab name="claude" focus=true {
        pane
    }
}
```

## Finding: SDK compatibility mapping

| Zellij Version | SDK Version | Compatibility |
|---------------|-------------|---------------|
| < 0.40        | < 0.40      | Incompatible  |
| 0.40 - 0.43   | 0.43        | Compatible     |
| > 0.43        | 0.43        | Untested       |

The plugin uses `zellij-tile = "0.43"`. Zellij maintains backward compatibility within major SDK versions but newer Zellij versions may introduce features the plugin doesn't use.

## Finding: Existing Zellij plugin on disk

A cc_deck.wasm already exists at `~/.config/zellij/plugins/cc_deck.wasm` (1.2 MB) from manual installation. The `plugin install` command will detect this and prompt before overwriting.
