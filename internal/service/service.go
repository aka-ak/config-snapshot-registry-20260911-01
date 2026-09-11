package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"example.com/config-snapshot-registry/internal/domain"
	"example.com/config-snapshot-registry/internal/engine"
	"example.com/config-snapshot-registry/internal/impact"
	"example.com/config-snapshot-registry/internal/store"
)

type Service struct {
	store store.Store
	now   func() time.Time
}

type PutResult struct {
	Record  domain.SnapshotRecord `json:"record"`
	Created bool                  `json:"created"`
}

type ListOptions struct {
	Service     string
	Environment string
}

type Diff struct {
	From    domain.SnapshotRecord `json:"from"`
	To      domain.SnapshotRecord `json:"to"`
	Changes []engine.Change       `json:"changes"`
}

func New(st store.Store) *Service {
	return &Service{store: st, now: time.Now}
}

func (s *Service) PutSnapshot(ctx context.Context, input domain.Snapshot) (PutResult, error) {
	if err := input.Validate(); err != nil {
		return PutResult{}, err
	}
	input = cloneSnapshot(input)
	var result PutResult
	err := s.update(ctx, func(state *domain.State) (bool, error) {
		if existing, ok := state.Snapshots[input.ID]; ok {
			if sameSnapshot(existing.Snapshot, input) {
				result = PutResult{Record: existing, Created: false}
				return false, nil
			}
			return false, domain.ErrSnapshotConflict
		}
		record := domain.SnapshotRecord{Snapshot: input, CreatedAt: s.now().UTC()}
		state.Snapshots[input.ID] = record
		key := domain.ScopeKey(input.Service, input.Environment)
		if previousID, ok := state.Latest[key]; !ok || !newerThan(input, state.Snapshots[previousID].Snapshot) {
			state.Latest[key] = input.ID
		}
		state.Audit = append(state.Audit, domain.AuditEvent{
			ID: nextAuditID(state), Kind: "snapshot.created", SnapshotID: input.ID,
			Message: "snapshot registered", At: s.now().UTC(),
		})
		result = PutResult{Record: record, Created: true}
		return true, nil
	})
	return result, err
}

func (s *Service) GetSnapshot(ctx context.Context, id string) (domain.SnapshotRecord, error) {
	state, err := s.store.Read(ctx)
	if err != nil {
		return domain.SnapshotRecord{}, err
	}
	record, ok := state.Snapshots[id]
	if !ok {
		return domain.SnapshotRecord{}, domain.ErrSnapshotNotFound
	}
	return record, nil
}

func (s *Service) ListSnapshots(ctx context.Context, options ListOptions) ([]domain.SnapshotRecord, error) {
	state, err := s.store.Read(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]domain.SnapshotRecord, 0)
	for _, record := range state.Snapshots {
		if options.Service != "" && record.Snapshot.Service != options.Service {
			continue
		}
		if options.Environment != "" && record.Snapshot.Environment != options.Environment {
			continue
		}
		result = append(result, record)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Snapshot.CapturedAt.After(result[j].Snapshot.CapturedAt)
	})
	return result, nil
}

func (s *Service) Diff(ctx context.Context, fromID, toID string) (Diff, error) {
	state, err := s.store.Read(ctx)
	if err != nil {
		return Diff{}, err
	}
	from, ok := state.Snapshots[fromID]
	if !ok {
		return Diff{}, fmt.Errorf("from: %w", domain.ErrSnapshotNotFound)
	}
	to, ok := state.Snapshots[toID]
	if !ok {
		return Diff{}, fmt.Errorf("to: %w", domain.ErrSnapshotNotFound)
	}
	return Diff{From: from, To: to, Changes: engine.Compare(from.Snapshot.Values, to.Snapshot.Values)}, nil
}

func (s *Service) AssessImpact(ctx context.Context, serviceName, environment, fromID, toID string) (impact.Assessment, error) {
	state, err := s.store.Read(ctx)
	if err != nil {
		return impact.Assessment{}, err
	}
	return impact.Assess(state, serviceName, environment, fromID, toID)
}

func (s *Service) Audit(ctx context.Context) ([]domain.AuditEvent, error) {
	state, err := s.store.Read(ctx)
	if err != nil {
		return nil, err
	}
	return append([]domain.AuditEvent(nil), state.Audit...), nil
}

func (s *Service) PruneBefore(ctx context.Context, cutoff time.Time) error {
	return s.update(ctx, func(state *domain.State) (bool, error) {
		changed := false
		for id, record := range state.Snapshots {
			if !record.CreatedAt.Before(cutoff) {
				continue
			}
			key := domain.ScopeKey(record.Snapshot.Service, record.Snapshot.Environment)
			if state.Latest[key] == id {
				continue
			}
			delete(state.Snapshots, id)
			state.Audit = append(state.Audit, domain.AuditEvent{
				ID: nextAuditID(state), Kind: "snapshot.pruned", SnapshotID: id,
				Message: "snapshot removed by retention policy", At: s.now().UTC(),
			})
			changed = true
		}
		return changed, nil
	})
}

func (s *Service) update(ctx context.Context, fn func(*domain.State) (bool, error)) error {
	for attempt := 0; attempt < 4; attempt++ {
		state, err := s.store.Read(ctx)
		if err != nil {
			return err
		}
		changed, err := fn(&state)
		if err != nil {
			return err
		}
		if !changed {
			return nil
		}
		expected := state.Revision
		state.Revision++
		if err := s.store.CompareAndSwap(ctx, expected, state); err != nil {
			if errors.Is(err, store.ErrConflict) {
				continue
			}
			return err
		}
		return nil
	}
	return store.ErrConflict
}

func nextAuditID(state *domain.State) uint64 {
	if len(state.Audit) == 0 {
		return 1
	}
	return state.Audit[len(state.Audit)-1].ID + 1
}

func newerThan(candidate, existing domain.Snapshot) bool {
	return candidate.CapturedAt.After(existing.CapturedAt)
}

func sameSnapshot(left, right domain.Snapshot) bool {
	leftBytes, _ := json.Marshal(left)
	rightBytes, _ := json.Marshal(right)
	return string(leftBytes) == string(rightBytes)
}

func cloneSnapshot(input domain.Snapshot) domain.Snapshot {
	data, _ := json.Marshal(input)
	var output domain.Snapshot
	_ = json.Unmarshal(data, &output)
	return output
}
