package setup

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed commands/*.md
var embeddedCommands embed.FS

//go:embed scripts/*.sh
var embeddedScripts embed.FS

//go:embed templates/*.tmpl
var embeddedTemplates embed.FS

// ExtractCommands writes embedded command files to the target directory.
func ExtractCommands(targetDir string) error {
	return extractFS(embeddedCommands, "commands", targetDir)
}

// ExtractScripts writes embedded script files to the target directory.
func ExtractScripts(targetDir string) error {
	return extractFS(embeddedScripts, "scripts", targetDir)
}

// ManifestTemplate returns the manifest template content.
func ManifestTemplate() ([]byte, error) {
	return embeddedTemplates.ReadFile("templates/cc-deck-setup.yaml.tmpl")
}

// extractFS walks an embedded filesystem and writes files to disk.
func extractFS(fsys embed.FS, root string, targetDir string) error {
	return fs.WalkDir(fsys, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Get the relative path within the embedded FS
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		target := filepath.Join(targetDir, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		data, err := fsys.ReadFile(path)
		if err != nil {
			return err
		}

		// Preserve executable bit for scripts
		perm := os.FileMode(0o644)
		if filepath.Ext(path) == ".sh" {
			perm = 0o755
		}

		return os.WriteFile(target, data, perm)
	})
}
