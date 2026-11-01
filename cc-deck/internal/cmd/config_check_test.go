package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cc-deck/cc-deck/internal/config"
)

// captureCheckStdout redirects os.Stdout for the duration of fn and returns
// everything written to it. runCheck uses fmt.Println/fmt.Printf directly
// (not a cobra OutOrStdout writer), so tests must swap os.Stdout itself.
func captureCheckStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)

	origStdout := os.Stdout
	os.Stdout = w

	fnErr := fn()

	w.Close()
	os.Stdout = origStdout

	var buf bytes.Buffer
	_, err = buf.ReadFrom(r)
	require.NoError(t, err)
	return buf.String(), fnErr
}

func saveConfigFile(t *testing.T, cfg *config.Config) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, cfg.Save(path))
	return path
}

func TestRunCheck_NoIssues(t *testing.T) {
	cfgPath := saveConfigFile(t, &config.Config{})
	gf := &GlobalFlags{ConfigFile: cfgPath}

	out, err := captureCheckStdout(t, func() error { return runCheck(gf) })

	require.NoError(t, err)
	assert.Contains(t, out, "config: no issues found")
}

func TestRunCheck_WarningOnly(t *testing.T) {
	silence := 20.0 // > 10 triggers a voice.silence warning, not an error
	cfgPath := saveConfigFile(t, &config.Config{
		Defaults: config.Defaults{Voice: config.VoiceDefaults{Silence: &silence}},
	})
	gf := &GlobalFlags{ConfigFile: cfgPath}

	out, err := captureCheckStdout(t, func() error { return runCheck(gf) })

	require.NoError(t, err, "warnings alone must not fail the check")
	assert.Contains(t, out, "[voice]")
	assert.Contains(t, out, "warning")
	assert.Contains(t, out, "voice.silence 20 is unusually high")
	assert.NotContains(t, out, "error(s) found")
}

func TestRunCheck_ErrorFailsCheck(t *testing.T) {
	cfgPath := saveConfigFile(t, &config.Config{
		Sharing: config.SharingConfig{VerifyTimeout: -1},
	})
	gf := &GlobalFlags{ConfigFile: cfgPath}

	out, err := captureCheckStdout(t, func() error { return runCheck(gf) })

	require.Error(t, err)
	assert.Contains(t, err.Error(), "1 error(s) found")
	assert.Contains(t, out, "[sharing]")
	assert.Contains(t, out, "error")
	assert.Contains(t, out, "sharing.verify_timeout must be positive")
}

func TestRunCheck_GroupsAndOrdersByCategory(t *testing.T) {
	// Sharing (error) and voice (warning) findings in one config: categories
	// must be printed in stable alphabetical order ("sharing" before "voice").
	preRoll := 5.0 // > 2 triggers a voice.pre_roll warning
	cfgPath := saveConfigFile(t, &config.Config{
		Sharing:  config.SharingConfig{VerifyTimeout: -1},
		Defaults: config.Defaults{Voice: config.VoiceDefaults{PreRoll: &preRoll}},
	})
	gf := &GlobalFlags{ConfigFile: cfgPath}

	out, err := captureCheckStdout(t, func() error { return runCheck(gf) })

	require.Error(t, err)
	sharingIdx := bytes.Index([]byte(out), []byte("[sharing]"))
	voiceIdx := bytes.Index([]byte(out), []byte("[voice]"))
	require.NotEqual(t, -1, sharingIdx)
	require.NotEqual(t, -1, voiceIdx)
	assert.Less(t, sharingIdx, voiceIdx, "sharing category should print before voice")
}

func TestRunCheck_LoadErrorPropagates(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("not: [valid yaml"), 0o644))
	gf := &GlobalFlags{ConfigFile: cfgPath}

	_, err := captureCheckStdout(t, func() error { return runCheck(gf) })

	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing config file")
}

func TestNewCheckCmd_Wiring(t *testing.T) {
	cfgPath := saveConfigFile(t, &config.Config{})
	gf := &GlobalFlags{ConfigFile: cfgPath}
	cmd := NewCheckCmd(gf)

	assert.Equal(t, "check", cmd.Use)
	assert.Contains(t, cmd.Aliases, "validate")

	cmd.SetArgs([]string{})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	out, err := captureCheckStdout(t, func() error { return cmd.Execute() })

	require.NoError(t, err)
	assert.Contains(t, out, "config: no issues found")
}
