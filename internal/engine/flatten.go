package engine

import (
	"encoding/json"
)

func Flatten(values map[string]any) map[string]json.RawMessage {
	result := make(map[string]json.RawMessage)
	for key, value := range values {
		flattenValue(key, value, result)
	}
	return result
}

func flattenValue(path string, value any, result map[string]json.RawMessage) {
	if object, ok := value.(map[string]any); ok && len(object) > 0 {
		for key, nested := range object {
			child := key
			if path != "" {
				child = path + "." + key
			}
			flattenValue(child, nested, result)
		}
		return
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		encoded = []byte("null")
	}
	result[path] = encoded
}
