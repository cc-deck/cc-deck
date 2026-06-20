# Feature Specification: OpenShell SSH-to-HTTPS Git Clone Conversion

**Feature Branch**: `072-openshell-ssh-to-https`
**Created**: 2026-06-20
**Status**: Draft
**Input**: Brainstorm 073 - convert SSH git URLs to HTTPS for OpenShell sandboxes

## User Scenarios & Testing *(mandatory)*

### User Story 1 - SSH URLs Auto-Convert on Workspace Creation (Priority: P1)

A developer runs `cc-deck ws new dev --type openshell --repo git@github.com:org/repo.git`. The SSH URL is automatically converted to `https://github.com/org/repo.git` before cloning inside the sandbox. The clone succeeds because HTTPS goes through the OpenShell HTTP CONNECT proxy where DNS resolution works. The user sees a log message about the conversion.

**Why this priority**: Without this fix, every OpenShell workspace creation with SSH URLs fails silently, leaving the workspace without source code. This is the primary user-facing failure.

**Independent Test**: Run `cc-deck ws new` with an SSH URL targeting an OpenShell sandbox. Verify the clone succeeds and the repo is present in the workspace.

**Acceptance Scenarios**:

1. **Given** an OpenShell workspace and `--repo git@github.com:org/repo.git`, **When** the workspace is created, **Then** the URL is converted to `https://github.com/org/repo.git` and the clone succeeds
2. **Given** an OpenShell workspace and `--repo git@gitlab.com:org/repo.git`, **When** the workspace is created, **Then** the URL is converted to `https://gitlab.com/org/repo.git` and the clone succeeds
3. **Given** an OpenShell workspace and `--repo git@bitbucket.org:org/repo.git`, **When** the workspace is created, **Then** the URL is converted to `https://bitbucket.org/org/repo.git` and the clone succeeds
4. **Given** an OpenShell workspace and `--repo https://github.com/org/repo.git`, **When** the workspace is created, **Then** the URL is used as-is (no conversion needed)
5. **Given** a non-OpenShell workspace (container or SSH) and `--repo git@github.com:org/repo.git`, **When** the workspace is created, **Then** the SSH URL is used as-is (conversion only applies to OpenShell)
6. **Given** an OpenShell workspace with a private repo and git token credentials configured, **When** the SSH URL is converted to HTTPS, **Then** the token is injected into the HTTPS URL for authentication

---

### User Story 2 - Git InsteadOf Config in OpenShell Images (Priority: P2)

Inside a running OpenShell sandbox, all git operations (submodule init, go get, cargo build with git deps) that use SSH URLs are transparently redirected to HTTPS via git's `insteadOf` config. This covers git operations beyond the initial `ws new` clone.

**Why this priority**: The initial clone (US1) is the most visible failure. But developers also run `git submodule update`, `go get`, and other commands inside the sandbox that may reference SSH URLs. Without `insteadOf`, those fail too.

**Independent Test**: Inside an OpenShell sandbox built with the updated Containerfile, run `git config --get-regexp url` and verify `insteadOf` entries exist for github.com, gitlab.com, and bitbucket.org. Then clone a repo using an SSH URL and verify it is transparently redirected to HTTPS.

**Acceptance Scenarios**:

1. **Given** an OpenShell sandbox, **When** `git config --get-regexp url` is run, **Then** insteadOf entries map `git@github.com:` to `https://github.com/`, `git@gitlab.com:` to `https://gitlab.com/`, and `git@bitbucket.org:` to `https://bitbucket.org/`
2. **Given** an OpenShell sandbox with insteadOf config, **When** `git clone git@github.com:org/repo.git` is run by the user, **Then** git transparently uses HTTPS and the clone succeeds

---

### Edge Cases

- What happens when the SSH URL uses a custom host (e.g., `git@git.company.com:org/repo.git`)? The conversion should still work using the general pattern: `git@<host>:<path>` becomes `https://<host>/<path>`.
- What happens when the SSH URL has a port (e.g., `ssh://git@github.com:2222/org/repo.git`)? The conversion should handle the `ssh://` scheme by extracting host and path.
- What happens when token credentials are not available for a private repo? The HTTPS clone fails with a 401. cc-deck should warn that the repo may need authentication.

## Requirements *(mandatory)*

### Functional Requirements

**Go Code (ws/repos.go)**

- **FR-001**: `buildCloneCommand()` MUST convert SSH URLs to HTTPS when the workspace type is OpenShell
- **FR-002**: The conversion MUST handle `git@<host>:<path>.git` format by transforming to `https://<host>/<path>.git`
- **FR-003**: The conversion MUST handle `ssh://git@<host>/<path>.git` format
- **FR-004**: HTTPS URLs MUST be passed through unchanged (no double-conversion)
- **FR-005**: Non-OpenShell workspace types (container, SSH) MUST NOT convert URLs
- **FR-006**: When a URL is converted, a log message MUST be emitted: `"Converting SSH URL to HTTPS for OpenShell sandbox: <old> -> <new>"`
- **FR-007**: If git token credentials are available, they MUST be injected into the converted HTTPS URL (existing `injectToken()` function)

**Build Skill (cc-deck.build.md)**

- **FR-008**: Section C2 MUST include `git config --global` commands to set `insteadOf` for github.com, gitlab.com, and bitbucket.org SSH-to-HTTPS mappings in the OpenShell Containerfile

**Tests**

- **FR-009**: Unit tests MUST cover SSH-to-HTTPS conversion for github.com, gitlab.com, bitbucket.org, and custom hosts
- **FR-010**: Unit tests MUST verify that HTTPS URLs pass through unchanged
- **FR-011**: Unit tests MUST verify that non-OpenShell workspaces do not trigger conversion

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: `cc-deck ws new --type openshell --repo git@github.com:org/repo.git` clones successfully (previously failed with DNS resolution error)
- **SC-002**: Inside an OpenShell sandbox, `git clone git@github.com:org/repo.git` succeeds via insteadOf redirect
- **SC-003**: All unit tests for SSH-to-HTTPS conversion pass
- **SC-004**: Non-OpenShell workspace types continue to use SSH URLs unchanged (no regression)

## Assumptions

- OpenShell's HTTP CONNECT proxy will continue to handle HTTPS traffic (port 443) with DNS resolution inside the tunnel
- The `git@<host>:<path>` format is the standard SSH URL pattern used by GitHub, GitLab, and Bitbucket
- The `insteadOf` git config is the standard Git mechanism for URL rewriting and is available in all supported Git versions
- Token-based HTTPS auth is sufficient for private repo access in OpenShell sandboxes (SSH key auth is not available through the proxy)
- Container builds (Section A) do not need `insteadOf` config because they have unrestricted network access
