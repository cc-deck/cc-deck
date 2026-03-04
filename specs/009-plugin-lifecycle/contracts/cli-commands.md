# CLI Contract: Plugin Commands

**Feature**: 009-plugin-lifecycle

## Command: `cc-deck plugin install`

**Synopsis**: `cc-deck plugin install [flags]`

**Description**: Install the Zellij session management plugin and layout file.

**Flags**:

| Flag                | Short | Type   | Default   | Description                                      |
|---------------------|-------|--------|-----------|--------------------------------------------------|
| `--force`           | `-f`  | bool   | false     | Overwrite existing installation without prompting |
| `--layout`          | `-l`  | string | "minimal" | Layout template: "minimal" or "full"             |
| `--inject-default`  |       | bool   | false     | Inject plugin pane into default Zellij layout     |

**Exit codes**:

| Code | Meaning                         |
|------|----------------------------------|
| 0    | Installation succeeded           |
| 1    | File I/O or permission error     |
| 2    | User cancelled (declined prompt) |

**Output (stdout)**:

```
Plugin installed successfully.

  Binary:  ~/.config/zellij/plugins/cc_deck.wasm (1.2 MB)
  Layout:  ~/.config/zellij/layouts/cc-deck.kdl (minimal)

To start Zellij with the plugin:

  zellij --layout cc-deck
```

When `--inject-default` is used, adds:

```
  Default layout injected: ~/.config/zellij/layouts/default.kdl
```

**Stderr warnings**:

- `Warning: Zellij not found on PATH. Install Zellij first.`
- `Warning: Zellij version 0.38.0 may be incompatible (requires 0.40+).`

---

## Command: `cc-deck plugin status`

**Synopsis**: `cc-deck plugin status`

**Description**: Show the current installation state of the plugin.

**Flags**: None (inherits global `--output` for JSON/YAML)

**Exit codes**:

| Code | Meaning                |
|------|------------------------|
| 0    | Status retrieved       |

**Output (stdout, text mode)**:

```
Plugin Status
  Installed:      yes
  Path:           ~/.config/zellij/plugins/cc_deck.wasm
  Size:           1.2 MB
  Version:        0.1.0

Zellij
  Installed:      yes (0.43.1)
  Compatibility:  compatible
  Config dir:     ~/.config/zellij/

Layouts
  cc-deck.kdl:    installed (minimal)
  Default layout: not injected
```

When not installed:

```
Plugin Status
  Installed:      no
  Expected path:  ~/.config/zellij/plugins/cc_deck.wasm

Zellij
  Installed:      yes (0.43.1)
  Compatibility:  compatible
  Config dir:     ~/.config/zellij/
```

**Output (JSON mode)**:

```json
{
  "plugin": {
    "installed": true,
    "path": "~/.config/zellij/plugins/cc_deck.wasm",
    "size": 1258291,
    "version": "0.1.0"
  },
  "zellij": {
    "installed": true,
    "version": "0.43.1",
    "compatibility": "compatible",
    "configDir": "~/.config/zellij/"
  },
  "layouts": {
    "ccDeckLayout": "minimal",
    "defaultInjected": false
  }
}
```

---

## Command: `cc-deck plugin remove`

**Synopsis**: `cc-deck plugin remove`

**Description**: Remove the plugin binary, layout file, and undo default layout injection.

**Flags**: None

**Exit codes**:

| Code | Meaning                          |
|------|-----------------------------------|
| 0    | Removal succeeded or nothing found |

**Output (stdout)**:

When plugin was installed:

```
Plugin removed.

  Removed: ~/.config/zellij/plugins/cc_deck.wasm
  Removed: ~/.config/zellij/layouts/cc-deck.kdl
```

When default layout was injected:

```
  Reverted: ~/.config/zellij/layouts/default.kdl (plugin pane removed)
```

When Zellij is running:

```
  Warning: Zellij may be running. Restart Zellij sessions to fully unload the plugin.
```

When nothing was installed:

```
Nothing to remove. Plugin is not installed.
```
