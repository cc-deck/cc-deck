// WASM host function wrappers.
//
// Zellij SDK functions like subscribe(), request_permission(), set_timeout(),
// and set_selectable() link against WASM host functions that are unavailable
// on native targets. These wrappers provide no-op stubs for native builds,
// enabling `cargo test` to compile and run on the host platform.

use zellij_tile::prelude::*;

#[cfg(target_family = "wasm")]
pub fn subscribe_wasm(events: &[EventType]) {
    subscribe(events);
}

#[cfg(not(target_family = "wasm"))]
pub fn subscribe_wasm(_events: &[EventType]) {}

#[cfg(target_family = "wasm")]
pub fn request_permission_wasm(permissions: &[PermissionType]) {
    request_permission(permissions);
}

#[cfg(not(target_family = "wasm"))]
pub fn request_permission_wasm(_permissions: &[PermissionType]) {}

#[cfg(target_family = "wasm")]
pub fn set_timeout_wasm(interval: f64) {
    set_timeout(interval);
}

#[cfg(not(target_family = "wasm"))]
pub fn set_timeout_wasm(_interval: f64) {}

#[cfg(target_family = "wasm")]
pub fn set_selectable_wasm(selectable: bool) {
    set_selectable(selectable);
}

#[cfg(not(target_family = "wasm"))]
pub fn set_selectable_wasm(_selectable: bool) {}

#[cfg(target_family = "wasm")]
pub fn rename_tab_wasm(tab_idx: usize, name: &str) {
    rename_tab(tab_idx as u32 + 1, name);
}

#[cfg(not(target_family = "wasm"))]
pub fn rename_tab_wasm(_tab_idx: usize, _name: &str) {}
