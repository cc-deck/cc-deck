# Data Model: Policy Binary Resolution

## Entities

### Well-Known Paths Table

A static mapping from tool names to lists of filesystem paths where tools are commonly installed beyond the default `/usr/bin/<name>`.

**Structure**: `map[string][]string`

**Example entries**:
- `"cargo"` -> `["/sandbox/.cargo/bin/cargo", "/sandbox/.rustup/toolchains/*/bin/cargo"]`
- `"go"` -> `["/usr/local/go/bin/go"]`
- `"claude"` -> `["/sandbox/.local/bin/claude", "/sandbox/.local/share/claude/**", "/usr/local/bin/claude"]`

### Resolved Binary Set

The output of the resolution process for a single policy component. Contains deduplicated filesystem paths.

**Fields**:
- Paths: Ordered list of unique binary path strings

### Resolution Input

The data needed to resolve binaries for one component.

**Fields**:
- Component match.tools: List of tool names from the policy component
- Manifest tools: List of ToolEntry records (name, install method, install_path)

## Relationships

```
PolicyComponent (matched)
  └── match.Tools[]
       └── for each tool name:
            ├── Manifest.Tools lookup
            │    ├── install: "package" → /usr/bin/<name>
            │    └── install: "github-release" → InstallPath
            └── Well-Known Paths Table lookup
                 └── additional paths[]
            ↓
       Deduplicated → component.Binaries[]
```

## Resolution Flow

```
Component has binaries? ─── Yes → Keep as-is (explicit override)
         │ No
         ↓
For each tool in match.Tools:
  ├── Look up in manifest.Tools
  │    ├── Found, install=package → add /usr/bin/<name>
  │    ├── Found, install=github-release → add InstallPath
  │    └── Not found → skip
  └── Look up in well-known paths table
       ├── Found → add all paths
       └── Not found → skip
         ↓
Deduplicate all collected paths
         ↓
Set component.Binaries = resolved paths
```
