package session

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"gopkg.in/yaml.v3"

	"github.com/rhuss/cc-mux/cc-deck/internal/config"
	"github.com/rhuss/cc-mux/cc-deck/internal/k8s"
)

// PodStatus represents the live status of a session's Pod.
type PodStatus string

const (
	PodStatusRunning  PodStatus = "Running"
	PodStatusPending  PodStatus = "Pending"
	PodStatusFailed   PodStatus = "Failed"
	PodStatusNotFound PodStatus = "Not Found"
	PodStatusDeleted  PodStatus = "Deleted"
)

// SessionInfo combines local config with live cluster state for display.
type SessionInfo struct {
	Name       string    `json:"name" yaml:"name"`
	Namespace  string    `json:"namespace" yaml:"namespace"`
	Status     PodStatus `json:"status" yaml:"status"`
	Age        string    `json:"age" yaml:"age"`
	Profile    string    `json:"profile" yaml:"profile"`
	Connection string    `json:"connection" yaml:"connection"`
}

// ListOptions configures the list operation.
type ListOptions struct {
	ConfigPath string
	Output     string // "text", "json", or "yaml"
}

// List reads the local config, reconciles with live cluster state, and writes the
// session table to the provided writer.
func List(ctx context.Context, clientset kubernetes.Interface, w io.Writer, opts ListOptions) error {
	cfg, err := config.Load(opts.ConfigPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if len(cfg.Sessions) == 0 {
		fmt.Fprintln(w, "No sessions found.")
		return nil
	}

	// Reconcile stale sessions and collect info
	infos := make([]SessionInfo, 0, len(cfg.Sessions))
	configChanged := false

	for i := range cfg.Sessions {
		sess := &cfg.Sessions[i]
		podName := k8s.ResourcePrefix(sess.Name) + "-0"
		status := getPodStatus(ctx, clientset, sess.Namespace, podName)

		// Stale session reconciliation: if the StatefulSet no longer exists, mark as deleted
		if status == PodStatusNotFound {
			stsName := k8s.ResourcePrefix(sess.Name)
			_, stsErr := clientset.AppsV1().StatefulSets(sess.Namespace).Get(ctx, stsName, metav1.GetOptions{})
			if stsErr != nil {
				status = PodStatusDeleted
				if sess.Status != "deleted" {
					sess.Status = "deleted"
					configChanged = true
				}
			}
		}

		info := SessionInfo{
			Name:      sess.Name,
			Namespace: sess.Namespace,
			Status:    status,
			Age:       formatAge(sess.CreatedAt),
			Profile:   sess.Profile,
		}

		if sess.Connection.WebURL != "" {
			info.Connection = sess.Connection.WebURL
		} else if sess.Connection.ExecTarget != "" {
			info.Connection = "exec:" + sess.Connection.ExecTarget
		}

		infos = append(infos, info)
	}

	// Save config if stale sessions were updated
	if configChanged {
		if err := cfg.Save(opts.ConfigPath); err != nil {
			return fmt.Errorf("saving config after reconciliation: %w", err)
		}
	}

	return writeOutput(w, infos, opts.Output)
}

// getPodStatus queries the cluster for the live Pod status.
func getPodStatus(ctx context.Context, clientset kubernetes.Interface, namespace, podName string) PodStatus {
	pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return PodStatusNotFound
	}

	switch pod.Status.Phase {
	case corev1.PodRunning:
		return PodStatusRunning
	case corev1.PodPending:
		return PodStatusPending
	case corev1.PodFailed:
		return PodStatusFailed
	case corev1.PodSucceeded:
		return PodStatusRunning
	default:
		return PodStatusPending
	}
}

// writeOutput formats and writes the session list in the requested format.
func writeOutput(w io.Writer, infos []SessionInfo, format string) error {
	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(infos)
	case "yaml":
		return yaml.NewEncoder(w).Encode(infos)
	default:
		return writeTable(w, infos)
	}
}

// writeTable writes a formatted table of session information.
func writeTable(w io.Writer, infos []SessionInfo) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tNAMESPACE\tSTATUS\tAGE\tPROFILE\tCONNECTION")

	for _, info := range infos {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			info.Name,
			info.Namespace,
			info.Status,
			info.Age,
			info.Profile,
			info.Connection,
		)
	}

	return tw.Flush()
}

// formatAge returns a human-readable age string from an RFC3339 timestamp.
func formatAge(createdAt string) string {
	if createdAt == "" {
		return "<unknown>"
	}

	created, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return "<unknown>"
	}

	d := time.Since(created)

	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		days := int(d.Hours()) / 24
		return fmt.Sprintf("%dd", days)
	}
}
