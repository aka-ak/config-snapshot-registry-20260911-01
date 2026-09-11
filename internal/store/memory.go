package store

import (
	"context"
	"encoding/json"
	"sync"

	"example.com/config-snapshot-registry/internal/domain"
)

type Memory struct {
	mu    sync.RWMutex
	state domain.State
}

func NewMemory() *Memory {
	return &Memory{state: domain.NewState()}
}

func (m *Memory) Read(context.Context) (domain.State, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneState(m.state)
}

func (m *Memory) CompareAndSwap(_ context.Context, expected uint64, next domain.State) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state.Revision != expected {
		return ErrConflict
	}
	m.state, _ = cloneState(next)
	return nil
}

func cloneState(input domain.State) (domain.State, error) {
	data, err := json.Marshal(input)
	if err != nil {
		return domain.State{}, err
	}
	var output domain.State
	if err := json.Unmarshal(data, &output); err != nil {
		return domain.State{}, err
	}
	if output.Snapshots == nil {
		output.Snapshots = make(map[string]domain.SnapshotRecord)
	}
	if output.Latest == nil {
		output.Latest = make(map[string]string)
	}
	return output, nil
}
