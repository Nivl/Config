package state

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// MarshalJq returns the compact JSON encoding of v with HTML escaping
// disabled, matching jq's tojson semantics. Used wherever output must
// be byte-stable (decisions.json cache keys, path-filter rendering,
// conflict prompt values). The default json.Marshal escapes `<`, `>`,
// `&` to their `\uXXXX` form; we want the unescaped form here.
func MarshalJq(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	// json.Encoder.Encode always appends '\n'; callers that need it
	// can re-append, but the natural use-site here is inline output.
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}
