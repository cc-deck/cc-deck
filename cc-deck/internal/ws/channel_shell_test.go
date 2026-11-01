package ws

import (
	"context"
	"os"
	"os/exec"
	"testing"
)

func TestShellQuote(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "simple", in: "hello", want: "'hello'"},
		{name: "empty", in: "", want: "''"},
		{name: "spaces", in: "hello world", want: "'hello world'"},
		{
			name: "single quote",
			in:   "it's",
			want: `'it'\''s'`,
		},
		{
			name: "multiple single quotes",
			in:   "'a'b'",
			want: `''\''a'\''b'\'''`,
		},
		{
			name: "shell metacharacters are inert inside quotes",
			in:   "$(rm -rf /); echo pwned",
			want: "'$(rm -rf /); echo pwned'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shellQuote(tt.in)
			if got != tt.want {
				t.Errorf("shellQuote(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestShellQuote_RoundTripsThroughShell(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	inputs := []string{
		"hello world",
		"it's a test",
		"$(echo injected)",
		"a;b|c&d",
		"",
	}

	for _, in := range inputs {
		in := in
		t.Run(in, func(t *testing.T) {
			cmd := exec.Command("sh", "-c", "printf '%s' "+shellQuote(in))
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("running quoted command: %v", err)
			}
			if string(out) != in {
				t.Errorf("round-trip = %q, want %q", string(out), in)
			}
		})
	}
}

func TestShellJoin(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "no args", args: nil, want: ""},
		{name: "single arg", args: []string{"foo"}, want: "'foo'"},
		{
			name: "multiple args",
			args: []string{"foo", "bar baz"},
			want: "'foo' 'bar baz'",
		},
		{
			name: "arg with single quote",
			args: []string{"it's"},
			want: `'it'\''s'`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shellJoin(tt.args)
			if got != tt.want {
				t.Errorf("shellJoin(%v) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}

func TestCurrentBranch(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", "main", "-q")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")
	run("commit", "--allow-empty", "-q", "-m", "init")

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	got, err := currentBranch(context.Background())
	if err != nil {
		t.Fatalf("currentBranch: %v", err)
	}
	if got != "main" {
		t.Errorf("currentBranch() = %q, want %q", got, "main")
	}
}

func TestCurrentBranch_NotAGitRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	if _, err := currentBranch(context.Background()); err == nil {
		t.Error("currentBranch() in non-git dir: want error, got nil")
	}
}
