package build

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type ProbeResult struct {
	Binary    string `json:"binary"`
	Path      string `json:"path"`
	Method    string `json:"method"`
	Component string `json:"component"`
}

type ProbeReport struct {
	Results  map[string][]ProbeResult
	Warnings []string
	Duration time.Duration
}

func collectProbeBinaries(comp PolicyComponent) []string {
	if len(comp.ProbeBinaries) > 0 {
		return comp.ProbeBinaries
	}
	return comp.Match.Tools
}

func generateProbeScript(components []PolicyComponent) string {
	var sb strings.Builder
	sb.WriteString("#!/bin/sh\n")

	for _, comp := range components {
		binaries := collectProbeBinaries(comp)
		for _, bin := range binaries {
			if !isValidBinaryName(bin) {
				continue
			}
			sb.WriteString(fmt.Sprintf(
				`timeout 30 sh -c 'p=$(which %s 2>/dev/null) && printf '"'"'{"binary":"%s","path":"%%s","method":"which","component":"%s"}\n'"'"' "$p" || { p=$(find / -name %s -type f -executable -print -quit 2>/dev/null) && printf '"'"'{"binary":"%s","path":"%%s","method":"find","component":"%s"}\n'"'"' "$p" || printf '"'"'{"binary":"%s","path":"","method":"not-found","component":"%s"}\n'"'"'; }'`,
				bin, bin, comp.Key,
				bin, bin, comp.Key,
				bin, comp.Key,
			))
			sb.WriteByte('\n')
		}
	}

	return sb.String()
}

func parseProbeOutput(output string) (map[string][]ProbeResult, []string) {
	results := make(map[string][]ProbeResult)
	var warnings []string

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var pr ProbeResult
		if err := json.Unmarshal([]byte(line), &pr); err != nil {
			continue
		}

		results[pr.Component] = append(results[pr.Component], pr)
		if pr.Method == "not-found" {
			warnings = append(warnings, fmt.Sprintf("binary %q not found in image for component %s", pr.Binary, pr.Component))
		}
	}

	return results, warnings
}

func ProbeBinaries(ctx context.Context, runtime string, imageRef string, components []PolicyComponent) (*ProbeReport, error) {
	var probeComponents []PolicyComponent
	for _, comp := range components {
		if len(comp.Binaries) > 0 {
			continue
		}
		if len(comp.Match.Tools) == 0 && len(comp.ProbeBinaries) == 0 {
			continue
		}
		probeComponents = append(probeComponents, comp)
	}

	if len(probeComponents) == 0 {
		return &ProbeReport{
			Results: make(map[string][]ProbeResult),
		}, nil
	}

	script := generateProbeScript(probeComponents)

	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(probeCtx, runtime, "run", "--rm", imageRef, "/bin/sh", "-c", script)
	cmd.Stderr = os.Stderr

	start := time.Now()
	out, err := cmd.Output()
	duration := time.Since(start)

	if err != nil {
		return nil, fmt.Errorf("probe container failed: %w", err)
	}

	results, warnings := parseProbeOutput(string(out))

	return &ProbeReport{
		Results:  results,
		Warnings: warnings,
		Duration: duration,
	}, nil
}
