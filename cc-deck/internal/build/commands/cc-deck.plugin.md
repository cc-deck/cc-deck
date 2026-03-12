---
description: Add Claude Code plugins to the cc-deck build manifest
---

## User Input

$ARGUMENTS

## Outline

Add Claude Code plugins to `cc-deck-build.yaml`. Plugins can come from marketplaces or direct git URLs.

### Step 1: Show current plugins

Read `cc-deck-build.yaml` and display the current plugins section. If empty, say so.

### Step 2: Get plugin selection

If the user provided plugins in the input, use those. Otherwise ask:

"Which plugins would you like to add? You can specify:
- A marketplace name (e.g., 'sdd', 'cc-rosa')
- A git URL (e.g., 'git:https://github.com/org/plugin.git')
- 'list' to see available marketplace plugins"

### Step 3: Validate and add

For each plugin:
- If marketplace: set source to "marketplace"
- If git URL: set source to the URL (prefixed with "git:")
- Check for duplicates (same name already in manifest)

### Step 4: Update the manifest

Add the new plugin entries to the `plugins` section of `cc-deck-build.yaml`.

### Key Rules

- Don't add duplicates (warn if plugin name already exists)
- Validate git URLs are well-formed
- Show the updated plugins list after changes
