# Quickstart: Tool PATH Restoration

## What this feature does

Ensures tool install paths (e.g., `/usr/local/go/bin`, `$HOME/.cargo/bin`) survive shell initialization in container builds by prepending them to shell rc files during image generation.

## Key files

| File | What to change |
|------|---------------|
| `cc-deck/internal/build/containerfile.go` | Add `ToolPaths` field, `toolPathRegistry` map, `ResolveToolPaths()` function, update `ContainerDataForTarget()` |
| `cc-deck/internal/build/containerfile_test.go` | Tests for registry resolution and deduplication |
| `cc-deck/internal/build/templates/containerfile/05-shell-finalize.tmpl` | Add PATH prepend RUN step |

## How to verify

```bash
make test    # All existing + new tests pass
make lint    # No linting errors
```

Then build an image and verify tools are on PATH:

```bash
cc-deck build refresh .cc-deck/setup
cc-deck build run .cc-deck/setup --target openshell
# Start a shell in the sandbox and run:
go version       # Should work
cargo --version  # Should work
```

## Example: registry entry

```go
var toolPathRegistry = map[string]string{
    "go":    "/usr/local/go/bin",
    "cargo": "{home}/.cargo/bin",
    "rust":  "{home}/.cargo/bin",
}
```

## Example: generated Containerfile output

```dockerfile
# Tool PATH restoration (auto-generated from manifest tools)
RUN for RC in /sandbox/.bashrc /sandbox/.zshrc; do \
      if [ -f "$RC" ]; then \
        sed -i '1i export PATH="/usr/local/go/bin:/sandbox/.cargo/bin:$PATH"' "$RC"; \
      fi; \
    done
```
