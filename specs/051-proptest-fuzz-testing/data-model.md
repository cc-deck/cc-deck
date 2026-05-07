# Data Model: Property-Based Fuzz Testing

## Entities

### FuzzAction (test-only enum)

Represents any user input that can be applied to the sidebar state machine.

| Variant | Maps To | Category |
|---------|---------|----------|
| KeyJ | `handle_key(bare(Char('j')))` | Navigation |
| KeyK | `handle_key(bare(Char('k')))` | Navigation |
| KeyEnter | `handle_key(bare(Enter))` | Confirm |
| KeyEsc | `handle_key(bare(Esc))` | Cancel |
| KeyD | `handle_key(bare(Char('d')))` | Delete |
| KeyR | `handle_key(bare(Char('r')))` | Rename |
| KeyP | `handle_key(bare(Char('p')))` | Pause |
| KeySlash | `handle_key(bare(Char('/')))` | Filter |
| KeyQuestion | `handle_key(bare(Char('?')))` | Help |
| KeyM | `handle_key(bare(Char('m')))` | Mute |
| KeyN | `handle_key(bare(Char('n')))` | New session |
| KeyBigR | `handle_key(bare(Char('R')))` | Refresh |
| KeyY | `handle_key(bare(Char('y')))` | Delete confirm |
| KeyBigY | `handle_key(bare(Char('Y')))` | Delete confirm |
| KeyBackspace | `handle_key(bare(Backspace))` | Edit |
| ArbitraryChar(char) | `handle_key(bare(Char(c)))` | Text input |
| ToggleNavigate | `toggle_navigate(state)` | Mode entry |
| ToggleNavigatePrev | `toggle_navigate_prev(state)` | Mode entry |
| LeftClick(usize) | `handle_mouse(LeftClick(row, 0))` | Mouse |
| RightClick(usize) | `handle_mouse(RightClick(row, 0))` | Mouse |
| AddSession | Append to cached_payload | Mutation |
| RemoveSession | Remove from cached_payload | Mutation |

### Invariant (verification contract)

| ID | Name | Condition | Applies When |
|----|------|-----------|--------------|
| INV-1 | Cursor in bounds | `cursor_index < max(1, filtered_sessions.len())` | Any navigation sub-mode |
| INV-2 | Filter state consistency | `filter_state().is_some()` | Mode is NavigateFilter |
| INV-3 | Passive filter clean | `filter_text.is_empty()` | Mode is Passive |
| INV-4 | Selectable matches mode | `is_selectable() == !matches!(mode, Passive)` | Always |
| INV-5 | Help consistency | `is_help()` implies inner mode is valid | Mode is Help |

## State Transitions Under Test

The 7 SidebarMode variants and their valid transitions:

```text
Passive ──→ Navigate (toggle_navigate/prev, header click)
Navigate ──→ Passive (Esc, Enter, click, 'n')
Navigate ──→ NavigateFilter ('/')
Navigate ──→ NavigateDeleteConfirm ('d')
Navigate ──→ NavigateRename ('r')
Navigate ──→ Help ('?', F1)
NavigateFilter ──→ Navigate (Esc, Enter)
NavigateDeleteConfirm ──→ Navigate (y/Y/n/N/Esc)
NavigateRename ──→ Navigate (Enter, Esc)
Passive ──→ RenamePassive (double-click, right-click)
RenamePassive ──→ Passive (Enter, Esc, click)
Help ──→ <previous mode> (any key)
```
