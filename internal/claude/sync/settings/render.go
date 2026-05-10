package settings

import (
	"regexp"
	"strings"

	"github.com/Nivl/config/internal/claude/sync/state"
)

// identifierRe matches the bare-identifier predicate:
// ^[a-zA-Z_][a-zA-Z_0-9]*$.
var identifierRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z_0-9]*$`)

// renderPath produces a jq-style filter string for a path. A path
// segment matching identifierRe goes through .name; anything else
// uses .["name"] with the key JSON-encoded so embedded
// quotes/specials are escaped.
//
// Examples:
//
//	["model"]                       → ".model"
//	["permissions","allow"]         → ".permissions.allow"
//	["enabledPlugins","warp@x"]     → `.enabledPlugins.["warp@x"]`
//	[""]                            → `.[""]`
func renderPath(path []string) string {
	var sb strings.Builder
	for _, seg := range path {
		if identifierRe.MatchString(seg) {
			sb.WriteByte('.')
			sb.WriteString(seg)
			continue
		}
		// JSON-encode the segment for the bracket form. Marshaling a
		// string returns it surrounded by ASCII double quotes with
		// escapes, exactly what jq's tojson would produce. Use
		// state.MarshalJq so HTML-special chars (<, >, &) in keys are
		// not \uXXXX-escaped (jq doesn't escape them).
		//
		// Emit the leading dot before every bracket step: bare
		// identifiers become ".name" and bracket segments become
		// ".[\"name\"]". The dot is always present.
		quoted, _ := state.MarshalJq(seg) // strings always marshal
		sb.WriteByte('.')
		sb.WriteByte('[')
		sb.Write(quoted)
		sb.WriteByte(']')
	}
	return sb.String()
}
