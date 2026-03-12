---
description: Analyze repositories to discover tool dependencies for the cc-deck build manifest
---

## User Input

$ARGUMENTS

## Outline

Analyze locally checked-out repositories to discover build tools, compilers, and runtime dependencies. Update the `cc-deck-build.yaml` manifest with findings.

### Step 1: Identify repositories

If the user provided paths in the input, use those. Otherwise ask:

"Which repositories should I analyze? Provide paths to local checkouts (one per line or comma-separated)."

Validate that each path exists and contains source code.

### Step 2: Analyze each repository

For each repository, examine these files (if present):

**Build files**: `go.mod`, `package.json`, `Cargo.toml`, `pyproject.toml`, `Gemfile`, `pom.xml`, `build.gradle`, `Makefile`, `CMakeLists.txt`

**CI configs**: `.github/workflows/*.yml`, `.gitlab-ci.yml`, `Jenkinsfile`, `.circleci/config.yml`

**Tool version files**: `.tool-versions`, `.nvmrc`, `.python-version`, `.sdkmanrc`, `.go-version`, `.ruby-version`, `rust-toolchain.toml`

**Container files**: `Dockerfile`, `Containerfile` (for system package hints)

For each file found, extract:
- Programming language and version requirements
- Build tools and their versions
- System-level dependencies (compilers, libraries)
- Runtime requirements

### Step 3: Deduplicate and resolve conflicts

- Merge findings across all repositories
- Deduplicate identical tools
- For version conflicts (e.g., Go 1.22 vs Go 1.23), suggest the highest compatible version
- Group tools by category (compilers, package managers, system tools)

### Step 4: Present findings for review

Show the user a summary table:

```
Tool                  | Version    | Source
---------------------|------------|------------------
Go compiler          | >= 1.23    | repo1/go.mod
Python               | >= 3.12    | repo2/pyproject.toml
protoc               | >= 25.0    | repo1/.github/workflows/ci.yml
```

Ask: "Which tools should I add to the manifest? You can accept all, reject specific ones, or modify versions."

### Step 5: Update the manifest

Read the current `cc-deck-build.yaml`. Update the `tools` section with accepted entries (as free-form text). Update the `sources` section with repository provenance (URL, ref, path, detected_tools, detected_from).

Write the updated manifest. Use `yq` if available for safe YAML updates, otherwise write the full file.

### Key Rules

- Never modify tools the user explicitly rejected
- Keep tool descriptions human-readable (e.g., "Go compiler >= 1.23", not "golang-1.23.4")
- Record which files each tool was detected from (for provenance)
- If re-running on an already-analyzed repo, update existing entries and highlight changes
