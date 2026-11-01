package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/cc-deck/cc-deck/internal/cmd"
)

// resetViper ensures viper's global state does not leak between tests, since
// initConfig configures the shared viper singleton.
func resetViper(t *testing.T) {
	t.Helper()
	v := viper.GetViper()
	t.Cleanup(func() {
		*v = *viper.New()
	})
}

func TestNewRootCmd_Structure(t *testing.T) {
	root := newRootCmd()

	if root.Use != appName {
		t.Errorf("Use = %q, want %q", root.Use, appName)
	}
	if !root.SilenceUsage {
		t.Error("SilenceUsage = false, want true")
	}

	wantFlags := []string{"kubeconfig", "namespace", "profile", "config", "verbose", "output"}
	for _, name := range wantFlags {
		if root.PersistentFlags().Lookup(name) == nil {
			t.Errorf("missing persistent flag %q", name)
		}
	}

	wantGroups := map[string]bool{"workspace": false, "session": false, "build": false, "config": false}
	for _, g := range root.Groups() {
		if _, ok := wantGroups[g.ID]; ok {
			wantGroups[g.ID] = true
		}
	}
	for id, found := range wantGroups {
		if !found {
			t.Errorf("missing command group %q", id)
		}
	}

	wantCmds := []string{"attach", "list", "exec", "ws", "snapshot", "build", "config", "hook", "version"}
	for _, name := range wantCmds {
		found := false
		for _, c := range root.Commands() {
			if c.Name() == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing subcommand %q", name)
		}
	}
}

func TestNewRootCmd_PersistentPreRunSkipsConfigCheckAndHook(t *testing.T) {
	root := newRootCmd()

	hasCheckSubcommand := false
	for _, c := range root.Commands() {
		if c.Name() != "config" {
			continue
		}
		for _, sub := range c.Commands() {
			if sub.Name() == "check" {
				hasCheckSubcommand = true
			}
		}
	}
	if !hasCheckSubcommand {
		t.Fatal("expected a 'config check' subcommand to exist")
	}

	// PersistentPreRun walks up the command tree looking for "check" or
	// "hook"; for either it must return before attempting config.Load, which
	// would otherwise be the only side effect observable here.
	root.PersistentPreRun(&cobra.Command{Use: "hook"}, nil)
	root.PersistentPreRun(&cobra.Command{Use: "check"}, nil)
}

func TestAddToGroup(t *testing.T) {
	parent := &cobra.Command{Use: "parent"}
	c1 := &cobra.Command{Use: "one"}
	c2 := &cobra.Command{Use: "two"}

	addToGroup(parent, "mygroup", c1, c2)

	if c1.GroupID != "mygroup" || c2.GroupID != "mygroup" {
		t.Errorf("GroupID not set: c1=%q c2=%q", c1.GroupID, c2.GroupID)
	}
	if parent.Commands()[0].Name() != "one" || parent.Commands()[1].Name() != "two" {
		t.Errorf("children not added to parent: %v", parent.Commands())
	}
}

func TestInitConfig_ExplicitConfigFile(t *testing.T) {
	resetViper(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "myconfig.yaml")
	if err := os.WriteFile(path, []byte("foo: bar\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	gf := &cmd.GlobalFlags{ConfigFile: path}
	initConfig(gf)

	if got := viper.GetString("foo"); got != "bar" {
		t.Errorf("viper.GetString(foo) = %q, want %q", got, "bar")
	}
}

func TestInitConfig_MissingConfigFileVerbose(t *testing.T) {
	resetViper(t)

	gf := &cmd.GlobalFlags{ConfigFile: filepath.Join(t.TempDir(), "does-not-exist.yaml"), Verbose: true}

	// Should not panic even though the explicit config file is missing.
	initConfig(gf)
}

func TestInitConfig_DefaultConfigPath(t *testing.T) {
	resetViper(t)

	gf := &cmd.GlobalFlags{}
	// Should not panic when falling back to the default XDG-based config path.
	initConfig(gf)
}

func TestFindBaseImagesYAML_Found(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "base-images.yaml")
	if err := os.WriteFile(path, []byte("images: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Chdir(dir)

	got, err := findBaseImagesYAML()
	if err != nil {
		t.Fatalf("findBaseImagesYAML() error = %v", err)
	}
	if got != path {
		t.Errorf("findBaseImagesYAML() = %q, want %q", got, path)
	}
}

func TestFindBaseImagesYAML_NotFound(t *testing.T) {
	t.Chdir(t.TempDir())

	_, err := findBaseImagesYAML()
	if err == nil {
		t.Fatal("findBaseImagesYAML() error = nil, want error for missing file")
	}
}
