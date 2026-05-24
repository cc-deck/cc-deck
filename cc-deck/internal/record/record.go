package record

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/cc-deck/cc-deck/internal/build"
	"github.com/cc-deck/cc-deck/internal/podman"
)

const (
	CoreDNSImage   = "docker.io/coredns/coredns:1.12.0"
	dnsLogDir      = "/var/log/coredns"
	dnsLogFilename = "queries.log"
	corefilePath   = "/etc/coredns/Corefile"
)

// SessionConfig holds configuration for a recording session.
type SessionConfig struct {
	WorkspaceImage string
}

// GenerateCorefile returns a minimal CoreDNS Corefile that forwards queries
// to upstream DNS and logs all queries to a file.
func GenerateCorefile() string {
	return `.:53 {
    forward . /etc/resolv.conf
    log . "{combined}" {
        class all
    }
}
`
}

// RunRecordingSession orchestrates the full DNS-recording session lifecycle.
// It creates a Podman pod with a workspace container and CoreDNS sidecar,
// attaches the user interactively, and on exit extracts and processes the
// DNS query log.
func RunRecordingSession(ctx context.Context, cfg SessionConfig) (*RecordingResult, error) {
	if err := validateImage(ctx, cfg.WorkspaceImage); err != nil {
		return nil, err
	}

	timestamp := time.Now().Format("20060102-150405")
	podName := "cc-deck-record-" + timestamp
	volumeName := podName + "-dnslog"

	tmpDir, err := os.MkdirTemp("", "cc-deck-record-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp dir: %w", err)
	}

	corefileLocal := filepath.Join(tmpDir, "Corefile")
	if err := os.WriteFile(corefileLocal, []byte(GenerateCorefile()), 0o644); err != nil {
		return nil, fmt.Errorf("writing Corefile: %w", err)
	}

	if err := podman.VolumeCreate(ctx, volumeName); err != nil {
		return nil, fmt.Errorf("creating DNS log volume: %w", err)
	}

	var cleanupOnce sync.Once
	cleanup := func() {
		cleanupOnce.Do(func() {
			cleanCtx := context.Background()
			_ = podman.PodRemove(cleanCtx, podName)
			_ = podman.VolumeRemove(cleanCtx, volumeName)
			os.RemoveAll(tmpDir)
		})
	}
	defer cleanup()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		select {
		case <-sigCh:
			fmt.Println("\nInterrupted, cleaning up...")
			cleanup()
			os.Exit(1)
		case <-done:
			return
		}
	}()
	defer func() {
		signal.Stop(sigCh)
		close(done)
	}()

	if err := podman.PodCreate(ctx, podName, "--dns", "127.0.0.1"); err != nil {
		return nil, fmt.Errorf("creating pod: %w", err)
	}

	sidecarName := podName + "-dns"
	if err := runInPod(ctx, podName, sidecarName, CoreDNSImage,
		[]string{corefileLocal + ":" + corefilePath + ":ro", volumeName + ":/var/log/coredns"},
		[]string{"sh", "-c", "/coredns -conf " + corefilePath + " > " + dnsLogDir + "/" + dnsLogFilename + " 2>&1"}); err != nil {
		return nil, fmt.Errorf("starting CoreDNS sidecar: %w", err)
	}

	workspaceName := podName + "-workspace"
	if err := runInPod(ctx, podName, workspaceName, cfg.WorkspaceImage,
		[]string{volumeName + ":/var/log/coredns:ro"},
		[]string{"sleep", "infinity"}); err != nil {
		return nil, fmt.Errorf("starting workspace container: %w", err)
	}

	fmt.Println("Recording session started. All DNS queries will be captured.")
	fmt.Println("Use the workspace normally. Exit with Ctrl+D or 'exit' when done.")
	fmt.Println()

	_ = podman.ExecWithCleanup(ctx, workspaceName, []string{"/bin/bash"}, "\n")

	fmt.Println()
	fmt.Println("Session ended. Processing DNS log...")

	logContent, err := extractDNSLog(ctx, volumeName)
	if err != nil {
		return nil, fmt.Errorf("extracting DNS log: %w", err)
	}

	entries := ParseDNSLog(strings.NewReader(logContent))
	filtered := FilterNoise(entries)
	domains := DeduplicateDomains(filtered.Domains)

	result := &RecordingResult{
		ObservedDomains: domains,
		TotalQueries:    len(entries),
		FilteredCount:   filtered.NoiseCount,
	}

	return result, nil
}

