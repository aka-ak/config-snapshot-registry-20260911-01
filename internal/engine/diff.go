package engine

import (
	"bytes"
	"encoding/json"
	"sort"
)

type ChangeKind string

const (
	Added    ChangeKind = "added"
	Removed  ChangeKind = "removed"
	Modified ChangeKind = "modified"
)

type Change struct {
	Path   string     `json:"path"`
	Kind   ChangeKind `json:"kind"`
	Before any        `json:"before,omitempty"`
	After  any        `json:"after,omitempty"`
}

func Compare(before, after map[string]any) []Change {
	left, right := Flatten(before), Flatten(after)
	paths := make(map[string]struct{}, len(left)+len(right))
	for path := range left {
		paths[path] = struct{}{}
	}
	for path := range right {
		paths[path] = struct{}{}
	}
	ordered := make([]string, 0, len(paths))
	for path := range paths {
		ordered = append(ordered, path)
	}
	sort.Strings(ordered)

	changes := make([]Change, 0)
	for _, path := range ordered {
		oldValue, hadOld := left[path]
		newValue, hadNew := right[path]
		switch {
		case !hadOld:
			changes = append(changes, Change{Path: path, Kind: Added, After: decode(newValue)})
		case !hadNew:
			changes = append(changes, Change{Path: path, Kind: Removed, Before: decode(oldValue)})
		case !bytes.Equal(oldValue, newValue):
			changes = append(changes, Change{Path: path, Kind: Modified, Before: decode(oldValue), After: decode(newValue)})
		}
	}
	return changes
}

func decode(raw json.RawMessage) any {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil
	}
	return value
}
