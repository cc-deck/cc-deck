package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/cc-deck/cc-deck/internal/build"
	"github.com/cc-deck/cc-deck/internal/build/imageprobe"
)

func validateImageRef(ref string) error {
	if strings.HasPrefix(ref, "-") {
		return fmt.Errorf("invalid image reference %q: must not start with '-'", ref)
	}
	for _, c := range ref {
		if c < 0x20 || c == 0x7f {
			return fmt.Errorf("invalid image reference: contains control characters")
		}
	}
	if ref == "" {
		return fmt.Errorf("image reference must not be empty")
	}
	return nil
}

func newBuildProbeCmd(_ *GlobalFlags) *cobra.Command {
	var setupDir string
	var format string
	var noCache bool
	var timeout int

	cmd := &cobra.Command{
		Use:   "probe <image-ref>",
		Short: "Inspect a base image for OS, package manager, and tools",
		Long: `Probe a container image to discover its OS family, package manager,
pre-installed tools with versions, user setup, and shell availability.

Results are cached by image reference and digest so repeat probes skip
the container execution. Use --no-cache to force a fresh probe.

When a build.yaml manifest is available in the setup directory, the
output includes a diff section comparing required vs installed tools.`,
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			imageRef := args[0]
			if err := validateImageRef(imageRef); err != nil {
				return err
			}
			return runBuildProbe(cmd.Context(), imageRef, setupDir, format, noCache, timeout)
		},
	}

	cmd.Flags().StringVar(&setupDir, "setup-dir", "", "Path to setup directory (default: .cc-deck/setup)")
	cmd.Flags().StringVar(&format, "format", "table", "Output format: json or table")
	cmd.Flags().BoolVar(&noCache, "no-cache", false, "Force a fresh probe, ignoring cached results")
	cmd.Flags().IntVar(&timeout, "timeout", 30, "Probe timeout in seconds")

	return cmd
}

func runBuildProbe(ctx context.Context, imageRef, setupDir, format string, noCache bool, timeout int) error {
	if setupDir == "" {
		dir, _ := resolveBuildDirAndRoot(nil)
		setupDir = dir
	}

	probeCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	cached := false
	var result *imageprobe.ProbeResult

	if !noCache {
		cache, err := imageprobe.LoadCache(setupDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not load probe cache: %v\n", err)
		} else {
			digest, err := imageprobe.ResolveDigest(probeCtx, imageRef)
			if err == nil {
				if entry, hit := imageprobe.LookupCache(cache, imageRef, digest); hit {
					result = &entry
					cached = true
				}
			}
		}
	}

	if result == nil {
		tools := resolveProbeTools(setupDir)

		var err error
		result, err = imageprobe.RunProbe(probeCtx, imageRef, tools)
		if err != nil {
			return fmt.Errorf("probe failed: %w", err)
		}

		digest, _ := imageprobe.ResolveDigest(probeCtx, imageRef)
		result.ImageDigest = digest

		cache, _ := imageprobe.LoadCache(setupDir)
		if cache != nil {
			imageprobe.StoreResult(cache, result)
			if err := imageprobe.SaveCache(setupDir, cache); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not save probe cache: %v\n", err)
			}
		}
	}

	switch format {
	case "json":
		output, err := imageprobe.FormatJSON(result, cached)
		if err != nil {
			return err
		}
		fmt.Println(output)
	case "table":
		fmt.Print(imageprobe.FormatTable(result, cached))

		diffs := computeManifestDiff(setupDir, result)
		if len(diffs) > 0 {
			fmt.Print(imageprobe.FormatDiff(diffs))
		}
	default:
		return fmt.Errorf("invalid format %q: must be json or table", format)
	}

	return nil
}

func resolveProbeTools(setupDir string) []imageprobe.ProbeToolEntry {
	manifestPath := filepath.Join(setupDir, "build.yaml")
	m, err := build.LoadManifest(manifestPath)
	if err != nil {
		return imageprobe.MergeToolSets(nil)
	}

	var manifestProbeTools []imageprobe.ProbeToolEntry
	for _, pt := range m.ProbeTools {
		manifestProbeTools = append(manifestProbeTools, imageprobe.ProbeToolEntry{
			Name:    pt.Name,
			Version: pt.Version,
		})
	}

	return imageprobe.MergeToolSets(manifestProbeTools)
}

func computeManifestDiff(setupDir string, result *imageprobe.ProbeResult) []imageprobe.ToolDiff {
	manifestPath := filepath.Join(setupDir, "build.yaml")
	m, err := build.LoadManifest(manifestPath)
	if err != nil {
		return nil
	}

	probeVersions := make(map[string]string, len(m.ProbeTools))
	for _, pt := range m.ProbeTools {
		probeVersions[pt.Name] = pt.Version
	}

	var required []imageprobe.ProbeToolEntry
	for _, t := range m.Tools {
		required = append(required, imageprobe.ProbeToolEntry{
			Name:    t.Name,
			Version: probeVersions[t.Name],
		})
	}

	if len(required) == 0 {
		return nil
	}

	return imageprobe.ComputeToolDiff(result.Tools, required, result.PackageManager)
}