// RecordingResult holds the output of post-session processing.
type RecordingResult struct {
	ObservedDomains []string
	CoveredDomains  []CoveredDomain
	NewDomains      []string
	TotalQueries    int
	FilteredCount   int
}

// CoveredDomain represents a domain that is already handled by an existing
// catalog component or manifest entry.
type CoveredDomain struct {
	Domain    string
	CoveredBy string
}

// UpdateManifest loads the manifest, appends new domains to allowed_domains
// (deduplicating against existing entries), and saves it back.
func UpdateManifest(manifestPath string, newDomains []string) error {
	m, err := build.LoadManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("loading manifest: %w", err)
	}

	if m.Network == nil {
		m.Network = &build.NetworkConfig{}
	}

	existing := make(map[string]bool, len(m.Network.AllowedDomains))
	for _, d := range m.Network.AllowedDomains {
		existing[strings.ToLower(d)] = true
	}

	for _, d := range newDomains {
		if !existing[strings.ToLower(d)] {
			m.Network.AllowedDomains = append(m.Network.AllowedDomains, d)
			existing[strings.ToLower(d)] = true
		}
	}

	return build.SaveManifest(m, manifestPath)
}

// PrintSummary prints the recording session summary to stdout.
func PrintSummary(result *RecordingResult, manifestPath string) {
	fmt.Printf("\n=== Recording Session Summary ===\n\n")
	fmt.Printf("  Total DNS queries:    %d\n", result.TotalQueries)
	fmt.Printf("  Filtered as noise:    %d\n", result.FilteredCount)
	fmt.Printf("  Unique domains:       %d\n", len(result.ObservedDomains))

	if len(result.CoveredDomains) > 0 {
		fmt.Printf("  Already covered:      %d\n", len(result.CoveredDomains))
		for _, cd := range result.CoveredDomains {
			fmt.Printf("    - %s (by %s)\n", cd.Domain, cd.CoveredBy)
		}
	}

	if len(result.NewDomains) > 0 {
		fmt.Printf("  New domains added:    %d\n", len(result.NewDomains))
		for _, d := range result.NewDomains {
			fmt.Printf("    + %s\n", d)
		}
		fmt.Printf("\n  Updated: %s\n", manifestPath)
		fmt.Println("  Run 'cc-deck build refresh' to regenerate the policy.")
	} else if len(result.ObservedDomains) == 0 {
		fmt.Println("  No meaningful egress domains observed.")
	} else {
		fmt.Println("  All observed domains are already covered.")
	}
	fmt.Println()
}

func validateImage(ctx context.Context, image string) error {
	cmd := exec.CommandContext(ctx, "podman", "image", "inspect", image)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("workspace image %q not found; run 'cc-deck build run --target openshell' first", image)
	}
	return nil
}

func runInPod(ctx context.Context, podName, containerName, image string, volumes []string, cmd []string) error {
	args := []string{"run", "-d", "--pod", podName, "--name", containerName}
	for _, v := range volumes {
		args = append(args, "-v", v)
	}
	args = append(args, image)
	args = append(args, cmd...)

	c := exec.CommandContext(ctx, "podman", args...)
	out, err := c.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func extractDNSLog(ctx context.Context, volumeName string) (string, error) {
	extractName := "cc-deck-extract-" + time.Now().Format("150405")
	c := exec.CommandContext(ctx, "podman", "run", "--rm", "--name", extractName,
		"-v", volumeName+":/data:ro",
		"docker.io/library/busybox:latest",
		"cat", "/data/"+dnsLogFilename)

	out, err := c.CombinedOutput()
	if err != nil {
		combined := string(out)
		if strings.Contains(combined, "No such file") {
			return "", nil
		}
		return "", fmt.Errorf("extracting DNS log: %w (%s)", err, strings.TrimSpace(combined))
	}
	return string(out), nil
}
