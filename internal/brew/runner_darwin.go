//go:build darwin

package brew

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// escapeAppleScriptString escapes the two characters that have special
// meaning inside an AppleScript double-quoted string: backslash and
// double-quote. Backslash must be escaped first so we don't double-
// escape the backslashes we add for double-quotes.
func escapeAppleScriptString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

// IsAppRunning checks whether the named macOS app is running via
// pgrep -ix then via osascript tell application. Either positive
// result returns true. A cancelled context propagates as an error so
// the caller can stop instead of treating "killed by signal" as "not
// running" and proceeding to a destructive brew install.
func (r *runner) IsAppRunning(ctx context.Context, app string) (bool, error) {
	if err := exec.CommandContext(ctx, "pgrep", "-ix", app).Run(); err == nil {
		return true, nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return false, fmt.Errorf("is app running %s: %w", app, ctxErr)
	}
	// Escape app for safe embedding in the AppleScript string literal —
	// a cask app name with `"` or `\` would otherwise break the script
	// and osascript would silently exit non-zero (folded into "not
	// running" below).
	script := fmt.Sprintf(`tell application "%s" to if it is running then return "true"`, escapeAppleScriptString(app)) //nolint:gocritic // %q would break AppleScript string syntax
	out, err := exec.CommandContext(ctx, "osascript", "-e", script).Output()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return false, fmt.Errorf("is app running %s: %w", app, ctxErr)
	}
	if err != nil {
		// osascript exits non-zero for unknown apps; treat as not-running.
		return false, nil //nolint:nilerr // intentional: osascript error means not running
	}
	return strings.TrimSpace(string(out)) == "true", nil
}
