package settings

import (
	"fmt"
	"sort"
)

// Decide is the pure merge algorithm. Inputs are JSON byte payloads
// (base/local/remote can be empty or `{}`). Returns the list of
// non-noop decisions in stable path order.
//
// Granularity rule: a top-level key whose value is an object in EVERY
// side that has it is split into (topKey, subKey) units; otherwise
// the whole top-level value is the merge unit.
//
// Set-merge rule: if at least one of B/L/R is present and every
// present occurrence of the merge-unit value is a primitive array,
// the unit is set-merged.
func Decide(base, local, remote []byte) ([]Decision, error) {
	bV, err := unmarshalForMerge(base)
	if err != nil {
		return nil, fmt.Errorf("parse base: %w", err)
	}
	lV, err := unmarshalForMerge(local)
	if err != nil {
		return nil, fmt.Errorf("parse local: %w", err)
	}
	rV, err := unmarshalForMerge(remote)
	if err != nil {
		return nil, fmt.Errorf("parse remote: %w", err)
	}

	bObj, _ := bV.(map[string]any) // nil if not object — treated as empty
	lObj, _ := lV.(map[string]any)
	rObj, _ := rV.(map[string]any)

	// Top-level keys present in any side.
	topKeys := unionKeys(bObj, lObj, rObj)

	// Build the list of merge-unit paths.
	var paths [][]string
	for _, k := range topKeys {
		if shouldDrillDown(k, bObj, lObj, rObj) {
			// Sub-keys = union of keys across each side's k-value object.
			subKeys := unionKeys(
				asMap(bObj, k), asMap(lObj, k), asMap(rObj, k),
			)
			for _, sk := range subKeys {
				paths = append(paths, []string{k, sk})
			}
		} else {
			paths = append(paths, []string{k})
		}
	}

	var out []Decision
	for _, p := range paths {
		b := lookup(bObj, p)
		l := lookup(lObj, p)
		r := lookup(rObj, p)
		d := decide(b, l, r)
		if d.Action == ActionNoop {
			continue
		}
		d.Path = p
		out = append(out, d)
	}
	return out, nil
}

// decide runs the 6-branch decision tree. Branch order is
// significant.
func decide(b, l, r presence) Decision {
	// Branch 1: set-merge if at least one present and every present
	// occurrence is a primitive array.
	if eligibleForSetMerge(b, l, r) {
		return decideSet(b, l, r)
	}

	// Branch 2: equal(B,L) and equal(B,R) → noop.
	if equalPresence(b, l) && equalPresence(b, r) {
		return Decision{Action: ActionNoop}
	}

	// Branch 3: equal(B,L) (with R different).
	if equalPresence(b, l) {
		if r.present {
			return Decision{Action: ActionTakeRemote, Value: r.value}
		}
		return Decision{Action: ActionRemoteDelete}
	}

	// Branch 4: equal(B,R) (with L different).
	if equalPresence(b, r) {
		return Decision{Action: ActionKeepLocal}
	}

	// Branch 5: equal(L,R) (with B different).
	if equalPresence(l, r) {
		if l.present {
			return Decision{Action: ActionNoop}
		}
		return Decision{Action: ActionRemoteDelete}
	}

	// Branch 6: conflict.
	d := Decision{Action: ActionConflict}
	if b.present {
		v := b.value
		d.BaseValue = &v
	}
	if l.present {
		v := l.value
		d.LocalValue = &v
	}
	if r.present {
		v := r.value
		d.RemoteValue = &v
	}
	return d
}

// shouldDrillDown returns true iff every present occurrence of the
// top-level key is an object.
func shouldDrillDown(key string, b, l, r map[string]any) bool {
	anyPresent := false
	for _, side := range []map[string]any{b, l, r} {
		v, ok := side[key]
		if !ok {
			continue
		}
		if _, isObj := v.(map[string]any); !isObj {
			return false
		}
		anyPresent = true
	}
	return anyPresent
}

