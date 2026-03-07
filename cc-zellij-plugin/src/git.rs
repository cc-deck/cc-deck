// T009: Async git repo and branch detection via run_command()

use std::collections::BTreeMap;

const GIT_CONTEXT_TYPE: &str = "type";
const GIT_CONTEXT_PANE: &str = "pane_id";
const GIT_TYPE_REPO: &str = "git_repo";
const GIT_TYPE_BRANCH: &str = "git_branch";

/// Launch async git repo detection for a pane's working directory.
#[cfg(target_family = "wasm")]
pub fn detect_git_repo(pane_id: u32, cwd: &str) {
    use zellij_tile::prelude::run_command;
    let mut ctx = BTreeMap::new();
    ctx.insert(GIT_CONTEXT_TYPE.into(), GIT_TYPE_REPO.into());
    ctx.insert(GIT_CONTEXT_PANE.into(), pane_id.to_string());
    run_command(&["git", "-C", cwd, "rev-parse", "--show-toplevel"], ctx);
}

#[cfg(not(target_family = "wasm"))]
pub fn detect_git_repo(_pane_id: u32, _cwd: &str) {}

/// Launch async git branch detection for a pane's working directory.
#[cfg(target_family = "wasm")]
pub fn detect_git_branch(pane_id: u32, cwd: &str) {
    use zellij_tile::prelude::run_command;
    let mut ctx = BTreeMap::new();
    ctx.insert(GIT_CONTEXT_TYPE.into(), GIT_TYPE_BRANCH.into());
    ctx.insert(GIT_CONTEXT_PANE.into(), pane_id.to_string());
    run_command(&["git", "-C", cwd, "rev-parse", "--abbrev-ref", "HEAD"], ctx);
}

#[cfg(not(target_family = "wasm"))]
pub fn detect_git_branch(_pane_id: u32, _cwd: &str) {}

/// Result of a git detection command.
pub enum GitResult {
    RepoDetected { pane_id: u32, repo_path: String },
    BranchDetected { pane_id: u32, branch: String },
    NotGit,
}

/// Parse a RunCommandResult into a GitResult.
pub fn parse_git_result(
    exit_code: Option<i32>,
    stdout: Vec<u8>,
    context: BTreeMap<String, String>,
) -> GitResult {
    if exit_code != Some(0) {
        return GitResult::NotGit;
    }

    let result_type = match context.get(GIT_CONTEXT_TYPE) {
        Some(t) => t.as_str(),
        None => return GitResult::NotGit,
    };

    let pane_id = match context.get(GIT_CONTEXT_PANE).and_then(|s| s.parse::<u32>().ok()) {
        Some(id) => id,
        None => return GitResult::NotGit,
    };

    let output = String::from_utf8_lossy(&stdout).trim().to_string();
    if output.is_empty() {
        return GitResult::NotGit;
    }

    match result_type {
        GIT_TYPE_REPO => GitResult::RepoDetected {
            pane_id,
            repo_path: output,
        },
        GIT_TYPE_BRANCH => {
            match filter_default_branch(&output) {
                Some(b) => GitResult::BranchDetected {
                    pane_id,
                    branch: b.to_string(),
                },
                None => GitResult::NotGit,
            }
        }
        _ => GitResult::NotGit,
    }
}

/// Extract repo name from a toplevel path (basename).
pub fn repo_name_from_path(path: &str) -> &str {
    path.rsplit('/').next().unwrap_or(path)
}

/// Filter out default branches (main, master, HEAD).
fn filter_default_branch(branch: &str) -> Option<&str> {
    match branch {
        "main" | "master" | "HEAD" | "" => None,
        b => Some(b),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_repo_name_from_path() {
        assert_eq!(repo_name_from_path("/home/user/projects/api-server"), "api-server");
        assert_eq!(repo_name_from_path("/tmp/test"), "test");
        assert_eq!(repo_name_from_path("simple"), "simple");
    }

    #[test]
    fn test_filter_default_branch() {
        assert_eq!(filter_default_branch("main"), None);
        assert_eq!(filter_default_branch("master"), None);
        assert_eq!(filter_default_branch("HEAD"), None);
        assert_eq!(filter_default_branch("feature/auth"), Some("feature/auth"));
        assert_eq!(filter_default_branch("fix-123"), Some("fix-123"));
    }

    #[test]
    fn test_parse_git_result_repo() {
        let mut ctx = BTreeMap::new();
        ctx.insert("type".into(), "git_repo".into());
        ctx.insert("pane_id".into(), "42".into());

        match parse_git_result(Some(0), b"/home/user/api-server\n".to_vec(), ctx) {
            GitResult::RepoDetected { pane_id, repo_path } => {
                assert_eq!(pane_id, 42);
                assert_eq!(repo_path, "/home/user/api-server");
            }
            _ => panic!("expected RepoDetected"),
        }
    }

    #[test]
    fn test_parse_git_result_branch() {
        let mut ctx = BTreeMap::new();
        ctx.insert("type".into(), "git_branch".into());
        ctx.insert("pane_id".into(), "42".into());

        match parse_git_result(Some(0), b"feature/auth\n".to_vec(), ctx) {
            GitResult::BranchDetected { pane_id, branch } => {
                assert_eq!(pane_id, 42);
                assert_eq!(branch, "feature/auth");
            }
            _ => panic!("expected BranchDetected"),
        }
    }

    #[test]
    fn test_parse_git_result_default_branch_filtered() {
        let mut ctx = BTreeMap::new();
        ctx.insert("type".into(), "git_branch".into());
        ctx.insert("pane_id".into(), "42".into());

        assert!(matches!(
            parse_git_result(Some(0), b"main\n".to_vec(), ctx),
            GitResult::NotGit
        ));
    }

    #[test]
    fn test_parse_git_result_failure() {
        let ctx = BTreeMap::new();
        assert!(matches!(
            parse_git_result(Some(128), vec![], ctx),
            GitResult::NotGit
        ));
    }
}
