# Tasks: Credential Transport Abstraction

**Branch**: `079-credential-transport` | **Generated**: 2026-07-08

## Dependencies

```
Task 1 (OpenShell migration) ──┐
                                ├──> Task 3 (Cleanup & verify)
Task 2 (SSH migration) ────────┘
```

Tasks 1 and 2 are independent and can be executed in parallel [P].

## Task List

- [X] **Task 1: Migrate OpenShell credential resolution to shared credential package** [P]

  **Files**: `cc-deck/internal/ws/openshell.go`, `cc-deck/internal/openshell/credentials.go`, `cc-deck/internal/openshell/credentials_test.go`

  **Interfaces consumed**:
  - `agent.Get(name string) Agent` returning an `Agent` with `CredentialSpecs() []agent.CredentialSpec`
  - `credential.Detect(specs []agent.CredentialSpec) []AvailableMode`
  - `credential.Resolve(spec agent.CredentialSpec) ResolvedCredentials`
  - `credential.InjectOpenShell(ctx context.Context, client credential.OpenShellClient, sandboxID string, resolved credential.ResolvedCredentials) error`
  - `credential.OpenShellClient` interface: `ExecRun(ctx, sandboxID, cmd []string) error` + `FileUpload(ctx, sandboxID, localPath, remotePath string) error`

  **What to do**:

  1. In `openshell/credentials.go`, add an `OpenShellClientAdapter` struct that wraps `v1.ClientInterface` and satisfies `credential.OpenShellClient`:
     ```go
     type OpenShellClientAdapter struct {
         client v1.ClientInterface
     }
     func (a *OpenShellClientAdapter) ExecRun(ctx context.Context, sandboxID string, cmd []string) error {
         _, err := a.client.Exec().Run(ctx, sandboxID, cmd)
         return err
     }
     func (a *OpenShellClientAdapter) FileUpload(ctx context.Context, sandboxID, localPath, remotePath string) error {
         return a.client.Files().Upload(ctx, sandboxID, localPath, remotePath)
     }
     ```

  2. In `ws/openshell.go`, replace the credential resolution block (lines 308-337):
     - Get the workspace's agent via `agent.Get(agentName)` (default "claude" if not set)
     - Call `credential.Detect(agent.CredentialSpecs())` to find available modes (returns `[]AvailableMode`)
     - Select mode by workspace `Auth` field or auto-select by priority
     - Call `credential.Resolve(selectedSpec)` to get `ResolvedCredentials`
     - Map `ResolvedCredentials` to OpenShell SDK `Provider` type for provider creation via `Providers().Ensure()`. For the `claude-vertex` special case: when the spec name is "vertex", use OpenShell provider type `google-cloud` and map `ANTHROPIC_VERTEX_PROJECT_ID` -> `project_id` and `CLOUD_ML_REGION` -> `region` in the provider config.

  3. Replace the post-start injection block (lines 373-386) with a single call:
     ```go
     adapter := &openshell.OpenShellClientAdapter{Client: w.client}
     if err := credential.InjectOpenShell(ctx, adapter, w.sandboxID, resolved); err != nil {
         log.Printf("WARNING: failed to inject credentials: %v", err)
     }
     ```
     This replaces both `openshell.UploadFileCredential()` and `openshell.InjectEnvVars()`.

  4. In `openshell/credentials.go`, remove all deprecated functions and types:
     - Remove `KnownProviderProfiles` map
     - Remove `ResolveDefaultEnvVars()` function
     - Remove `ResolveCredentials()` function
     - Remove `DetectCredentials()` function
     - Remove `InjectEnvVars()` function (replaced by `credential.InjectOpenShell()`)
     - Remove `UploadFileCredential()` function (replaced by `credential.InjectOpenShell()`)
     - Remove `CredentialInput`, `DetectedCredential`, `ProviderConfig`, `KnownProviderProfile` types
     - Keep `ProviderEndpoint` type if still referenced by provider creation code

  5. Update `openshell/credentials_test.go`: Remove tests for all deleted functions. Add tests for `OpenShellClientAdapter` verifying it correctly delegates to `v1.ClientInterface`.

  **Acceptance**: `make verify` passes. No references to `KnownProviderProfiles`, `ResolveCredentials`, `DetectCredentials`, `ResolveDefaultEnvVars`, `InjectEnvVars`, or `UploadFileCredential` remain in production code.

---

