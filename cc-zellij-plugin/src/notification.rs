// T026: Inline notification display in sidebar

use crate::session::unix_now;
use crate::state::Notification;

/// Create a notification that expires after `duration_secs` seconds.
pub fn create_notification(message: &str, duration_secs: u64) -> Notification {
    Notification {
        message: message.to_string(),
        expires_at_ms: unix_now() + duration_secs,
    }
}

/// Check if a notification has expired.
pub fn is_expired(notif: &Notification) -> bool {
    unix_now() >= notif.expires_at_ms
}

/// Render a notification on the given row, dimmed.
pub fn render_notification(notif: &Notification, row: usize, cols: usize) {
    let msg = &notif.message;
    let truncated = if msg.len() > cols {
        &msg[..cols]
    } else {
        msg
    };
    let padding = if truncated.len() < cols {
        " ".repeat(cols - truncated.len())
    } else {
        String::new()
    };
    print!(
        "\x1b[{};1H\x1b[2m{truncated}{padding}\x1b[0m",
        row + 1
    );
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_create_notification() {
        let notif = create_notification("hello", 5);
        assert_eq!(notif.message, "hello");
        // expires_at_ms should be roughly now + 5
        let now = unix_now();
        assert!(notif.expires_at_ms >= now);
        assert!(notif.expires_at_ms <= now + 6);
    }

    #[test]
    fn test_is_expired_fresh() {
        let notif = create_notification("test", 60);
        assert!(!is_expired(&notif));
    }

    #[test]
    fn test_is_expired_past() {
        let notif = Notification {
            message: "old".into(),
            expires_at_ms: 0,
        };
        assert!(is_expired(&notif));
    }

    #[test]
    fn test_render_notification_does_not_panic() {
        let notif = create_notification("Attending: my-session", 3);
        // Just verify it doesn't panic
        render_notification(&notif, 5, 20);
    }
}
