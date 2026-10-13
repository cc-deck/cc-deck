package ws

import (
	"context"
	"fmt"

	"github.com/cc-deck/cc-deck/internal/agent"
	"github.com/cc-deck/cc-deck/internal/config"
	"github.com/cc-deck/cc-deck/internal/profile"
	"github.com/cc-deck/cc-deck/internal/ssh"
)

// BuildProfileTarget creates a profile.Target for the named workspace,
// suitable for passing to profile.Provision. Only SSH and OpenShell workspace
// types support remote provisioning; other types return an error.
func BuildProfileTarget(ctx context.Context, name string, store *FileStateStore, defs *DefinitionStore) (profile.Target, error) {
	inst, err := store.FindInstanceByName(name)
	if err != nil {
		return nil, fmt.Errorf("workspace %q not found: %w", name, err)
	}

	switch inst.Type {
	case WorkspaceTypeSSH:
		return buildSSHProfileTarget(ctx, name, defs)
	case WorkspaceTypeOpenShell:
		return buildOpenShellProfileTarget(ctx, name, store, defs)
	default:
		return nil, fmt.Errorf("workspace type %q does not support remote profile provisioning", inst.Type)
	}
}

// buildSSHProfileTarget creates a profile.Target backed by an SSH connection.
func buildSSHProfileTarget(ctx context.Context, name string, defs *DefinitionStore) (profile.Target, error) {
	if defs == nil {
		return nil, fmt.Errorf("no definition store available")
	}
	def, err := defs.FindByName(name)
	if err != nil {
		return nil, fmt.Errorf("loading workspace definition: %w", err)
	}
	if def.Host == "" {
		return nil, fmt.Errorf("SSH host is required for workspace %q", name)
	}
	client := ssh.NewClient(def.Host, def.Port, def.IdentityFile, def.JumpHost, def.SSHConfig)
	return newSSHTarget(ctx, client)
}

// buildOpenShellProfileTarget creates a profile.Target backed by an OpenShell
// sandbox.
func buildOpenShellProfileTarget(ctx context.Context, name string, store *FileStateStore, defs *DefinitionStore) (profile.Target, error) {
	w := &OpenShellWorkspace{name: name, store: store, defs: defs}
	if err := w.ensureClient(); err != nil {
		return nil, err
	}
	w.loadSandboxID()
	if w.sandboxID == "" {
		return nil, fmt.Errorf("workspace %q has no sandbox; create it first", name)
	}
	return &openShellTarget{ws: w, ctx: ctx, home: "/sandbox"}, nil
}

// NeededProfileProviders computes the provider names that would be created for
// the given config and workspace during OpenShell sandbox creation. Callers
// can compare the result with the recorded OpenShellFields.Providers to detect
// profiles added after workspace creation (FR-026 re-create hint).
func NeededProfileProviders(cfg *config.Config, wsName string) []string {
	var names []string
	seen := make(map[string]bool)
	for pName, p := range cfg.Profiles {
		harness := p.HarnessName()
		a := agent.Get(harness)
		if a == nil {
			continue
		}
		tr, ok := profile.Lookup(harness)
		if !ok {
			continue
		}
		backend := p.EffectiveBackend()
		provType := tr.ProviderType(backend)
		if provType == "" {
			continue
		}
		provName := fmt.Sprintf("cc-deck-%s-%s", wsName, pName)
		if seen[provName] {
			continue
		}
		seen[provName] = true
		names = append(names, provName)
	}
	return names
}
