/// Parsed pipe message from Claude Code hooks.
#[derive(Debug, Clone, PartialEq)]
pub struct PipeEvent {
    /// The event type (working, waiting, done)
    pub event_type: PipeEventType,
    /// The Zellij pane ID this event relates to
    pub pane_id: u32,
}

/// Types of events received via pipe messages.
#[derive(Debug, Clone, PartialEq)]
pub enum PipeEventType {
    Working,
    Waiting,
    Done,
}

const PIPE_PREFIX: &str = "cc-deck";

/// Parse a pipe message name in the format `cc-deck::EVENT_TYPE::PANE_ID`.
///
/// Returns `None` for malformed messages (silently ignored per spec).
pub fn parse_pipe_message(message: &str) -> Option<PipeEvent> {
    let parts: Vec<&str> = message.split("::").collect();
    if parts.len() != 3 {
        return None;
    }

    if parts[0] != PIPE_PREFIX {
        return None;
    }

    let event_type = match parts[1].to_lowercase().as_str() {
        "working" => PipeEventType::Working,
        "waiting" => PipeEventType::Waiting,
        "done" => PipeEventType::Done,
        _ => return None, // Unknown event types silently ignored
    };

    let pane_id = parts[2].parse::<u32>().ok()?;

    Some(PipeEvent {
        event_type,
        pane_id,
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_parse_working() {
        let result = parse_pipe_message("cc-deck::working::42");
        assert_eq!(
            result,
            Some(PipeEvent {
                event_type: PipeEventType::Working,
                pane_id: 42,
            })
        );
    }

    #[test]
    fn test_parse_waiting() {
        let result = parse_pipe_message("cc-deck::waiting::7");
        assert_eq!(
            result,
            Some(PipeEvent {
                event_type: PipeEventType::Waiting,
                pane_id: 7,
            })
        );
    }

    #[test]
    fn test_parse_done() {
        let result = parse_pipe_message("cc-deck::done::100");
        assert_eq!(
            result,
            Some(PipeEvent {
                event_type: PipeEventType::Done,
                pane_id: 100,
            })
        );
    }

    #[test]
    fn test_parse_case_insensitive() {
        let result = parse_pipe_message("cc-deck::WORKING::42");
        assert_eq!(
            result,
            Some(PipeEvent {
                event_type: PipeEventType::Working,
                pane_id: 42,
            })
        );

        let result = parse_pipe_message("cc-deck::Done::5");
        assert_eq!(
            result,
            Some(PipeEvent {
                event_type: PipeEventType::Done,
                pane_id: 5,
            })
        );
    }

    #[test]
    fn test_parse_wrong_prefix() {
        assert_eq!(parse_pipe_message("other::working::42"), None);
    }

    #[test]
    fn test_parse_too_few_parts() {
        assert_eq!(parse_pipe_message("cc-deck::working"), None);
    }

    #[test]
    fn test_parse_too_many_parts() {
        assert_eq!(parse_pipe_message("cc-deck::working::42::extra"), None);
    }

    #[test]
    fn test_parse_invalid_pane_id() {
        assert_eq!(parse_pipe_message("cc-deck::working::abc"), None);
    }

    #[test]
    fn test_parse_unknown_event_type() {
        assert_eq!(parse_pipe_message("cc-deck::unknown_event::42"), None);
    }

    #[test]
    fn test_parse_empty_string() {
        assert_eq!(parse_pipe_message(""), None);
    }

    #[test]
    fn test_parse_negative_pane_id() {
        assert_eq!(parse_pipe_message("cc-deck::working::-1"), None);
    }
}
