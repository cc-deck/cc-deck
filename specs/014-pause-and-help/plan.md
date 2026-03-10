# Implementation Plan: Session Pause Mode & Keyboard Help

**Branch**: `014-pause-and-help` | **Date**: 2026-03-10 | **Spec**: [spec.md](spec.md)

## Summary

Add a per-session `paused` boolean flag toggled with `p` in navigation mode. Paused sessions show ⏸ icon with dimmed name and are excluded from attend cycling. Add `?` key for a help overlay showing all keyboard shortcuts.

## Technical Context

**Language/Version**: Rust stable (edition 2021, wasm32-wasip1 target)
**Primary Dependencies**: zellij-tile 0.43.1, serde/serde_json 1.x
**Testing**: `cargo test` (native target)
**Target Platform**: WASM (wasm32-wasip1)
**Project Type**: Zellij WASM plugin

## Constitution Check

No violations. Changes are within the existing plugin component, no new dependencies.

## Project Structure

```text
cc-zellij-plugin/src/
├── session.rs       # Add `paused: bool` field to Session
├── attend.rs        # Filter out paused sessions from candidates
├── main.rs          # p key handler, ? key handler, show_help state
├── sidebar.rs       # Paused rendering (⏸ icon, dimmed name), help overlay
├── state.rs         # Add show_help: bool field
└── (other files unchanged)
```
