package engine

import "testing"

func TestCompareNestedValuesInStableOrder(t *testing.T) {
	changes := Compare(
		map[string]any{"database": map[string]any{"host": "old", "pool": map[string]any{"max": 5}}, "keep": true},
		map[string]any{"database": map[string]any{"host": "new", "pool": map[string]any{"max": 10}}, "added": 1},
	)
	if len(changes) != 4 {
		t.Fatalf("got %d changes, want 4: %#v", len(changes), changes)
	}
	if changes[0].Path != "added" || changes[1].Path != "database.host" || changes[2].Path != "database.pool.max" || changes[3].Path != "keep" {
		t.Fatalf("unexpected order: %#v", changes)
	}
}
