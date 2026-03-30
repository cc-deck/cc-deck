package tui

import (
	"time"

	"github.com/cc-deck/cc-deck/internal/env"
)

// envRow is the flattened display model for one environment in the list view.
type envRow struct {
	name           string
	envType        string
	state          string
	sessionCount   int
	attentionCount int
	storageName    string
	lastAttached   *time.Time
	tags           []string
}

// buildEnvRows merges records, instances, and definitions into a flat list
// of envRow for display in the list view.
func buildEnvRows(store *env.FileStateStore, defs *env.DefinitionStore) []envRow {
	// Reconcile state with actual runtime status.
	_ = env.ReconcileLocalEnvs(store)
	_ = env.ReconcileContainerEnvs(store, defs)
	_ = env.ReconcileComposeEnvs(store)

	var rows []envRow
	seen := make(map[string]bool)

	// V1 records (local environments).
	records, err := store.List(nil)
	if err == nil {
		for _, r := range records {
			seen[r.Name] = true
			rows = append(rows, envRow{
				name:         r.Name,
				envType:      string(r.Type),
				state:        string(r.State),
				storageName:  storageDisplayName(r),
				lastAttached: r.LastAttached,
			})
		}
	}

	// V2 instances (container/compose environments).
	instances, err := store.ListInstances()
	if err == nil {
		for _, inst := range instances {
			if seen[inst.Name] {
				continue
			}
			seen[inst.Name] = true
			instType := string(inst.Type)
			if instType == "" {
				instType = "container"
			}
			storage := "named-volume"
			if inst.Compose != nil {
				storage = "host-path"
			}
			rows = append(rows, envRow{
				name:         inst.Name,
				envType:      instType,
				state:        string(inst.State),
				storageName:  storage,
				lastAttached: inst.LastAttached,
			})
		}
	}

	// Definitions without instances ("not created").
	allDefs, err := defs.List(nil)
	if err == nil {
		for _, d := range allDefs {
			if seen[d.Name] {
				continue
			}
			if d.Type == env.EnvironmentTypeLocal {
				continue
			}
			rows = append(rows, envRow{
				name:        d.Name,
				envType:     string(d.Type),
				state:       "not created",
				storageName: "-",
			})
		}
	}

	return rows
}

func storageDisplayName(r *env.EnvironmentRecord) string {
	if r.Type == env.EnvironmentTypeLocal {
		return "host"
	}
	if r.Storage != nil {
		return string(r.Storage.Type)
	}
	return "-"
}
