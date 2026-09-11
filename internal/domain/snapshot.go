package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalidSnapshot  = errors.New("invalid snapshot")
	ErrSnapshotNotFound = errors.New("snapshot not found")
	ErrSnapshotConflict = errors.New("snapshot id already contains different data")
)

type Snapshot struct {
	ID           string         `json:"id"`
	Service      string         `json:"service"`
	Environment  string         `json:"environment"`
	CapturedAt   time.Time      `json:"captured_at"`
	Values       map[string]any `json:"values"`
	Dependencies []string       `json:"dependencies,omitempty"`
}

type SnapshotRecord struct {
	Snapshot  Snapshot  `json:"snapshot"`
	CreatedAt time.Time `json:"created_at"`
}

type AuditEvent struct {
	ID         uint64    `json:"id"`
	Kind       string    `json:"kind"`
	SnapshotID string    `json:"snapshot_id,omitempty"`
	Message    string    `json:"message"`
	At         time.Time `json:"at"`
}

type State struct {
	Revision  uint64                    `json:"revision"`
	Snapshots map[string]SnapshotRecord `json:"snapshots"`
	Latest    map[string]string         `json:"latest"`
	Audit     []AuditEvent              `json:"audit"`
}

func NewState() State {
	return State{Snapshots: make(map[string]SnapshotRecord), Latest: make(map[string]string)}
}

func (s Snapshot) Validate() error {
	if strings.TrimSpace(s.ID) == "" || strings.TrimSpace(s.Service) == "" || strings.TrimSpace(s.Environment) == "" {
		return fmt.Errorf("%w: id, service and environment are required", ErrInvalidSnapshot)
	}
	if s.CapturedAt.IsZero() {
		return fmt.Errorf("%w: captured_at is required", ErrInvalidSnapshot)
	}
	if s.Values == nil {
		return fmt.Errorf("%w: values is required", ErrInvalidSnapshot)
	}
	for _, dependency := range s.Dependencies {
		if strings.TrimSpace(dependency) == "" {
			return fmt.Errorf("%w: dependencies cannot contain empty names", ErrInvalidSnapshot)
		}
	}
	return nil
}

func ScopeKey(service, environment string) string {
	return service + "\x00" + environment
}
