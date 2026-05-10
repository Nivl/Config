package settings

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
)

// presence carries lookup's result: either present:false (the path
// was absent in the input) or present:true with a value.
//
//nolint:govet // fieldalignment: bool before any keeps logical layout clear
type presence struct {
	present bool
	value   any
}

// lookup walks path one step at a time through obj. A path is
// "absent" if any intermediate object lacks the key OR any
// intermediate value is not a map[string]any.
func lookup(obj any, path []string) presence {
	if len(path) == 0 {
		return presence{present: true, value: obj}
	}
	m, ok := obj.(map[string]any)
	if !ok {
		return presence{present: false}
	}
	v, ok := m[path[0]]
	if !ok {
		return presence{present: false}
	}
	return lookup(v, path[1:])
}

// equalPresence returns true iff both presences agree on presence and,
// when both present, their values are deeply equal. JSON value equality
// uses reflect.DeepEqual which matches jq's structural equality on the
// types unmarshalForMerge produces.
func equalPresence(x, y presence) bool {
	if x.present != y.present {
		return false
	}
	if !x.present {
		return true
	}
	return reflect.DeepEqual(x.value, y.value)
}

// unmarshalForMerge parses JSON bytes into an any value, treating empty
// input as the empty object {} (caller's responsibility to normalize
// missing inputs to []byte("{}") or empty before invoking Decide).
// Uses json.Decoder with UseNumber=false (default) so numbers stay as
// float64 — matches jq's number handling for comparison purposes.
func unmarshalForMerge(data []byte) (any, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return map[string]any{}, nil
	}
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("json unmarshal: %w", err)
	}
	return v, nil
}
