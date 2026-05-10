package settings

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Apply applies a list of Decisions to the local JSON bytes, returning
// the new local bytes. Conflicts (Action=Conflict) MUST have been
// resolved into TakeRemote/KeepLocal/RemoteDelete by the caller before
// Apply — passing an unresolved Conflict panics.
//
// Preserves the rest of local untouched at the structural level.
// Output is pretty-printed (2-space indent, alphabetic object keys,
// trailing newline) so `~/.claude/settings.json` is line-stable and
// editor/git diffs stay clean.
func Apply(local []byte, decisions []Decision) ([]byte, error) {
	var doc any
	if len(local) == 0 {
		doc = map[string]any{}
	} else if err := json.Unmarshal(local, &doc); err != nil {
		return nil, fmt.Errorf("parse local: %w", err)
	}

	for _, d := range decisions {
		switch d.Action {
		case ActionTakeRemote:
			doc = setPath(doc, d.Path, d.Value)
		case ActionRemoteDelete:
			doc = delPath(doc, d.Path)
		case ActionNoop, ActionKeepLocal:
			// no-op
		case ActionConflict:
			panic(fmt.Sprintf("settings.Apply: unresolved conflict at %v", d.Path))
		}
	}

	// Use a json.Encoder with HTML escaping OFF so strings containing
	// `<`, `>`, or `&` are emitted verbatim. The default MarshalIndent
	// would emit escaped forms, which would diff-churn every
	// `Bash(... > foo)` permission against the canonicalize.jq pass
	// run by the pre-commit hook. Encode appends a trailing newline.
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return nil, fmt.Errorf("marshal merged: %w", err)
	}
	return buf.Bytes(), nil
}

// setPath sets doc[path] = value, creating intermediate map objects as
// needed. If an intermediate is not a map, it is replaced with one.
// Mirrors jq's `setpath($p; $v)`.
func setPath(doc any, path []string, value any) any {
	if len(path) == 0 {
		return value
	}
	m, ok := doc.(map[string]any)
	if !ok {
		m = map[string]any{}
	}
	m[path[0]] = setPath(m[path[0]], path[1:], value)
	return m
}

// delPath removes the leaf at path. Intermediate non-map values are
// left alone. Mirrors jq's `delpaths([$p])`.
func delPath(doc any, path []string) any {
	if len(path) == 0 {
		return nil
	}
	m, ok := doc.(map[string]any)
	if !ok {
		return doc
	}
	if len(path) == 1 {
		delete(m, path[0])
		return m
	}
	if child, exists := m[path[0]]; exists {
		m[path[0]] = delPath(child, path[1:])
	}
	return m
}
