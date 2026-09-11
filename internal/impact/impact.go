// Package impact assesses the blast radius of a configuration change: it
// diffs two snapshots of one service/environment scope and lists the other
// services whose latest snapshot declares a dependency on the changed
// service.
package impact

import (
	"fmt"
	"sort"

	"example.com/config-snapshot-registry/internal/domain"
	"example.com/config-snapshot-registry/internal/engine"
)

// AffectedService is a service/environment scope whose latest snapshot
// depends on the assessed service. SnapshotID records the snapshot that
// declares the dependency, so callers can audit the evidence.
type AffectedService struct {
	Service     string `json:"service"`
	Environment string `json:"environment"`
	SnapshotID  string `json:"snapshot_id"`
}

// Assessment describes the impact of changing one scope from one snapshot to
// another: the configuration changes plus the services that may be affected.
type Assessment struct {
	Service     string                `json:"service"`
	Environment string                `json:"environment"`
	From        domain.SnapshotRecord `json:"from"`
	To          domain.SnapshotRecord `json:"to"`
	Changes     []engine.Change       `json:"changes"`
	Affected    []AffectedService     `json:"affected"`
}

// Assess compares the from/to snapshots, which must both exist and belong to
// the requested service/environment scope, and reports the scopes that depend
// on the changed service.
//
// Dependency semantics: a snapshot lists the services it depends on, so the
// affected set is the reverse-dependency lookup — every scope whose latest
// snapshot in the same environment names the changed service. The assessment
// is environment-scoped because dependency declarations do not carry an
// environment. Every dependency declared by the to snapshot must have at
// least one snapshot in that environment, otherwise the dependency graph is
// incomplete and Assess returns domain.ErrIncompleteDependencies instead of a
// partial answer.
func Assess(state domain.State, serviceName, environment, fromID, toID string) (Assessment, error) {
	from, ok := state.Snapshots[fromID]
	if !ok {
		return Assessment{}, fmt.Errorf("from: %w", domain.ErrSnapshotNotFound)
	}
	to, ok := state.Snapshots[toID]
	if !ok {
		return Assessment{}, fmt.Errorf("to: %w", domain.ErrSnapshotNotFound)
	}
	if from.Snapshot.Service != to.Snapshot.Service || from.Snapshot.Environment != to.Snapshot.Environment {
		return Assessment{}, fmt.Errorf("%w: %s and %s belong to different scopes", domain.ErrScopeMismatch, fromID, toID)
	}
	if from.Snapshot.Service != serviceName || from.Snapshot.Environment != environment {
		return Assessment{}, fmt.Errorf("%w: snapshots do not belong to %s/%s", domain.ErrScopeMismatch, serviceName, environment)
	}
	if err := checkDependencies(state, to.Snapshot); err != nil {
		return Assessment{}, err
	}
	return Assessment{
		Service:     serviceName,
		Environment: environment,
		From:        from,
		To:          to,
		Changes:     engine.Compare(from.Snapshot.Values, to.Snapshot.Values),
		Affected:    affectedScopes(state, serviceName, environment),
	}, nil
}

// checkDependencies requires every service the snapshot depends on to have at
// least one snapshot in the same environment, so the assessment never reasons
// over a partially observed dependency graph.
func checkDependencies(state domain.State, snapshot domain.Snapshot) error {
	scopes := make(map[string]struct{}, len(state.Snapshots))
	for _, record := range state.Snapshots {
		scopes[domain.ScopeKey(record.Snapshot.Service, record.Snapshot.Environment)] = struct{}{}
	}
	for _, dependency := range snapshot.Dependencies {
		if _, ok := scopes[domain.ScopeKey(dependency, snapshot.Environment)]; !ok {
			return fmt.Errorf("%w: no snapshot for dependency %q in environment %q", domain.ErrIncompleteDependencies, dependency, snapshot.Environment)
		}
	}
	return nil
}

// affectedScopes lists the scopes whose latest snapshot in the given
// environment depends on the changed service. Only the latest snapshot per
// scope counts: a dependency removed in a newer snapshot no longer exposes
// that scope. The changed scope itself is never its own victim.
func affectedScopes(state domain.State, serviceName, environment string) []AffectedService {
	affected := make([]AffectedService, 0)
	for _, snapshotID := range state.Latest {
		record, ok := state.Snapshots[snapshotID]
		if !ok {
			continue
		}
		snapshot := record.Snapshot
		if snapshot.Environment != environment {
			continue
		}
		if snapshot.Service == serviceName {
			continue
		}
		if !dependsOn(snapshot, serviceName) {
			continue
		}
		affected = append(affected, AffectedService{
			Service:     snapshot.Service,
			Environment: snapshot.Environment,
			SnapshotID:  snapshot.ID,
		})
	}
	sort.Slice(affected, func(i, j int) bool {
		return affected[i].Service < affected[j].Service
	})
	return affected
}

func dependsOn(snapshot domain.Snapshot, serviceName string) bool {
	for _, dependency := range snapshot.Dependencies {
		if dependency == serviceName {
			return true
		}
	}
	return false
}
