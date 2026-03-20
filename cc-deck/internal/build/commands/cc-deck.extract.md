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

**Build files**: `go.mod`, `package.json`, `Cargo.toml`, `pyproject.toml`, `Gemfile`, `pom.xml`, `build.gradle`, `build.gradle.kts`, `settings.gradle`, `Makefile`, `CMakeLists.txt`

**CI configs**: `.github/workflows/*.yml`, `.gitlab-ci.yml`, `Jenkinsfile`, `.circleci/config.yml`

**Tool version files**: `.tool-versions`, `.nvmrc`, `.python-version`, `.sdkmanrc`, `.go-version`, `.ruby-version`, `.java-version`, `rust-toolchain.toml`

**Java-specific**: `mvnw` / `gradlew` (detect Maven/Gradle version from wrapper properties), `jvm.config`, `.mvn/jvm.config` (JVM flags), `pom.xml` `<maven.compiler.source>` / `<maven.compiler.target>` / `<java.version>` properties

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

### Step 5: Detect network domain groups

Based on the ecosystem files found in Step 2, determine which network domain groups to add to the manifest's `network.allowed_domains` section:

| Ecosystem file | Domain group |
|----------------|-------------|
| `go.mod` | `golang` |
| `pyproject.toml`, `requirements.txt`, `.python-version` | `python` |
| `package.json`, `.nvmrc` | `nodejs` |
| `Cargo.toml`, `rust-toolchain.toml` | `rust` |

Always include `github` if a `.git` directory or `.github/` directory is found.

Present the detected domain groups to the user alongside the tool findings:

```
Network domain groups detected:
  golang   (from go.mod)
  python   (from pyproject.toml)
  github   (from .github/)
```

Ask: "Should I add these domain groups to the manifest's network.allowed_domains?"

### Step 6: Update the manifest

Read the current `cc-deck-build.yaml`. Update the `tools` section with accepted entries (as free-form text). Update the `sources` section with repository provenance (URL, ref, path, detected_tools, detected_from).

If the user accepted domain groups from Step 5, add or update the `network` section:

```yaml
network:
  allowed_domains:
    - golang
    - python
    - github
```

If a `network.allowed_domains` section already exists, merge newly detected groups with existing entries (do not remove existing groups).

Write the updated manifest. Use `yq` if available for safe YAML updates, otherwise write the full file.

### Key Rules

- Never modify tools the user explicitly rejected
- Keep tool descriptions human-readable (e.g., "Go compiler >= 1.23", not "golang-1.23.4")
- Record which files each tool was detected from (for provenance)
- If re-running on an already-analyzed repo, update existing entries and highlight changes
- **NEVER include container runtimes** (podman, docker, buildah, skopeo) as detected tools.
  These are host build tools, not image dependencies. The base image does not need them.
  If found in CI configs or Containerfiles, silently exclude them from the results.
