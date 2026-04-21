package ws

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/cc-deck/cc-deck/internal/config"
)

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://github.com/org/repo.git", "github.com/org/repo"},
		{"https://github.com/org/repo", "github.com/org/repo"},
		{"https://GitHub.COM/Org/Repo.git", "github.com/Org/Repo"},
		{"git@github.com:org/repo.git", "github.com/org/repo"},
		{"git@github.com:org/repo", "github.com/org/repo"},
		{"git@GitHub.COM:org/repo.git", "github.com/org/repo"},
		{"ssh://git@github.com/org/repo.git", "github.com/org/repo"},
		{"ssh://git@github.com/org/repo", "github.com/org/repo"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizeURL(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeURL(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeURL_SameRepoFormats(t *testing.T) {
	https := NormalizeURL("https://github.com/org/repo.git")
	ssh := NormalizeURL("git@github.com:org/repo.git")
	sshProto := NormalizeURL("ssh://git@github.com/org/repo.git")

	if https != ssh {
		t.Errorf("HTTPS (%q) and SSH (%q) should normalize the same", https, ssh)
	}
	if https != sshProto {
		t.Errorf("HTTPS (%q) and ssh:// (%q) should normalize the same", https, sshProto)
	}
}

func TestRepoNameFromURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://github.com/org/repo.git", "repo"},
		{"https://github.com/org/repo", "repo"},
		{"git@github.com:org/my-project.git", "my-project"},
		{"git@github.com:org/my-project", "my-project"},
		{"ssh://git@github.com/org/utils.git", "utils"},
		{"https://gitlab.com/group/subgroup/app.git", "app"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := RepoNameFromURL(tt.input)
			if got != tt.want {
				t.Errorf("RepoNameFromURL(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTargetDir(t *testing.T) {
	tests := []struct {
		entry RepoEntry
		want  string
	}{
		{RepoEntry{URL: "https://github.com/org/repo.git"}, "repo"},
		{RepoEntry{URL: "https://github.com/org/repo.git", Target: "my-project"}, "my-project"},
		{RepoEntry{URL: "git@github.com:org/utils.git", Target: ""}, "utils"},
	}

	for _, tt := range tests {
		got := TargetDir(tt.entry)
		if got != tt.want {
			t.Errorf("TargetDir(%v) = %q, want %q", tt.entry, got, tt.want)
		}
	}
}

func TestDeduplicateRepos(t *testing.T) {
	repos := []RepoEntry{
		{URL: "https://github.com/org/repo.git", Branch: "main"},
		{URL: "git@github.com:org/repo.git", Branch: "develop"},
		{URL: "https://github.com/org/other.git"},
	}

	result := DeduplicateRepos(repos)
	if len(result) != 2 {
		t.Fatalf("expected 2 repos after dedup, got %d", len(result))
	}
	if result[0].Branch != "main" {
		t.Errorf("first occurrence should win: expected branch 'main', got %q", result[0].Branch)
	}
	if result[1].URL != "https://github.com/org/other.git" {
		t.Errorf("expected other repo preserved, got %q", result[1].URL)
	}
}

func TestBuildCloneCommand(t *testing.T) {
	t.Run("without branch or creds", func(t *testing.T) {
		entry := RepoEntry{URL: "https://github.com/org/repo.git"}
		cmd := buildCloneCommand(entry, "/workspace", nil)
		if !strings.Contains(cmd, "git clone") {
			t.Errorf("expected git clone command, got %q", cmd)
		}
		if !strings.Contains(cmd, "/workspace/repo") {
			t.Errorf("expected target path /workspace/repo, got %q", cmd)
		}
	})

	t.Run("with branch", func(t *testing.T) {
		entry := RepoEntry{URL: "https://github.com/org/repo.git", Branch: "develop"}
		cmd := buildCloneCommand(entry, "/workspace", nil)
		if !strings.Contains(cmd, "--branch") {
			t.Errorf("expected --branch flag, got %q", cmd)
		}
		if !strings.Contains(cmd, "develop") {
			t.Errorf("expected branch name, got %q", cmd)
		}
	})

	t.Run("with token credentials", func(t *testing.T) {
		entry := RepoEntry{URL: "https://github.com/org/repo.git"}
		creds := &GitCredentials{Type: config.GitCredentialToken, Token: "ghp_test123"}
		cmd := buildCloneCommand(entry, "/workspace", creds)
		if !strings.Contains(cmd, "ghp_test123@github.com") {
			t.Errorf("expected token in URL, got %q", cmd)
		}
	})

	t.Run("with SSH URL ignores token", func(t *testing.T) {
		entry := RepoEntry{URL: "git@github.com:org/repo.git"}
		creds := &GitCredentials{Type: config.GitCredentialToken, Token: "ghp_test123"}
		cmd := buildCloneCommand(entry, "/workspace", creds)
		if strings.Contains(cmd, "ghp_test123") {
			t.Errorf("token should not be injected into SSH URL, got %q", cmd)
		}
	})

	t.Run("with custom target", func(t *testing.T) {
		entry := RepoEntry{URL: "https://github.com/org/repo.git", Target: "my-project"}
		cmd := buildCloneCommand(entry, "/workspace", nil)
		if !strings.Contains(cmd, "/workspace/my-project") {
			t.Errorf("expected custom target path, got %q", cmd)
		}
	})
}

func TestBuildTokenCleanupCommand(t *testing.T) {
	entry := RepoEntry{URL: "https://github.com/org/repo.git"}
	cmd := buildTokenCleanupCommand(entry, "/workspace")
	if !strings.Contains(cmd, "remote set-url origin") {
		t.Errorf("expected remote set-url command, got %q", cmd)
	}
	if !strings.Contains(cmd, "https://github.com/org/repo.git") {
		t.Errorf("expected clean URL in set-url, got %q", cmd)
	}
}

func TestInjectToken(t *testing.T) {
	tests := []struct {
		url   string
		token string
		want  string
	}{
		{"https://github.com/org/repo.git", "mytoken", "https://mytoken@github.com/org/repo.git"},
		{"git@github.com:org/repo.git", "mytoken", "git@github.com:org/repo.git"},
		{"http://example.com/repo.git", "mytoken", "http://example.com/repo.git"},
	}

	for _, tt := range tests {
		got := injectToken(tt.url, tt.token)
		if got != tt.want {
			t.Errorf("injectToken(%q, %q) = %q, want %q", tt.url, tt.token, got, tt.want)
		}
	}
}

// mockRunner creates a CommandRunner that records commands and returns
// configurable responses.
type mockRunner struct {
	mu       sync.Mutex
	commands []string
	handler  func(cmd string) (string, error)
}

func newMockRunner(handler func(cmd string) (string, error)) *mockRunner {
	return &mockRunner{handler: handler}
}

func (m *mockRunner) run(ctx context.Context, cmd string) (string, error) {
	m.mu.Lock()
	m.commands = append(m.commands, cmd)
	m.mu.Unlock()
	return m.handler(cmd)
}

func (m *mockRunner) getCommands() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]string, len(m.commands))
	copy(result, m.commands)
	return result
}

func TestCloneRepos_IdempotentSkip(t *testing.T) {
	runner := newMockRunner(func(cmd string) (string, error) {
		if strings.Contains(cmd, "test -d") {
			return "EXISTS", nil
		}
		return "", nil
	})

	repos := []RepoEntry{{URL: "https://github.com/org/repo.git"}}
	results := cloneRepos(context.Background(), runner.run, repos, "/workspace", nil, nil, "")

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Success {
		t.Error("expected success for existing repo")
	}
	if !strings.Contains(results[0].Message, "skipped") {
		t.Errorf("expected skip message, got %q", results[0].Message)
	}

	for _, cmd := range runner.getCommands() {
		if strings.Contains(cmd, "git clone") {
			t.Error("should not have run git clone for existing directory")
		}
	}
}

func TestCloneRepos_SuccessfulClone(t *testing.T) {
	runner := newMockRunner(func(cmd string) (string, error) {
		if strings.Contains(cmd, "test -d") {
			return "MISSING", nil
		}
		return "", nil
	})

	repos := []RepoEntry{{URL: "https://github.com/org/repo.git"}}
	results := cloneRepos(context.Background(), runner.run, repos, "/workspace", nil, nil, "")

	if len(results) != 1 || !results[0].Success {
		t.Fatalf("expected 1 successful result")
	}

	foundClone := false
	for _, cmd := range runner.getCommands() {
		if strings.Contains(cmd, "git clone") {
			foundClone = true
		}
	}
	if !foundClone {
		t.Error("expected git clone command")
	}
}

func TestCloneRepos_FailureAsWarning(t *testing.T) {
	runner := newMockRunner(func(cmd string) (string, error) {
		if strings.Contains(cmd, "test -d") {
			return "MISSING", nil
		}
		if strings.Contains(cmd, "git clone") {
			return "", fmt.Errorf("network error")
		}
		return "", nil
	})

	repos := []RepoEntry{
		{URL: "https://github.com/org/repo1.git"},
		{URL: "https://github.com/org/repo2.git"},
	}
	results := cloneRepos(context.Background(), runner.run, repos, "/workspace", nil, nil, "")

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	for _, r := range results {
		if r.Success {
			t.Error("expected failure for both repos")
		}
		if !strings.Contains(r.Message, "clone failed") {
			t.Errorf("expected clone failed message, got %q", r.Message)
		}
	}
}

func TestCloneRepos_TokenCleanup(t *testing.T) {
	runner := newMockRunner(func(cmd string) (string, error) {
		if strings.Contains(cmd, "test -d") {
			return "MISSING", nil
		}
		return "", nil
	})

	repos := []RepoEntry{{URL: "https://github.com/org/repo.git"}}
	creds := &GitCredentials{Type: config.GitCredentialToken, Token: "ghp_abc"}
	cloneRepos(context.Background(), runner.run, repos, "/workspace", creds, nil, "")

	foundCleanup := false
	for _, cmd := range runner.getCommands() {
		if strings.Contains(cmd, "remote set-url") {
			foundCleanup = true
			if strings.Contains(cmd, "ghp_abc") {
				t.Error("cleanup command should not contain token")
			}
		}
	}
	if !foundCleanup {
		t.Error("expected token cleanup command")
	}
}

func TestCloneRepos_MaxConcurrency(t *testing.T) {
	var maxConcurrent int32
	var currentConcurrent int32

	runner := newMockRunner(func(cmd string) (string, error) {
		if strings.Contains(cmd, "test -d") {
			return "MISSING", nil
		}
		if strings.Contains(cmd, "git clone") {
			current := atomic.AddInt32(&currentConcurrent, 1)
			for {
				max := atomic.LoadInt32(&maxConcurrent)
				if current > max {
					atomic.CompareAndSwapInt32(&maxConcurrent, max, current)
				} else {
					break
				}
			}
			atomic.AddInt32(&currentConcurrent, -1)
		}
		return "", nil
	})

	repos := make([]RepoEntry, 8)
	for i := range repos {
		repos[i] = RepoEntry{URL: fmt.Sprintf("https://github.com/org/repo%d.git", i)}
	}
	cloneRepos(context.Background(), runner.run, repos, "/workspace", nil, nil, "")

	observed := atomic.LoadInt32(&maxConcurrent)
	if observed > int32(maxConcurrentClones) {
		t.Errorf("max concurrent clones exceeded: observed %d, limit %d", observed, maxConcurrentClones)
	}
}

func TestCloneRepos_ExtraRemotes(t *testing.T) {
	runner := newMockRunner(func(cmd string) (string, error) {
		if strings.Contains(cmd, "test -d") {
			return "MISSING", nil
		}
		return "", nil
	})

	repos := []RepoEntry{
		{URL: "https://github.com/org/repo.git"},
		{URL: "https://github.com/org/other.git"},
	}
	extras := map[string]string{
		"upstream": "https://github.com/upstream/repo.git",
	}
	cloneRepos(context.Background(), runner.run, repos, "/workspace", nil, extras, NormalizeURL("https://github.com/org/repo.git"))

	foundRemoteAdd := false
	foundRemoteOnSecond := false
	for _, cmd := range runner.getCommands() {
		if strings.Contains(cmd, "remote add") && strings.Contains(cmd, "upstream") {
			foundRemoteAdd = true
			if strings.Contains(cmd, "/workspace/other") {
				foundRemoteOnSecond = true
			}
		}
	}
	if !foundRemoteAdd {
		t.Error("expected git remote add for upstream")
	}
	if foundRemoteOnSecond {
		t.Error("extra remotes should only be applied to the first (auto-detected) repo")
	}
}

func TestCloneRepos_Deduplication(t *testing.T) {
	var cloneCount int32
	runner := newMockRunner(func(cmd string) (string, error) {
		if strings.Contains(cmd, "test -d") {
			return "MISSING", nil
		}
		if strings.Contains(cmd, "git clone") {
			atomic.AddInt32(&cloneCount, 1)
		}
		return "", nil
	})

	repos := []RepoEntry{
		{URL: "https://github.com/org/repo.git"},
		{URL: "git@github.com:org/repo.git"},
	}
	results := cloneRepos(context.Background(), runner.run, repos, "/workspace", nil, nil, "")

	if len(results) != 1 {
		t.Errorf("expected 1 result after dedup, got %d", len(results))
	}
	if atomic.LoadInt32(&cloneCount) != 1 {
		t.Errorf("expected 1 clone after dedup, got %d", cloneCount)
	}
}

func TestResolveGitCredentials(t *testing.T) {
	t.Run("SSH type", func(t *testing.T) {
		creds, err := resolveGitCredentials(config.GitCredentialSSH, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if creds == nil || creds.Type != config.GitCredentialSSH {
			t.Error("expected SSH credentials")
		}
	})

	t.Run("token from env var", func(t *testing.T) {
		t.Setenv("TEST_GIT_TOKEN", "mytoken123")
		creds, err := resolveGitCredentials(config.GitCredentialToken, "TEST_GIT_TOKEN")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if creds == nil || creds.Token != "mytoken123" {
			t.Error("expected token credentials with resolved value")
		}
	})

	t.Run("empty env var", func(t *testing.T) {
		t.Setenv("EMPTY_TOKEN", "")
		_, err := resolveGitCredentials(config.GitCredentialToken, "EMPTY_TOKEN")
		if err == nil {
			t.Error("expected error for empty env var")
		}
	})

	t.Run("unconfigured", func(t *testing.T) {
		creds, err := resolveGitCredentials("", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if creds != nil {
			t.Error("expected nil credentials for unconfigured")
		}
	})
}

func TestDefinitionRoundTrip_WithRepos(t *testing.T) {
	dir := t.TempDir()
	store := NewDefinitionStore(dir + "/workspaces.yaml")

	def := &WorkspaceDefinition{
		Name: "test-env",
		Type: WorkspaceTypeSSH,
		Host: "user@host",
		Repos: []RepoEntry{
			{URL: "https://github.com/org/repo.git", Branch: "main", Target: "my-repo"},
			{URL: "git@github.com:org/other.git"},
		},
	}

	if err := store.Add(def); err != nil {
		t.Fatalf("Add: %v", err)
	}

	loaded, err := store.FindByName("test-env")
	if err != nil {
		t.Fatalf("FindByName: %v", err)
	}

	if len(loaded.Repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(loaded.Repos))
	}
	if loaded.Repos[0].URL != "https://github.com/org/repo.git" {
		t.Errorf("expected first repo URL preserved, got %q", loaded.Repos[0].URL)
	}
	if loaded.Repos[0].Branch != "main" {
		t.Errorf("expected branch preserved, got %q", loaded.Repos[0].Branch)
	}
	if loaded.Repos[0].Target != "my-repo" {
		t.Errorf("expected target preserved, got %q", loaded.Repos[0].Target)
	}
}
