# Quickstart: cc-deck Documentation & Landing Page

## Development Setup

### Landing Page (cc-deck.github.io)

```bash
# Clone the site repo
git clone https://github.com/rhuss/cc-deck.github.io
cd cc-deck.github.io

# Install dependencies
npm install

# Start dev server (landing page only)
npm run dev

# Build everything (landing page + Antora docs)
npm run build

# Preview built site
npm run preview
```

### Documentation (Antora in cc-deck repo)

```bash
# From the cc-deck repo root
cd docs

# Preview docs locally (requires Antora CLI)
npx antora antora-playbook.yml

# Or build from the .github.io repo (pulls from local)
cd ../cc-deck.github.io
npm run build:docs
```

### Demo Image

```bash
# From the cc-deck repo root
make demo-image

# Test locally
podman run -d --name cc-demo \
  -e ANTHROPIC_API_KEY=sk-ant-... \
  quay.io/rhuss/cc-deck-demo:latest

podman exec -it cc-demo zellij --layout cc-deck
```

## Content Workflow

1. Write AsciiDoc pages in `cc-deck/docs/modules/<module>/pages/`
2. Update `nav.adoc` in the module to include new pages
3. Preview locally with Antora
4. Commit to cc-deck main branch
5. Trigger rebuild of cc-deck.github.io (manual dispatch or auto)

## Deployment

GitHub Pages serves `cc-deck.github.io`:
- `/` serves the Astro landing page
- `/docs/` serves the Antora documentation
- Both are built by a GitHub Actions workflow
