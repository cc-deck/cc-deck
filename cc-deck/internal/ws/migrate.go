package ws

import (
	"fmt"
	"strings"
	"time"

	"github.com/cc-deck/cc-deck/internal/config"
)

// MigrateFromConfig reads the config file at configPath, converts any
// Session entries to WorkspaceInstance values with type K8sDeploy, saves
// them to the state store, and removes the sessions from the config file.
//
// This function is idempotent: if the config file does not exist or has
// no sessions, it returns nil without modifying anything.
func MigrateFromConfig(configPath string, store *FileStateStore) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("loading config for migration: %w", err)
	}

	if len(cfg.Sessions) == 0 {
		return nil
	}

	state, err := store.Load()
	if err != nil {
		return fmt.Errorf("loading state for migration: %w", err)
	}

	existingNames := make(map[string]bool, len(state.Instances))
	for _, inst := range state.Instances {
		existingNames[inst.Name] = true
	}

	for _, sess := range cfg.Sessions {
		if existingNames[sess.Name] {
			continue
		}

		inst := sessionToInstance(sess)
		state.Instances = append(state.Instances, inst)
		existingNames[sess.Name] = true
	}

	if err := store.Save(state); err != nil {
		return fmt.Errorf("saving migrated state: %w", err)
	}

	// Clear sessions from config and save it back.
	cfg.Sessions = nil
	if err := cfg.Save(configPath); err != nil {
		return fmt.Errorf("saving config after migration: %w", err)
	}

	return nil
}

// sessionToInstance converts a legacy config.Session to a WorkspaceInstance.
func sessionToInstance(sess config.Session) WorkspaceInstance {
	inst := WorkspaceInstance{
		Name:  sess.Name,
		Type:  WorkspaceTypeK8sDeploy,
		State: mapSessionStatus(sess.Status),
	}

	// Parse created_at timestamp.
	if sess.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339, sess.CreatedAt); err == nil {
			inst.CreatedAt = t
		}
	}

	// Populate K8s fields.
	inst.K8s = &K8sFields{
		Namespace:   sess.Namespace,
		Profile:     sess.Profile,
		StatefulSet: deriveStatefulSet(sess.PodName),
	}

	return inst
}

// mapSessionStatus converts a legacy session status string to a WorkspaceState.
func mapSessionStatus(status string) WorkspaceState {
	switch strings.ToLower(status) {
	case "running":
		return WorkspaceStateRunning
	default:
		return WorkspaceStateUnknown
	}
}

// deriveStatefulSet strips the trailing "-0" ordinal suffix from a pod name
// to derive the StatefulSet name.
func deriveStatefulSet(podName string) string {
	if podName == "" {
		return ""
	}
	return strings.TrimSuffix(podName, "-0")
}
