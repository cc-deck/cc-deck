package session

import (
	"context"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/cc-deck/cc-deck/internal/k8s"
)

// LogsOptions configures the logs operation.
type LogsOptions struct {
	Follow     bool
	TailLines  int64
	Timestamps bool
}

// Logs streams Pod logs to the provided writer.
func Logs(ctx context.Context, clientset kubernetes.Interface, w io.Writer, sessionName, namespace string, opts LogsOptions) error {
	podName := k8s.ResourcePrefix(sessionName) + "-0"

	logOpts := &corev1.PodLogOptions{
		Follow:     opts.Follow,
		Timestamps: opts.Timestamps,
	}

	if opts.TailLines > 0 {
		logOpts.TailLines = &opts.TailLines
	}

	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, logOpts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return fmt.Errorf("opening log stream for pod %q: %w", podName, err)
	}
	defer stream.Close()

	if _, err := io.Copy(w, stream); err != nil {
		return fmt.Errorf("reading log stream: %w", err)
	}

	return nil
}
