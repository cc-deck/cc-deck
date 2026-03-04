# Data Model: Plugin Lifecycle Management

**Feature**: 009-plugin-lifecycle

## Entities

### PluginInfo

Represents the embedded plugin metadata and binary.

| Field         | Type   | Description                                      |
|---------------|--------|--------------------------------------------------|
| Version       | string | Semantic version of the embedded plugin           |
| SDKVersion    | string | Zellij SDK version the plugin was built against   |
| MinZellij     | string | Minimum compatible Zellij version                 |
| BinarySize    | int64  | Size of the embedded WASM binary in bytes         |
| Binary        | []byte | The embedded WASM binary data (via go:embed)      |

### ZellijInfo

Represents the detected Zellij installation on the system.

| Field       | Type   | Description                                        |
|-------------|--------|----------------------------------------------------|
| Installed   | bool   | Whether Zellij was found on PATH                   |
| Version     | string | Detected Zellij version (empty if not installed)   |
| BinaryPath  | string | Path to the Zellij binary                          |
| ConfigDir   | string | Resolved config directory (env or default)         |
| PluginsDir  | string | Resolved plugins directory (ConfigDir/plugins/)    |
| LayoutsDir  | string | Resolved layouts directory (ConfigDir/layouts/)    |

### InstallState

Represents the current installation state of the plugin.

| Field              | Type   | Description                                      |
|--------------------|--------|--------------------------------------------------|
| PluginInstalled    | bool   | Whether cc_deck.wasm exists in the plugins dir   |
| PluginPath         | string | Full path to the installed plugin binary          |
| PluginSize         | int64  | File size of the installed binary                 |
| LayoutInstalled    | bool   | Whether cc-deck.kdl exists in the layouts dir    |
| LayoutPath         | string | Full path to the installed layout file            |
| LayoutType         | string | "minimal" or "full" (detected from content)      |
| DefaultInjected    | bool   | Whether the default layout has the plugin pane    |
| DefaultLayoutPath  | string | Path to the default layout file (if it exists)    |
| Compatibility      | string | "compatible", "untested", or "incompatible"       |

## Relationships

```
PluginInfo (embedded, immutable)
    ├── provides Binary → written to PluginPath in InstallState
    └── provides SDKVersion → compared against ZellijInfo.Version

ZellijInfo (detected at runtime)
    ├── ConfigDir → determines PluginsDir, LayoutsDir
    └── Version → determines Compatibility in InstallState

InstallState (computed from filesystem + ZellijInfo)
    ├── PluginInstalled depends on ZellijInfo.PluginsDir
    ├── LayoutInstalled depends on ZellijInfo.LayoutsDir
    └── DefaultInjected depends on ZellijInfo.ConfigDir + sentinel markers
```

## Layout Templates

Two predefined layout templates stored as Go string constants:

### Minimal Layout

Contains: one terminal pane + plugin status bar pane.
File: `cc-deck.kdl` in layouts directory.

### Full Layout

Contains: default tab template with plugin status bar, preconfigured plugin options, tab bar disabled.
File: `cc-deck.kdl` in layouts directory (overwrites minimal if switching).

## Injection Markers

Default layout injection uses sentinel comments for detection and removal:

- Start marker: `// cc-deck-plugin-start (managed by cc-deck, do not edit)`
- End marker: `// cc-deck-plugin-end`

Detection: scan file for start marker.
Removal: delete all lines from start marker through end marker (inclusive).
