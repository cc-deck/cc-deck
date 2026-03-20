# Quickstart: Network Filtering

## 1. Add network config to your manifest

Edit `cc-deck-build.yaml` in your build directory:

```yaml
network:
  allowed_domains:
    - github
    - python
    - golang
```

Or let `/cc-deck.extract` auto-detect project ecosystems and populate the list.

## 2. Generate compose files

```bash
cc-deck deploy --compose my-build-dir/ --output ./deploy
```

This generates:
- `deploy/compose.yaml` (session + proxy sidecar)
- `deploy/.env.example` (credential template)
- `deploy/proxy/` (proxy config + whitelist)

## 3. Run the session

```bash
cp deploy/.env.example deploy/.env
# Edit .env with your credentials
podman compose -f deploy/compose.yaml up -d
```

## 4. Verify filtering works

```bash
# Inside the session container:
curl https://pypi.org          # Allowed (python group)
curl https://evil-server.com   # Blocked by proxy
```

## 5. Debug blocked domains

```bash
cc-deck domains blocked my-session
# Shows which domains were blocked with timestamps
```

## 6. Add a missing domain at runtime

```bash
cc-deck domains add my-session custom.example.com
# Proxy reconfigured, no session restart needed
```

## Explore domain groups

```bash
cc-deck domains list              # See all available groups
cc-deck domains show python       # See what domains are in a group
cc-deck domains init              # Seed config file for customization
```

## Customize for your organization

Edit `~/.config/cc-deck/domains.yaml`:

```yaml
# Extend built-in python group with internal registry
python:
  extends: builtin
  domains:
    - pypi.internal.corp

# Create a custom group
company:
  domains:
    - artifacts.internal.corp
    - git.internal.corp
```

Then reference in your manifest: `allowed_domains: [python, company]`