// unionKeys returns the sorted union of keys across the three maps.
func unionKeys(maps ...map[string]any) []string {
	seen := map[string]struct{}{}
	for _, m := range maps {
		for k := range m {
			seen[k] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// asMap returns m[key] as map[string]any, or nil if absent / wrong type.
func asMap(m map[string]any, key string) map[string]any {
	if m == nil {
		return nil
	}
	v, ok := m[key]
	if !ok {
		return nil
	}
	mm, _ := v.(map[string]any)
	return mm
}

// eligibleForSetMerge returns true when at least one of B/L/R is
// present and every present occurrence is a primitive array.
func eligibleForSetMerge(b, l, r presence) bool {
	anyPresent := false
	for _, p := range []presence{b, l, r} {
		if !p.present {
			continue
		}
		anyPresent = true
		if !isPrimArray(p.value) {
			return false
		}
	}
	return anyPresent
}

// isPrimArray returns true iff v is an array AND every element's
// type is neither object nor array. The empty array qualifies
// (universal-quantifier over empty set is vacuously true).
func isPrimArray(v any) bool {
	arr, ok := v.([]any)
	if !ok {
		return false
	}
	for _, e := range arr {
		switch e.(type) {
		case map[string]any:
			return false
		case []any:
			return false
		}
	}
	return true
}

// decideSet implements the 3-way set merge:
//
//	union     = unique(as_set(L) + as_set(R))
//	l_removes = as_set(B) − as_set(L)
//	r_removes = as_set(B) − as_set(R)
//	merged    = sort(union − l_removes − r_removes)
func decideSet(b, l, r presence) Decision {
	bSet := asSet(b)
	lSet := asSet(l)
	rSet := asSet(r)

	union := setUnion(lSet, rSet)
	lRemoves := setDifference(bSet, lSet)
	rRemoves := setDifference(bSet, rSet)
	merged := setDifference(setDifference(union, lRemoves), rRemoves)
	sortAny(merged)

	// Noop when merged == as_set(L) — no presence guard, only value
	// equality. When local is absent, as_set(L) = [] and merged = []
	// also → noop.
	if equalSet(merged, lSet) {
		return Decision{Action: ActionNoop, Kind: KindSet}
	}

	adds := len(setDifference(merged, lSet))
	removes := len(setDifference(lSet, merged))
	return Decision{
		Action:  ActionTakeRemote,
		Kind:    KindSet,
		Value:   merged,
		Adds:    adds,
		Removes: removes,
		Total:   len(merged),
	}
}

// asSet treats absent as the empty set; present primitive array as
// unique(value).
func asSet(p presence) []any {
	if !p.present {
		return []any{}
	}
	arr, _ := p.value.([]any)
	return uniqueAny(arr)
}

// uniqueAny returns the unique elements of arr, preserving first-seen
// order. We sort separately via sortAny when needed.
func uniqueAny(arr []any) []any {
	seen := map[string]struct{}{}
	out := []any{}
	for _, v := range arr {
		k := keyOf(v)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, v)
	}
	return out
}

// keyOf produces a stable string representation of a primitive value
// suitable for set membership comparison. Uses %#v for canonicality.
func keyOf(v any) string {
	return fmt.Sprintf("%T:%#v", v, v)
}

// setUnion returns unique(a + b), preserving first-seen order.
func setUnion(a, b []any) []any {
	return uniqueAny(append(append([]any{}, a...), b...))
}

// setDifference returns elements of a that are not in b.
func setDifference(a, b []any) []any {
	bset := map[string]struct{}{}
	for _, v := range b {
		bset[keyOf(v)] = struct{}{}
	}
	out := []any{}
	for _, v := range a {
		if _, ok := bset[keyOf(v)]; !ok {
			out = append(out, v)
		}
	}
	return out
}

// equalSet returns true iff a and b have the same multiset of unique elements.
func equalSet(a, b []any) bool {
	if len(a) != len(b) {
		return false
	}
	bset := map[string]int{}
	for _, v := range b {
		bset[keyOf(v)]++
	}
	for _, v := range a {
		bset[keyOf(v)]--
	}
	for _, c := range bset {
		if c != 0 {
			return false
		}
	}
	return true
}

// sortAny sorts a slice of any in jq-comparable order. jq orders by
// JSON value: null < false < true < numbers < strings < arrays < objects.
// Our merge sets only contain primitives, so we use a simpler total
// order: by type name, then by stringified value. The exact ordering
// doesn't matter for correctness (set equality is order-insensitive);
// it matters for the on-disk output to be deterministic.
func sortAny(s []any) {
	sort.SliceStable(s, func(i, j int) bool {
		return keyOf(s[i]) < keyOf(s[j])
	})
}
