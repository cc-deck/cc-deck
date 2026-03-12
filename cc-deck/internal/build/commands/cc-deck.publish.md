---
description: Build and push the cc-deck container image (convenience wrapper)
---

## User Input

$ARGUMENTS

## Outline

Convenience command that runs `cc-deck build` followed by `cc-deck push`. Equivalent to running both commands separately but saves tokens by doing it in one step.

### Step 1: Build the image

Run `cc-deck build .` (assuming the build directory is the current directory).

If this fails, report the error and stop.

### Step 2: Push the image

Run `cc-deck push .`

If this fails, report the error. The image is still built locally even if push fails.

### Step 3: Report

Show the built image name:tag and the registry it was pushed to.
