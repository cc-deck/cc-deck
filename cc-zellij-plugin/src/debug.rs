/// Debug logging is opt-in: only active when `/cache/debug_enabled` exists.
/// Enable:  `touch /cache/debug_enabled`  (inside the WASI sandbox)
/// Or from the host: `touch ~/.config/zellij/plugins/cc_deck.wasm/cache/debug_enabled`
/// Disable: remove the file (or just delete debug.log).
/// The flag is checked once on load and cached for the instance lifetime.
#[allow(dead_code)]
static mut DEBUG_ENABLED: bool = false;

#[cfg(target_family = "wasm")]
pub fn debug_init() {
    let enabled = std::fs::metadata("/cache/debug_enabled").is_ok();
    unsafe {
        DEBUG_ENABLED = enabled;
    }
    let _ = std::fs::write("/cache/debug.log", b"");
}

#[cfg(not(target_family = "wasm"))]
pub fn debug_init() {}

#[cfg(target_family = "wasm")]
pub fn debug_log(msg: &str) {
    if unsafe { !DEBUG_ENABLED } {
        return;
    }
    if let Ok(mut f) = std::fs::OpenOptions::new()
        .create(true)
        .append(true)
        .open("/cache/debug.log")
    {
        use std::io::Write;
        let ts = crate::session::unix_now_ms();
        let secs = ts / 1000;
        let millis = ts % 1000;
        let _ = writeln!(f, "[{secs}.{millis:03}] {msg}");
    }
}

#[cfg(not(target_family = "wasm"))]
pub fn debug_log(_msg: &str) {}

#[cfg(target_family = "wasm")]
pub fn install_panic_hook() {
    std::panic::set_hook(Box::new(|info| {
        let msg = if let Some(s) = info.payload().downcast_ref::<&str>() {
            s.to_string()
        } else if let Some(s) = info.payload().downcast_ref::<String>() {
            s.clone()
        } else {
            "unknown panic payload".to_string()
        };
        let location = info.location().map(|l| format!("{}:{}:{}", l.file(), l.line(), l.column())).unwrap_or_default();
        if let Ok(mut f) = std::fs::OpenOptions::new()
            .create(true)
            .append(true)
            .open("/cache/panic.log")
        {
            use std::io::Write;
            let _ = writeln!(f, "PANIC at {location}: {msg}");
        }
    }));
}

#[cfg(not(target_family = "wasm"))]
pub fn install_panic_hook() {}
