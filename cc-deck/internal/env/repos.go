package env

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/cc-deck/cc-deck/internal/config"
)

// RepoEntry describes a git repository to clone into the workspace.
type RepoEntry struct {
	URL    string `yaml:"url"`
	Branch string `yaml:"branch,omitempty"`
	Target string `yaml:"target,omitempty"`
}

// CommandRunner executes a shell command on the target environment and
// returns the combined stdout output.
type CommandRunner func(ctx context.Context, cmd string) (string, error)

// GitCredentials holds resolved credential information for git operations.
type GitCredentials struct {
	Type  config.GitCredentialType
	Token string
}

// RepoCloneResult captures the outcome of a single repo clone operation.
type RepoCloneResult struct {
	Entry   RepoEntry
	Success bool
	Message string
}

// NormalizeURL strips the .git suffix, lowercases the host, and converts
// SSH git URLs to a comparable canonical form.
func NormalizeURL(rawURL string) string {
	// Convert git@host:path SSH format to a pseudo-URL for comparison.
	if strings.HasPrefix(rawURL, "git@") {
		// git@github.com:org/repo.git -> github.com/org/repo
		rest := strings.TrimPrefix(rawURL, "git@")
		if idx := strings.Index(rest, ":"); idx >= 0 {
			host := strings.ToLower(rest[:idx])
			p := rest[idx+1:]
			p = strings.TrimSuffix(p, ".git")
			return host + "/" + p
		}
	}

	// Handle ssh:// URLs.
	if strings.HasPrefix(rawURL, "ssh://") {
		parsed, err := url.Parse(rawURL)
		if err == nil {
			host := strings.ToLower(parsed.Hostname())
			p := strings.TrimPrefix(parsed.Path, "/")
			p = strings.TrimSuffix(p, ".git")
			return host + "/" + p
		}
	}

	// HTTPS or other URL format.
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return strings.TrimSuffix(rawURL, ".git")
	}
	host := strings.ToLower(parsed.Hostname())
	p := strings.TrimPrefix(parsed.Path, "/")
	p = strings.TrimSuffix(p, ".git")
	return host + "/" + p
}

// RepoNameFromURL extracts the repository name from a git URL. This is
// the last path component without the .git suffix, matching git clone
// default behavior.
func RepoNameFromURL(rawURL string) string {
	// Handle SSH git@host:path format.
	if strings.HasPrefix(rawURL, "git@") {
		rest := strings.TrimPrefix(rawURL, "git@")
		if idx := strings.Index(rest, ":"); idx >= 0 {
			p := rest[idx+1:]
			return strings.TrimSuffix(path.Base(p), ".git")
		}
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		base := path.Base(rawURL)
		return strings.TrimSuffix(base, ".git")
	}
	return strings.TrimSuffix(path.Base(parsed.Path), ".git")
}

// TargetDir returns the directory name where the repo should be cloned.
// Uses the explicit Target field if set, otherwise derives from the URL.
func TargetDir(entry RepoEntry) string {
	if entry.Target != "" {
		return entry.Target
	}
	return RepoNameFromURL(entry.URL)
}

// DeduplicateRepos removes duplicate repos by normalized URL. The first
// occurrence wins, preserving its branch and target settings.
func DeduplicateRepos(repos []RepoEntry) []RepoEntry {
	seen := make(map[string]bool)
	var result []RepoEntry
	for _, r := range repos {
		key := NormalizeURL(r.URL)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, r)
	}
	return result
}

// resolveGitCredentials resolves the token value from the active Profile's
// credential configuration. Returns nil when no credentials are configured
// or for SSH type (handled by agent forwarding at the SSH client level).
func resolveGitCredentials(credType config.GitCredentialType, credSecret string) (*GitCredentials, error) {
	switch credType {
	case config.GitCredentialSSH:
		return &GitCredentials{Type: config.GitCredentialSSH}, nil
	case config.GitCredentialToken:
		if credSecret == "" {
			return nil, nil
		}
		// Resolve from environment variable.
		token := os.Getenv(credSecret)
		if token == "" {
			return nil, fmt.Errorf("git credential env var %q is empty or not set", credSecret)
		}
		return &GitCredentials{Type: config.GitCredentialToken, Token: token}, nil
	default:
		return nil, nil
	}
}

// buildCloneCommand constructs a git clone command string for the given
// repo entry. When credentials are provided and the URL is HTTPS, the
// token is injected into the URL.
func buildCloneCommand(entry RepoEntry, workspace string, creds *GitCredentials) string {
	cloneURL := entry.URL
	if creds != nil && creds.Type == config.GitCredentialToken && creds.Token != "" {
		cloneURL = injectToken(cloneURL, creds.Token)
	}

	target := TargetDir(entry)
	targetPath := workspace + "/" + target

	cmd := fmt.Sprintf("git clone %q %q", cloneURL, targetPath)
	if entry.Branch != "" {
		cmd = fmt.Sprintf("git clone --branch %q %q %q", entry.Branch, cloneURL, targetPath)
	}
	return cmd
}

