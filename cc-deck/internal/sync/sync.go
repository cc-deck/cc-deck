package sync

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

// DefaultExcludes are paths excluded from sync by default.
var DefaultExcludes = []string{
	".git",
	"node_modules",
	"target",
	"__pycache__",
}

// SyncOptions holds parameters for file synchronization.
type SyncOptions struct {
	PodName       string
	Namespace     string
	ContainerName string
	LocalDir      string
	RemoteDir     string
	Excludes      []string
	Clientset     kubernetes.Interface
	RestConfig    *rest.Config
}

// Push syncs files from the local directory to the Pod.
// Uses tar to stream files: tar -cf - --exclude <patterns> -C <local> . | exec tar -xf - -C <remote>
func Push(ctx context.Context, opts SyncOptions) error {
	if opts.RemoteDir == "" {
		opts.RemoteDir = "/workspace"
	}
	if opts.ContainerName == "" {
		opts.ContainerName = "claude"
	}

	excludes := mergeExcludes(opts.Excludes)

	// Create local tar archive
	tarArgs := buildTarCreateArgs(excludes, opts.LocalDir)
	tarCmd := exec.CommandContext(ctx, "tar", tarArgs...)
	tarCmd.Dir = opts.LocalDir

	tarOut, err := tarCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("creating tar stdout pipe: %w", err)
	}

	var tarErr bytes.Buffer
	tarCmd.Stderr = &tarErr

	if err := tarCmd.Start(); err != nil {
		return fmt.Errorf("starting local tar: %w", err)
	}

	// Stream tar output into the Pod via exec
	extractCmd := []string{"tar", "-xf", "-", "-C", opts.RemoteDir}
	if err := execInPod(ctx, opts, extractCmd, tarOut, nil); err != nil {
		// Kill the local tar process on error
		_ = tarCmd.Process.Kill()
		return fmt.Errorf("extracting files in Pod: %w", err)
	}

	if err := tarCmd.Wait(); err != nil {
		return fmt.Errorf("local tar failed: %w (stderr: %s)", err, tarErr.String())
	}

	return nil
}

// Pull syncs files from the Pod to the local directory.
// Uses tar to stream files: exec tar -cf - -C <remote> . | tar -xf - -C <local>
func Pull(ctx context.Context, opts SyncOptions) error {
	if opts.RemoteDir == "" {
		opts.RemoteDir = "/workspace"
	}
	if opts.ContainerName == "" {
		opts.ContainerName = "claude"
	}

	excludes := mergeExcludes(opts.Excludes)

	// Build remote tar command with excludes
	remoteTarArgs := []string{"tar", "-cf", "-"}
	for _, exc := range excludes {
		remoteTarArgs = append(remoteTarArgs, "--exclude", exc)
	}
	remoteTarArgs = append(remoteTarArgs, "-C", opts.RemoteDir, ".")

	// Set up a pipe: remote tar stdout -> local tar stdin
	pr, pw := io.Pipe()

	errCh := make(chan error, 1)

	// Run remote tar in Pod, writing to pipe writer
	go func() {
		defer pw.Close()
		errCh <- execInPod(ctx, opts, remoteTarArgs, nil, pw)
	}()

	// Run local tar extract, reading from pipe reader
	extractArgs := []string{"-xf", "-", "-C", opts.LocalDir}
	extractCmd := exec.CommandContext(ctx, "tar", extractArgs...)
	extractCmd.Stdin = pr

	var extractErr bytes.Buffer
	extractCmd.Stderr = &extractErr

	if err := extractCmd.Run(); err != nil {
		return fmt.Errorf("local tar extract failed: %w (stderr: %s)", err, extractErr.String())
	}

	// Wait for remote tar to finish
	if err := <-errCh; err != nil {
		return fmt.Errorf("remote tar in Pod failed: %w", err)
	}

	return nil
}

// buildTarCreateArgs builds the arguments for the local tar create command.
func buildTarCreateArgs(excludes []string, localDir string) []string {
	args := []string{"-cf", "-"}
	for _, exc := range excludes {
		args = append(args, "--exclude", exc)
	}
	args = append(args, "-C", localDir, ".")
	return args
}

// mergeExcludes combines user-specified excludes with defaults, avoiding duplicates.
func mergeExcludes(userExcludes []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, exc := range DefaultExcludes {
		if !seen[exc] {
			seen[exc] = true
			result = append(result, exc)
		}
	}

	for _, exc := range userExcludes {
		if !seen[exc] {
			seen[exc] = true
			result = append(result, exc)
		}
	}

	return result
}

// execInPod executes a command in a Pod container using the SPDY executor.
// If stdin is provided, it is piped to the command. If stdout is provided,
// the command's output is written to it.
func execInPod(ctx context.Context, opts SyncOptions, command []string, stdin io.Reader, stdout io.Writer) error {
	req := opts.Clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(opts.PodName).
		Namespace(opts.Namespace).
		SubResource("exec")

	req.VersionedParams(&corev1.PodExecOptions{
		Container: opts.ContainerName,
		Command:   command,
		Stdin:     stdin != nil,
		Stdout:    true,
		Stderr:    true,
		TTY:       false,
	}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(opts.RestConfig, http.MethodPost, req.URL())
	if err != nil {
		return fmt.Errorf("creating SPDY executor: %w", err)
	}

	var stderrBuf bytes.Buffer
	streamOpts := remotecommand.StreamOptions{
		Stdout: stdout,
		Stderr: &stderrBuf,
	}
	if stdin != nil {
		streamOpts.Stdin = stdin
	}
	if stdout == nil {
		streamOpts.Stdout = os.Stdout
	}

	if err := executor.StreamWithContext(ctx, streamOpts); err != nil {
		stderr := strings.TrimSpace(stderrBuf.String())
		if stderr != "" {
			return fmt.Errorf("exec failed: %w (stderr: %s)", err, stderr)
		}
		return fmt.Errorf("exec failed: %w", err)
	}

	return nil
}