- [X] **Task 2: Migrate SSH credential resolution to shared credential package** [P]

  **Files**: `cc-deck/internal/ws/ssh.go`, `cc-deck/internal/cmd/ws.go`, `cc-deck/internal/ssh/credentials.go`, `cc-deck/internal/ssh/credentials_test.go`

  **Interfaces consumed**:
  - `credential.DetectAll() []DetectedMode`
  - `credential.MergeCredentials(modes []DetectedMode) (ResolvedCredentials, error)`
  - `credential.InjectSSH(ctx context.Context, client credential.SSHClient, resolved ResolvedCredentials) error`
  - `credential.SSHClient` interface: `Run(ctx, cmd string) (string, error)` + `Upload(ctx, localPath, remotePath string) error`
  - Note: `*ssh.Client` already satisfies `credential.SSHClient`

  **What to do**:

  1. In `ws/ssh.go` (lines 190-212), remove the `else` fallback branch (lines 202-211) that calls `ssh.BuildCredentialSet()` + `ssh.WriteCredentialFile()`. The primary path (lines 194-201) already uses `credential.DetectAll()` + `credential.MergeCredentials()` + `credential.InjectSSH()` and handles all credential resolution correctly. After removal, the flow is:
     ```go
     modes := credential.DetectAll()
     if len(modes) > 0 {
         merged, mergeErr := credential.MergeCredentials(modes)
         if mergeErr != nil {
             log.Printf("WARNING: could not merge credentials: %v", mergeErr)
         } else if writeErr := credential.InjectSSH(ctx, client, merged); writeErr != nil {
             log.Printf("WARNING: could not write credentials to remote: %v", writeErr)
         }
     }
     ```

  2. In `cmd/ws.go` (around line 1821), replace `sshPkg.BuildCredentialSet()` + `sshPkg.WriteCredentialFile()` with:
     ```go
     modes := credential.DetectAll()
     if len(modes) == 0 {
         fmt.Fprintf(os.Stdout, "No credentials found to refresh for workspace %q\n", name)
         return nil
     }
     merged, mergeErr := credential.MergeCredentials(modes)
     if mergeErr != nil {
         return fmt.Errorf("merging credentials: %w", mergeErr)
     }
     if err := credential.InjectSSH(cmd_context(), client, merged); err != nil {
         return fmt.Errorf("writing credentials: %w", err)
     }
     ```

  3. In `ssh/credentials.go`, remove all deprecated functions:
     - Remove `BuildCredentialSet()` function
     - Remove `detectAuthMode()` function
     - Remove `WriteCredentialFile()` function (replaced by `credential.InjectSSH()`)
     - Remove `CopyCredentialFile()` function (replaced by `credential.InjectSSH()`)

  4. Update `ssh/credentials_test.go`: Remove all tests for deleted functions (`TestBuildCredentialSet_*`, `detectAuthMode` tests).

  **Acceptance**: `make verify` passes. No references to `BuildCredentialSet`, `detectAuthMode`, `WriteCredentialFile`, or `CopyCredentialFile` remain in production code.

---

- [X] **Task 3: Final cleanup and verification**

  **Depends on**: Task 1, Task 2

  **Files**: All modified files from Tasks 1 and 2

  **What to do**:

  1. Run `rg "KnownProviderProfiles\|BuildCredentialSet\|detectAuthMode\|ResolveDefaultEnvVars\|DetectCredentials\|WriteCredentialFile\|CopyCredentialFile\|InjectEnvVars\|UploadFileCredential" --type go` across the entire codebase. Confirm zero hits in production code (test files may reference them in migration comments).

  2. Check that no unused imports remain in the modified files (the lint step in `make verify` catches this, but verify explicitly).

  3. Run `make verify` for the final pass (tests + lint).

  4. Review each modified file to confirm:
     - OpenShell workspace creation with API key, Vertex, and Bedrock credentials produces the same provider configs as before
     - SSH workspace creation resolves the same credential sets as before
     - The `claude-vertex` special case (google-cloud provider type with project_id/region) is preserved
     - File credential upload (GOOGLE_APPLICATION_CREDENTIALS) works for both OpenShell and SSH paths

  5. Verify SC-004: confirm the `internal/credential` package has test coverage for detect, resolve, merge, validate, and all transport operations (`InjectContainer`, `InjectSSH`, `InjectOpenShell`, `InjectK8s`), with at least one test per agent auth mode. The existing tests should already cover this; verify they exist and pass.

  **Acceptance**: `make verify` passes. `rg` confirms zero legacy references in production code. SC-001 through SC-004 from the spec are met.
