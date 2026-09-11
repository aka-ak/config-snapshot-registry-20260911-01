package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"example.com/config-snapshot-registry/internal/domain"
	"example.com/config-snapshot-registry/internal/store"
)

func TestPutSnapshotIsIdempotentAndTracksLatest(t *testing.T) {
	svc := New(store.NewMemory())
	first := domain.Snapshot{ID: "snap-1", Service: "payments", Environment: "prod", CapturedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Values: map[string]any{"timeout": 3}}
	created, err := svc.PutSnapshot(context.Background(), first)
	if err != nil || !created.Created {
		t.Fatalf("first put: result=%#v err=%v", created, err)
	}
	replayed, err := svc.PutSnapshot(context.Background(), first)
	if err != nil || replayed.Created || replayed.Record.Snapshot.ID != "snap-1" {
		t.Fatalf("replay: result=%#v err=%v", replayed, err)
	}
	conflict := first
	conflict.Values = map[string]any{"timeout": 9}
	if _, err := svc.PutSnapshot(context.Background(), conflict); !errors.Is(err, domain.ErrSnapshotConflict) {
		t.Fatalf("got err %v, want conflict", err)
	}
}

func TestDiffUsesStoredSnapshots(t *testing.T) {
	svc := New(store.NewMemory())
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, snapshot := range []domain.Snapshot{
		{ID: "snap-1", Service: "catalog", Environment: "staging", CapturedAt: base, Values: map[string]any{"feature": map[string]any{"enabled": false}}},
		{ID: "snap-2", Service: "catalog", Environment: "staging", CapturedAt: base.Add(time.Hour), Values: map[string]any{"feature": map[string]any{"enabled": true}}},
	} {
		if _, err := svc.PutSnapshot(context.Background(), snapshot); err != nil {
			t.Fatal(err)
		}
	}
	diff, err := svc.Diff(context.Background(), "snap-1", "snap-2")
	if err != nil || len(diff.Changes) != 1 || diff.Changes[0].Path != "feature.enabled" {
		t.Fatalf("diff=%#v err=%v", diff, err)
	}
}
