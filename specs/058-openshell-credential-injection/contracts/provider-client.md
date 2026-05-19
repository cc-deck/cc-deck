# Contract: OpenShell Provider Client

## Interface Extension

The `openshell.Client` interface gains three new methods for provider management.

### CreateProvider

```
CreateProvider(ctx, name, providerType string, fromExisting bool, credentials map[string]string) error
```

- Creates a new provider on the OpenShell gateway
- If `fromExisting` is true, uses `--from-existing` flag (reads from host env)
- If `fromExisting` is false, passes each credential as `--credential KEY=VALUE`
- Returns error if the gateway is unreachable or the provider type is invalid

### UpdateProvider

```
UpdateProvider(ctx, name, providerType string, fromExisting bool, credentials map[string]string) error
```

- Updates an existing provider's credentials
- Same parameter semantics as CreateProvider
- Returns error if provider does not exist

### DeleteProvider

```
DeleteProvider(ctx, name string) error
```

- Removes a provider from the gateway
- Returns nil if provider does not exist (idempotent)

### EnsureProvider (convenience)

```
EnsureProvider(ctx, name, providerType string, fromExisting bool, credentials map[string]string) error
```

- Tries CreateProvider first
- If provider already exists (detected by error message), falls back to UpdateProvider
- Implements FR-011 (idempotent provider creation)

## CLI Mapping

| Method | CLI Command |
|--------|-------------|
| CreateProvider | `openshell provider create --name <name> --type <type> [--from-existing \| --credential K=V...]` |
| UpdateProvider | `openshell provider update <name> --type <type> [--from-existing \| --credential K=V...]` |
| DeleteProvider | `openshell provider delete <name>` |
