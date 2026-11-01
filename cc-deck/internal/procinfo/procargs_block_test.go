package procinfo

import (
	"reflect"
	"testing"
)

// parseEnvBlock and parseArgvBlock are the platform-independent NUL-block
// parsers used by the Linux /proc backend (procinfo_linux.go). They had no
// direct unit tests of their own; coverage only came indirectly through the
// Linux-only fakeProc integration tests.

func TestParseArgvBlock_Empty(t *testing.T) {
	exe, args := parseArgvBlock(nil)
	if exe != "" || args != nil {
		t.Errorf("got exe=%q args=%v, want \"\", nil", exe, args)
	}

	exe, args = parseArgvBlock([]byte{})
	if exe != "" || args != nil {
		t.Errorf("got exe=%q args=%v, want \"\", nil", exe, args)
	}
}

func TestParseArgvBlock_SingleArg(t *testing.T) {
	exe, args := parseArgvBlock([]byte("codex\x00"))
	if exe != "codex" || len(args) != 0 {
		t.Errorf("got exe=%q args=%v, want exe=%q args=[]", exe, args, "codex")
	}
}

func TestParseArgvBlock_Multiple(t *testing.T) {
	exe, args := parseArgvBlock([]byte("codex\x00-C\x00work dir\x00resume\x00"))
	if exe != "codex" {
		t.Errorf("exe: got %q, want %q", exe, "codex")
	}
	want := []string{"-C", "work dir", "resume"}
	if !reflect.DeepEqual(args, want) {
		t.Errorf("args: got %v, want %v", args, want)
	}
}

func TestParseArgvBlock_NoTrailingNUL(t *testing.T) {
	// Defensive: a cmdline block missing its final NUL terminator still
	// splits on the embedded separators.
	exe, args := parseArgvBlock([]byte("codex\x00resume"))
	if exe != "codex" {
		t.Errorf("exe: got %q, want %q", exe, "codex")
	}
	if want := []string{"resume"}; !reflect.DeepEqual(args, want) {
		t.Errorf("args: got %v, want %v", args, want)
	}
}

func TestParseArgvBlock_OnlyNULs(t *testing.T) {
	// TrimSuffix only removes one trailing NUL, so "\x00\x00" leaves a
	// single empty element after the first argument.
	exe, args := parseArgvBlock([]byte("codex\x00\x00"))
	if exe != "codex" {
		t.Errorf("exe: got %q, want %q", exe, "codex")
	}
	if want := []string{""}; !reflect.DeepEqual(args, want) {
		t.Errorf("args: got %v, want %v", args, want)
	}
}

func TestParseEnvBlock_Empty(t *testing.T) {
	env := parseEnvBlock(nil)
	if len(env) != 0 {
		t.Errorf("got %v, want empty map", env)
	}
}

func TestParseEnvBlock_Normal(t *testing.T) {
	env := parseEnvBlock([]byte("PATH=/bin\x00ZELLIJ_PANE_ID=4\x00"))
	want := map[string]string{"PATH": "/bin", "ZELLIJ_PANE_ID": "4"}
	if !reflect.DeepEqual(env, want) {
		t.Errorf("got %v, want %v", env, want)
	}
}

func TestParseEnvBlock_EmptyValue(t *testing.T) {
	env := parseEnvBlock([]byte("EMPTY=\x00"))
	if v, ok := env["EMPTY"]; !ok || v != "" {
		t.Errorf("env[EMPTY]: got (%q, %v), want (\"\", true)", v, ok)
	}
}

func TestParseEnvBlock_ValueContainsEquals(t *testing.T) {
	env := parseEnvBlock([]byte("WEIRD=a=b\x00"))
	if v := env["WEIRD"]; v != "a=b" {
		t.Errorf("env[WEIRD]: got %q, want %q", v, "a=b")
	}
}

func TestParseEnvBlock_MalformedEntriesSkipped(t *testing.T) {
	// "NOEQUALS" has no '=' and is skipped; "=val" has an empty key and is
	// also skipped. The trailing empty string left by strings.Split after a
	// final NUL is likewise skipped.
	env := parseEnvBlock([]byte("NOEQUALS\x00=val\x00GOOD=1\x00"))
	want := map[string]string{"GOOD": "1"}
	if !reflect.DeepEqual(env, want) {
		t.Errorf("got %v, want %v", env, want)
	}
}
