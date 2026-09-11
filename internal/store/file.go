package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"example.com/config-snapshot-registry/internal/domain"
)

type File struct {
	mu   sync.Mutex
	path string
}

func NewFile(path string) *File {
	return &File{path: path}
}

func (f *File) Read(_ context.Context) (domain.State, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.readUnlocked()
}

func (f *File) CompareAndSwap(_ context.Context, expected uint64, next domain.State) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	current, err := f.readUnlocked()
	if err != nil {
		return err
	}
	if current.Revision != expected {
		return ErrConflict
	}
	if err := os.MkdirAll(filepath.Dir(f.path), 0o755); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}
	data, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}
	temporary := f.path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return fmt.Errorf("write temporary state: %w", err)
	}
	if err := os.Rename(temporary, f.path); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("replace state: %w", err)
	}
	return nil
}

func (f *File) readUnlocked() (domain.State, error) {
	data, err := os.ReadFile(f.path)
	if errors.Is(err, os.ErrNotExist) {
		return domain.NewState(), nil
	}
	if err != nil {
		return domain.State{}, fmt.Errorf("read state: %w", err)
	}
	var state domain.State
	if err := json.Unmarshal(data, &state); err != nil {
		return domain.State{}, fmt.Errorf("decode state: %w", err)
	}
	if state.Snapshots == nil {
		state.Snapshots = make(map[string]domain.SnapshotRecord)
	}
	if state.Latest == nil {
		state.Latest = make(map[string]string)
	}
	return state, nil
}
