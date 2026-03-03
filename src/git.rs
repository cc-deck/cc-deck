use std::path::Path;

/// Trigger asynchronous git repo detection for a session.
///
/// Calls `git rev-parse --show-toplevel` via Zellij's `run_command` host function.
/// The result arrives later as a `RunCommandResult` event in `update()`.
///
/// This function is only available on WASM targets (requires Zellij host).
#[cfg(target_arch = "wasm32")]
pub fn detect_git_repo(session_id: u32) {
    use std::collections::BTreeMap;
    use zellij_tile::prelude::*;
    let context = BTreeMap::from([
        ("session_id".to_string(), session_id.to_string()),
        ("type".to_string(), "git_detect".to_string()),
    ]);
    run_command(&["git", "rev-parse", "--show-toplevel"], context);
}

/// Extract the repository name from a successful git detection result.
///
/// Parses the stdout from `git rev-parse --show-toplevel` and returns the
/// final path component (the repo directory name).
pub fn repo_name_from_stdout(stdout: &[u8]) -> Option<String> {
    let path_str = String::from_utf8_lossy(stdout).trim().to_string();
    if path_str.is_empty() {
        return None;
    }
    Path::new(&path_str)
        .file_name()
        .map(|n| n.to_string_lossy().to_string())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_repo_name_from_stdout() {
        let stdout = b"/home/user/projects/api-server\n";
        assert_eq!(
            repo_name_from_stdout(stdout),
            Some("api-server".to_string())
        );
    }

    #[test]
    fn test_repo_name_from_stdout_no_trailing_newline() {
        let stdout = b"/home/user/my-repo";
        assert_eq!(
            repo_name_from_stdout(stdout),
            Some("my-repo".to_string())
        );
    }

    #[test]
    fn test_repo_name_from_stdout_empty() {
        assert_eq!(repo_name_from_stdout(b""), None);
    }

    #[test]
    fn test_repo_name_from_stdout_whitespace_only() {
        assert_eq!(repo_name_from_stdout(b"  \n"), None);
    }

    #[test]
    fn test_repo_name_from_stdout_root_path() {
        // Edge case: repo at filesystem root (unlikely but handled)
        let stdout = b"/\n";
        // Path::new("/").file_name() returns None
        assert_eq!(repo_name_from_stdout(stdout), None);
    }
}
