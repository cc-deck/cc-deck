package shellrc

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	markerStart = "# >>> cc-deck >>>"
	markerEnd   = "# <<< cc-deck <<<"

	// pathContent is the managed PATH export written by EnsureAll.
	pathContent = `export PATH="$HOME/.local/share/cc-deck/bin:$PATH"`
)

// Ensure writes or replaces the cc-deck managed block in the file at path.
// The block is delimited by marker lines:
//
//	# >>> cc-deck >>>
//	<content>
//	# <<< cc-deck <<<
//
// Creates the file if absent. Preserves everything outside the markers byte
// for byte. Returns true when the file was changed.
func Ensure(path, content string) (changed bool, err error) {
	block := markerStart + "\n" + content + "\n" + markerEnd + "\n"

	existing, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return false, err
		}
		return true, os.WriteFile(path, []byte(block), 0o644)
	}
	if err != nil {
		return false, err
	}

	raw := string(existing)
	startIdx := strings.Index(raw, markerStart)
	endIdx := strings.Index(raw, markerEnd)

	if startIdx >= 0 && endIdx >= 0 && endIdx > startIdx {
		// Markers found: replace everything from start marker through end
		// marker line (including its trailing newline if present).
		afterEnd := endIdx + len(markerEnd)
		if afterEnd < len(raw) && raw[afterEnd] == '\n' {
			afterEnd++
		}
		oldBlock := raw[startIdx:afterEnd]
		if oldBlock == block {
			return false, nil
		}
		result := raw[:startIdx] + block + raw[afterEnd:]
		return true, os.WriteFile(path, []byte(result), 0o644)
	}

	// No markers: append the block.
	var buf bytes.Buffer
	buf.Write(existing)
	if len(existing) > 0 && existing[len(existing)-1] != '\n' {
		buf.WriteByte('\n')
	}
	buf.WriteString(block)
	if bytes.Equal(existing, buf.Bytes()) {
		return false, nil
	}
	return true, os.WriteFile(path, buf.Bytes(), 0o644)
}

// EnsureAll writes the managed PATH block to ~/.bashrc and ~/.zshrc under
// the given home directory. Returns true when any file was changed.
func EnsureAll(home string) (changed bool, err error) {
	for _, name := range []string{".bashrc", ".zshrc"} {
		c, e := Ensure(filepath.Join(home, name), pathContent)
		if e != nil {
			return changed, e
		}
		if c {
			changed = true
		}
	}
	return changed, nil
}
