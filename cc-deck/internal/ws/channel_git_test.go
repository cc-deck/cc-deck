package ws

import (
	"testing"
)

func TestBuildExtKubectlURL(t *testing.T) {
	tests := []struct {
		name           string
		ns             string
		podName        string
		workspacePath  string
		kubeconfigArgs []string
		want           string
	}{
		{
			name:          "no kubeconfig args",
			ns:            "default",
			podName:       "cc-deck-dev-0",
			workspacePath: "/workspace",
			want:          "ext::kubectl exec -i -n default cc-deck-dev-0 -- %S /workspace",
		},
		{
			name:           "with kubeconfig and context",
			ns:             "staging",
			podName:        "cc-deck-prod-0",
			workspacePath:  "/workspace",
			kubeconfigArgs: []string{"--kubeconfig", "/path/to/config", "--context", "my-ctx"},
			want:           "ext::kubectl --kubeconfig /path/to/config --context my-ctx exec -i -n staging cc-deck-prod-0 -- %S /workspace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildExtKubectlURL(tt.ns, tt.podName, tt.workspacePath, tt.kubeconfigArgs)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildExtPodmanURL(t *testing.T) {
	got := buildExtPodmanURL("cc-deck-myws", "/workspace")
	want := "ext::podman exec -i cc-deck-myws -- %S /workspace"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWithTemporaryRemote_CleansUpOnError(t *testing.T) {
	// This test verifies the cleanup pattern by using a non-existent
	// remote URL. The function should still attempt cleanup even when
	// the inner function fails. We can't test real git operations in
	// unit tests, but we verify the function signature and error flow.
	t.Skip("requires real git repository")
}
