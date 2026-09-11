package impact

import (
	"errors"
	"testing"
	"time"

	"example.com/config-snapshot-registry/internal/domain"
	"example.com/config-snapshot-registry/internal/engine"
)

var base = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func stateWith(snapshots ...domain.Snapshot) domain.State {
	state := domain.NewState()
	for _, snapshot := range snapshots {
		state.Snapshots[snapshot.ID] = domain.SnapshotRecord{Snapshot: snapshot, CreatedAt: snapshot.CapturedAt}
		key := domain.ScopeKey(snapshot.Service, snapshot.Environment)
		if current, ok := state.Latest[key]; !ok || snapshot.CapturedAt.After(state.Snapshots[current].Snapshot.CapturedAt) {
			state.Latest[key] = snapshot.ID
		}
	}
	return state
}

func TestAssessReportsChangesAndAffectedServices(t *testing.T) {
	state := stateWith(
		domain.Snapshot{ID: "snap-p1", Service: "payments", Environment: "prod", CapturedAt: base, Values: map[string]any{"timeout": 3}, Dependencies: []string{"auth"}},
		domain.Snapshot{ID: "snap-p2", Service: "payments", Environment: "prod", CapturedAt: base.Add(time.Hour), Values: map[string]any{"timeout": 5, "retries": 2}, Dependencies: []string{"auth"}},
		domain.Snapshot{ID: "snap-a1", Service: "auth", Environment: "prod", CapturedAt: base, Values: map[string]any{}},
		domain.Snapshot{ID: "snap-c1", Service: "checkout", Environment: "prod", CapturedAt: base, Values: map[string]any{}, Dependencies: []string{"payments", "auth"}},
		domain.Snapshot{ID: "snap-s1", Service: "search", Environment: "prod", CapturedAt: base, Values: map[string]any{}, Dependencies: []string{"payments"}},
		domain.Snapshot{ID: "snap-b1", Service: "billing", Environment: "staging", CapturedAt: base, Values: map[string]any{}, Dependencies: []string{"payments"}},
	)
	assessment, err := Assess(state, "payments", "prod", "snap-p1", "snap-p2")
	if err != nil {
		t.Fatalf("assess: %v", err)
	}
	if len(assessment.Changes) != 2 {
		t.Fatalf("got %d changes, want 2: %#v", len(assessment.Changes), assessment.Changes)
	}
	if assessment.Changes[0].Path != "retries" || assessment.Changes[0].Kind != engine.Added {
		t.Fatalf("first change = %#v, want added retries", assessment.Changes[0])
	}
	if assessment.Changes[1].Path != "timeout" || assessment.Changes[1].Kind != engine.Modified {
		t.Fatalf("second change = %#v, want modified timeout", assessment.Changes[1])
	}
	if len(assessment.Affected) != 2 {
		t.Fatalf("got %d affected, want 2: %#v", len(assessment.Affected), assessment.Affected)
	}
	if assessment.Affected[0].Service != "checkout" || assessment.Affected[0].SnapshotID != "snap-c1" {
		t.Fatalf("affected[0] = %#v, want checkout via snap-c1", assessment.Affected[0])
	}
	if assessment.Affected[1].Service != "search" {
		t.Fatalf("affected[1] = %#v, want search", assessment.Affected[1])
	}
	for _, affected := range assessment.Affected {
		if affected.Service == "billing" || affected.Environment != "prod" {
			t.Fatalf("cross-environment scope leaked into affected: %#v", affected)
		}
	}
}

func TestAssessUsesLatestSnapshotForDependencies(t *testing.T) {
	state := stateWith(
		domain.Snapshot{ID: "snap-p1", Service: "payments", Environment: "prod", CapturedAt: base, Values: map[string]any{"timeout": 3}},
		domain.Snapshot{ID: "snap-p2", Service: "payments", Environment: "prod", CapturedAt: base.Add(time.Hour), Values: map[string]any{"timeout": 5}},
		domain.Snapshot{ID: "snap-c1", Service: "checkout", Environment: "prod", CapturedAt: base, Values: map[string]any{}, Dependencies: []string{"payments"}},
		domain.Snapshot{ID: "snap-c2", Service: "checkout", Environment: "prod", CapturedAt: base.Add(2 * time.Hour), Values: map[string]any{}, Dependencies: []string{"auth"}},
		domain.Snapshot{ID: "snap-a1", Service: "auth", Environment: "prod", CapturedAt: base, Values: map[string]any{}},
	)
	assessment, err := Assess(state, "payments", "prod", "snap-p1", "snap-p2")
	if err != nil {
		t.Fatalf("assess: %v", err)
	}
	if len(assessment.Affected) != 0 {
		t.Fatalf("checkout dropped the dependency in its latest snapshot, want no affected: %#v", assessment.Affected)
	}
}

func TestAssessRequiresExistingSnapshots(t *testing.T) {
	state := stateWith(
		domain.Snapshot{ID: "snap-1", Service: "payments", Environment: "prod", CapturedAt: base, Values: map[string]any{}},
	)
	if _, err := Assess(state, "payments", "prod", "missing", "snap-1"); !errors.Is(err, domain.ErrSnapshotNotFound) {
		t.Fatalf("from: got err %v, want not found", err)
	}
	if _, err := Assess(state, "payments", "prod", "snap-1", "missing"); !errors.Is(err, domain.ErrSnapshotNotFound) {
		t.Fatalf("to: got err %v, want not found", err)
	}
}

func TestAssessRejectsScopeMismatch(t *testing.T) {
	state := stateWith(
		domain.Snapshot{ID: "snap-p1", Service: "payments", Environment: "prod", CapturedAt: base, Values: map[string]any{}},
		domain.Snapshot{ID: "snap-p2", Service: "payments", Environment: "prod", CapturedAt: base.Add(time.Hour), Values: map[string]any{}},
		domain.Snapshot{ID: "snap-s1", Service: "payments", Environment: "staging", CapturedAt: base, Values: map[string]any{}},
	)
	if _, err := Assess(state, "payments", "prod", "snap-p1", "snap-s1"); !errors.Is(err, domain.ErrScopeMismatch) {
		t.Fatalf("different environments: got err %v, want scope mismatch", err)
	}
	if _, err := Assess(state, "orders", "prod", "snap-p1", "snap-p2"); !errors.Is(err, domain.ErrScopeMismatch) {
		t.Fatalf("wrong requested scope: got err %v, want scope mismatch", err)
	}
}

func TestAssessRequiresCompleteDependencies(t *testing.T) {
	state := stateWith(
		domain.Snapshot{ID: "snap-p1", Service: "payments", Environment: "prod", CapturedAt: base, Values: map[string]any{}, Dependencies: []string{"auth"}},
		domain.Snapshot{ID: "snap-p2", Service: "payments", Environment: "prod", CapturedAt: base.Add(time.Hour), Values: map[string]any{}, Dependencies: []string{"auth"}},
		domain.Snapshot{ID: "snap-a1", Service: "auth", Environment: "staging", CapturedAt: base, Values: map[string]any{}},
	)
	if _, err := Assess(state, "payments", "prod", "snap-p1", "snap-p2"); !errors.Is(err, domain.ErrIncompleteDependencies) {
		t.Fatalf("got err %v, want incomplete dependencies", err)
	}
}
