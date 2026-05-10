// Package userinput collects the small set of interactive prompts that
// `melvin-config setup` runs before invoking the rest of the pipeline.
// Each prompt has a strict prearg fast-path so the CLI can run
// unattended in CI / scripted contexts.
//
// Pattern (one file per prompt):
//   - Take a `prearg string` as the first argument. The cmd layer
//     resolves the value from flag → env → "" before calling; "" means
//     "neither was set." If prearg is non-empty, return it verbatim and
//     skip persistence — pre-set values pass through unvalidated.
//   - Otherwise prompt to `out` (typically stderr) and read one line
//     from `in` (typically os.Stdin). EOF returns errNoInput.
//   - Optional persistence (only Personal writes ~/.zprofile).
//
// This package never reads os env directly — that boundary lives in
// internal/cmd's resolveBool/resolveString helpers.
package userinput

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

// errNoInput is returned when stdin reaches EOF before the user makes a
// choice. Package-level sentinel so callers and tests can match it with
// errors.Is. The "No input; aborting" wording matches install.bootstrap.sh.
var errNoInput = errors.New("no input; aborting")

// firstByte returns the first byte of line (typically used for y/n /
// 1234 dispatch), or 0 when the line is empty.
func firstByte(line string) byte {
	if line == "" {
		return 0
	}
	return line[0]
}

// readLine reads one line from r, returning the line trimmed of its
// trailing "\r\n" / "\n" / "\r". Returns errNoInput when r reaches EOF
// before any byte is read.
func readLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if errors.Is(err, io.EOF) && line == "" {
		return "", errNoInput
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read input: %w", err)
	}
	return strings.TrimRight(line, "\r\n"), nil
}
