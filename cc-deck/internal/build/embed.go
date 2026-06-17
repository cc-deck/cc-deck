package build

import (
	"embed"
	"fmt"
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

//go:embed templates/containerfile/*.tmpl
var embeddedContainerfileTemplates embed.FS

//go:embed policies/*.yaml
var embeddedPolicies embed.FS

//go:embed base-images.yaml
var embeddedBaseImages embed.FS

//go:embed skills/cc-deck-base-images/SKILL.md
var embeddedSkills embed.FS

// ExtractCommands writes embedded command files to the target directory.
func ExtractCommands(targetDir string) error {
	return extractFS(embeddedCommands, "commands", targetDir)
}

// ExtractScripts writes embedded script files to the target directory.
func ExtractScripts(targetDir string) error {
	return extractFS(embeddedScripts, "scripts", targetDir)
}

// ExtractSkills writes embedded skill files to the target directory.
func ExtractSkills(targetDir string) error {
	return extractFS(embeddedSkills, "skills", targetDir)
}

// ExtractBaseImagesYAML writes the embedded base-images.yaml to the target path.
func ExtractBaseImagesYAML(targetPath string) error {
	data, err := embeddedBaseImages.ReadFile("base-images.yaml")
	if err != nil {
		return fmt.Errorf("reading embedded base-images.yaml: %w", err)
	}
	return os.WriteFile(targetPath, data, 0o644)
}

// EmbeddedBaseImagesYAML returns the raw content of the embedded base-images.yaml.
func EmbeddedBaseImagesYAML() ([]byte, error) {
	return embeddedBaseImages.ReadFile("base-images.yaml")
}

// ManifestTemplate returns the manifest template content.
func ManifestTemplate() ([]byte, error) {
	return embeddedTemplates.ReadFile("templates/build.yaml.tmpl")
}

// extractFS walks an embedded filesystem and writes files to disk.
func extractFS(fsys embed.FS, root string, targetDir string) error {
	return fs.WalkDir(fsys, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
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