// buildTokenCleanupCommand returns a git remote set-url command that
// rewrites the origin URL to remove any embedded token.
func buildTokenCleanupCommand(entry RepoEntry, workspace string) string {
	target := TargetDir(entry)
	targetPath := workspace + "/" + target
	return fmt.Sprintf("git -C %q remote set-url origin %q", targetPath, entry.URL)
}

// injectToken inserts a token into an HTTPS URL for authentication.
// For non-HTTPS URLs, returns the URL unchanged.
func injectToken(rawURL, token string) string {
	if !strings.HasPrefix(rawURL, "https://") {
		return rawURL
	}
	return "https://" + token + "@" + strings.TrimPrefix(rawURL, "https://")
}

// loadActiveGitCredentials loads the active Profile from config and resolves
// git credentials. Returns nil credentials if no profile is configured or
// the profile has no git credentials set.
func loadActiveGitCredentials() *GitCredentials {
	cfg, err := config.Load("")
	if err != nil {
		return nil
	}
	profileName := cfg.ResolveProfile("")
	if profileName == "" {
		return nil
	}
	profile, err := cfg.GetProfile(profileName)
	if err != nil {
		return nil
	}
	creds, err := resolveGitCredentials(profile.GitCredentialType, profile.GitCredentialSecret)
	if err != nil {
		log.Printf("WARNING: resolving git credentials: %v", err)
		return nil
	}
	return creds
}

const maxConcurrentClones = 4

// cloneRepos clones the given repositories into the workspace directory
// using the provided CommandRunner. Repos are cloned in parallel (max 4).
// Clone failures are warnings, not fatal errors.
//
// extraRemotes is a map of remote-name to URL, applied only to the repo
// matching autoDetectedURL (the auto-detected repo from the user's cwd).
func cloneRepos(ctx context.Context, runner CommandRunner, repos []RepoEntry, workspace string, creds *GitCredentials, extraRemotes map[string]string, autoDetectedURL string) []RepoCloneResult {
	repos = DeduplicateRepos(repos)
	results := make([]RepoCloneResult, len(repos))

	sem := make(chan struct{}, maxConcurrentClones)
	var wg sync.WaitGroup

	for i, entry := range repos {
		wg.Add(1)
		isAutoDetected := autoDetectedURL != "" && NormalizeURL(entry.URL) == autoDetectedURL
		go func(idx int, e RepoEntry, autoDetected bool) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			results[idx] = cloneSingleRepo(ctx, runner, e, workspace, creds, autoDetected, extraRemotes)
		}(i, entry, isAutoDetected)
	}

	wg.Wait()

	for _, r := range results {
		if r.Success {
			log.Printf("  Cloned %s", TargetDir(r.Entry))
		} else {
			log.Printf("  WARNING: %s: %s", TargetDir(r.Entry), r.Message)
		}
	}

	return results
}

func cloneSingleRepo(ctx context.Context, runner CommandRunner, entry RepoEntry, workspace string, creds *GitCredentials, isAutoDetected bool, extraRemotes map[string]string) RepoCloneResult {
	if entry.URL == "" {
		return RepoCloneResult{Entry: entry, Success: false, Message: "empty URL"}
	}

	target := TargetDir(entry)
	targetPath := workspace + "/" + target

	// Idempotency: check if the directory already exists.
	checkCmd := fmt.Sprintf("test -d %q && echo EXISTS || echo MISSING", targetPath)
	out, err := runner(ctx, checkCmd)
	if err == nil && strings.TrimSpace(out) == "EXISTS" {
		return RepoCloneResult{
			Entry:   entry,
			Success: true,
			Message: "already exists, skipped",
		}
	}

	// Clone the repository.
	cloneCmd := buildCloneCommand(entry, workspace, creds)
	if _, cloneErr := runner(ctx, cloneCmd); cloneErr != nil {
		return RepoCloneResult{
			Entry:   entry,
			Success: false,
			Message: fmt.Sprintf("clone failed: %v", cloneErr),
		}
	}

	// Clean up token from remote URL if token-based auth was used.
	if creds != nil && creds.Type == config.GitCredentialToken && creds.Token != "" {
		cleanupCmd := buildTokenCleanupCommand(entry, workspace)
		if _, cleanErr := runner(ctx, cleanupCmd); cleanErr != nil {
			log.Printf("  WARNING: could not clean token from %s: %v", target, cleanErr)
		}
	}

	// Add extra remotes for the auto-detected repo.
	if isAutoDetected && len(extraRemotes) > 0 {
		for name, remoteURL := range extraRemotes {
			addCmd := fmt.Sprintf("git -C %q remote add %q %q", targetPath, name, remoteURL)
			if _, addErr := runner(ctx, addCmd); addErr != nil {
				log.Printf("  WARNING: could not add remote %q to %s: %v", name, target, addErr)
			}
		}
	}

	return RepoCloneResult{
		Entry:   entry,
		Success: true,
		Message: "cloned successfully",
	}
}
