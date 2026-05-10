package perms

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"sort"
)

// ListName identifies one of the three permission lists. The string
// value is also the JSON key under `permissions`.
type ListName string

const (
	// ListAllow is the auto-approve list (`permissions.allow`).
	ListAllow ListName = "allow"
	// ListAsk is the prompt-each-time list (`permissions.ask`).
	ListAsk ListName = "ask"
	// ListDeny is the auto-reject list (`permissions.deny`).
	ListDeny ListName = "deny"
)

// AllLists returns the three lists in stable order. Callers use this
// when scanning across lists for cross-list conflicts.
func AllLists() []ListName {
	return []ListName{ListAllow, ListAsk, ListDeny}
}

// Settings is an in-memory mirror of shared_config/.claude/
// settings.json. It captures the entire top-level object as raw JSON
// so unknown keys (model, theme, hooks, enabledPlugins, …) round-trip
// untouched, and exposes only the permissions.{allow,ask,deny}
// arrays as typed slices for the cmd layer to mutate.
type Settings struct {
	// top stores every top-level key. The serialized "permissions"
	// value is rebuilt on Save() from the typed lists + permsOther.
	top map[string]json.RawMessage

	// lists holds the three string lists keyed by ListName.
	lists map[ListName][]string

	// permsOther captures any keys inside `permissions` that aren't
	// allow/ask/deny (e.g. `defaultMode`) so we round-trip them.
	permsOther map[string]json.RawMessage
}

// Load reads settings.json from path and returns an in-memory
// Settings. If the file does not exist or `permissions` (or any of
// the three lists) is missing, the corresponding pieces are
// bootstrapped to empty so the caller can mutate freely without
// special-casing first-run scenarios.
func Load(path string) (*Settings, error) {
	s := &Settings{
		top:        map[string]json.RawMessage{},
		lists:      map[ListName][]string{ListAllow: {}, ListAsk: {}, ListDeny: {}},
		permsOther: map[string]json.RawMessage{},
	}

	data, err := os.ReadFile(path) //nolint:gosec // caller-supplied path to a project-tracked settings file
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if errors.Is(err, os.ErrNotExist) || len(bytes.TrimSpace(data)) == 0 {
		return s, nil
	}
	if err := json.Unmarshal(data, &s.top); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	permsRaw, hasPerms := s.top["permissions"]
	if !hasPerms {
		return s, nil
	}
	var permsObj map[string]json.RawMessage
	if err := json.Unmarshal(permsRaw, &permsObj); err != nil {
		return nil, fmt.Errorf("parse permissions in %s: %w", path, err)
	}
	for key, raw := range permsObj {
		switch ListName(key) {
		case ListAllow, ListAsk, ListDeny:
			var items []string
			if err := json.Unmarshal(raw, &items); err != nil {
				return nil, fmt.Errorf("parse permissions.%s in %s: %w", key, path, err)
			}
			s.lists[ListName(key)] = items
		default:
			s.permsOther[key] = raw
		}
	}
	return s, nil
}

// List returns the current items in name. The returned slice is
// owned by Settings — do not mutate it; use SetList to replace it.
func (s *Settings) List(name ListName) []string {
	return s.lists[name]
}

// SetList replaces the items in name. The caller is responsible for
// any deduplication and ordering — Save will sort+dedupe before
// writing to disk.
func (s *Settings) SetList(name ListName, items []string) {
	s.lists[name] = items
}

// Save serializes Settings back to path with 2-space indentation,
// alphabetically-sorted map keys (stdlib default), and a trailing
// newline so the file plays nicely with line-based tools and diffs.
// Each permission list is sorted + deduplicated as a final
// normalization pass.
func (s *Settings) Save(path string) error {
	permsObj := map[string]json.RawMessage{}
	maps.Copy(permsObj, s.permsOther)
	for _, name := range AllLists() {
		items := append([]string(nil), s.lists[name]...)
		items = sortedUnique(items)
		marshaled, err := json.MarshalIndent(items, "    ", "  ")
		if err != nil {
			return fmt.Errorf("marshal permissions.%s: %w", name, err)
		}
		// Empty list → "[]" without dedicated indent. json.MarshalIndent
		// already handles this correctly.
		permsObj[string(name)] = marshaled
	}
	permsBytes, err := marshalIndentObject(permsObj, "  ", "  ")
	if err != nil {
		return fmt.Errorf("marshal permissions: %w", err)
	}
	s.top["permissions"] = permsBytes

	out, err := marshalIndentObject(s.top, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	// Normalize: marshalIndentObject inlines pre-marshaled RawMessage
	// values verbatim, which produces correct layout only when those
	// values are themselves scalars or single-line. A nested
	// multi-line permsOther value (e.g. a future Claude Code addition
	// like `additionalDirectories: [...]`) would land with jagged
	// indentation. Run the whole document through json.Indent as a
	// final pass so the on-disk layout stays consistent regardless of
	// what's nested.
	var normalized bytes.Buffer
	if err := json.Indent(&normalized, out, "", "  "); err != nil {
		return fmt.Errorf("normalize indent: %w", err)
	}
	normalized.WriteByte('\n')

	if err := os.WriteFile(path, normalized.Bytes(), 0o644); err != nil { //nolint:gosec // conventional config-file perms
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// marshalIndentObject is json.MarshalIndent for a
// map[string]json.RawMessage that produces alphabetically-keyed
// output and lets the caller control the indent prefix so we can
// nest pre-marshaled raw values without breaking the layout.
func marshalIndentObject(m map[string]json.RawMessage, prefix, indent string) ([]byte, error) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf bytes.Buffer
	if len(keys) == 0 {
		buf.WriteString("{}")
		return buf.Bytes(), nil
	}
	buf.WriteString("{\n")
	for i, k := range keys {
		buf.WriteString(prefix)
		buf.WriteString(indent)
		keyJSON, err := json.Marshal(k)
		if err != nil {
			return nil, fmt.Errorf("marshal key %q: %w", k, err)
		}
		buf.Write(keyJSON)
		buf.WriteString(": ")
		buf.Write(m[k])
		if i < len(keys)-1 {
			buf.WriteString(",")
		}
		buf.WriteString("\n")
	}
	buf.WriteString(prefix)
	buf.WriteString("}")
	return buf.Bytes(), nil
}

// sortedUnique returns a deduplicated alphabetically-sorted copy of
// in. Always returns a non-nil slice so json.Marshal renders empty
// lists as `[]` rather than `null`.
func sortedUnique(in []string) []string {
	if len(in) == 0 {
		return []string{}
	}
	out := append([]string(nil), in...)
	sort.Strings(out)
	w := 0
	for r := range out {
		if r > 0 && out[r] == out[r-1] {
			continue
		}
		out[w] = out[r]
		w++
	}
	return out[:w]
}
