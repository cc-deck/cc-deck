// Library crate: exports modules for native testing via `cargo test`.
// The Zellij plugin entry point is in main.rs (binary target).

pub mod config;
pub mod git;
pub mod group;
pub mod picker;
pub mod pipe_handler;
pub mod recent;
pub mod session;
pub mod state;
pub mod status_bar;

// keybindings module uses Zellij host functions, only available on WASM
#[cfg(target_arch = "wasm32")]
pub mod keybindings;
