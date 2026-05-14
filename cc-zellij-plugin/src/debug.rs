/// Debug logging is opt-in: only active when `/cache/debug_enabled` exists.
/// Enable:  `touch /cache/debug_enabled`  (inside the WASI sandbox)
/// Or from the host: `touch ~/.config/zellij/plugins/cc_deck.wasm/cache/debug_enabled`
/// Disable: remove the file (or just delete debug.log).
/// The flag is checked once on load and cached for the instance lifetime.
///
/// Logging is buffered: lines accumulate in a thread_local buffer and are
/// flushed on timer tick (via debug_flush()) or when the buffer exceeds
/// 50 lines. WASI is single-threaded so thread_local has no overhead.
#[allow(dead_code)]
static mut DEBUG_ENABLED: bool = false;

#[cfg(target_family = "wasm")]
const BUFFER_CAPACITY: usize = 50;

#[cfg(target_family = "wasm")]
thread_local! {
    static LOG_BUFFER: std::cell::RefCell<Vec<String>> = std::cell::RefCell::new(Vec::new());
}

#[cfg(target_family = "wasm")]
pub fn debug_init() {
    let enabled = std::fs::metadata("/cache/debug_enabled").is_ok();
    unsafe {
        DEBUG_ENABLED = enabled;
    }
    // Do NOT truncate: multiple instances share this file, and truncating
    // from a later-loading instance erases earlier startup diagnostics.
    // Instead, append a separator on each load.
    if enabled {
        if let Ok(mut f) = std::fs::OpenOptions::new()
            .create(true)
            .append(true)
            .open("/cache/debug.log")
        {
            use std::io::Write;
            let ts = crate::session::unix_now_ms();
            let _ = writeln!(f, "--- instance load at {ts} ---");
        }
    }
}

#[cfg(not(target_family = "wasm"))]
pub fn debug_init() {}

#[cfg(target_family = "wasm")]
pub fn debug_log(msg: &str) {
    if unsafe { !DEBUG_ENABLED } {
        return;
    }
    let ts = crate::session::unix_now_ms();
    let secs = ts / 1000;
    let millis = ts % 1000;
    let line = format!("[{secs}.{millis:03}] {msg}");

    LOG_BUFFER.with(|buf| {
        let mut buf = buf.borrow_mut();
        buf.push(line);
        if buf.len() >= BUFFER_CAPACITY {
            flush_buffer(&buf);
            buf.clear();
        }
    });
}

#[cfg(target_family = "wasm")]
pub fn debug_flush() {
    if unsafe { !DEBUG_ENABLED } {
        return;
    }
    LOG_BUFFER.with(|buf| {
        let mut buf = buf.borrow_mut();
        if !buf.is_empty() {
            flush_buffer(&buf);
            buf.clear();
        }
    });
}

#[cfg(target_family = "wasm")]
fn flush_buffer(lines: &[String]) {
    if let Ok(mut f) = std::fs::OpenOptions::new()
        .create(true)
        .append(true)
        .open("/cache/debug.log")
    {
        use std::io::Write;
        for line in lines {
            let _ = writeln!(f, "{line}");
        }
    }
}

/// Write a log line immediately (unbuffered). Use for critical startup
/// diagnostics that must survive file truncation by later-loading instances.
#[cfg(target_family = "wasm")]
pub fn debug_log_immediate(msg: &str) {
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

#[cfg(not(target_family = "wasm"))]
pub fn debug_log_immediate(_msg: &str) {}

#[cfg(not(target_family = "wasm"))]
pub fn debug_flush() {}

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
