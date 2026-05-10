// Package iox provides a small bundle for passing the standard
// stdin/stdout/stderr triple through the codebase as one value. The
// goal is to keep the process-global os.Stdin/os.Stdout/os.Stderr
// references confined to the main binary + this package, so deeper
// layers can be redirected by callers (tests, future non-TTY modes)
// without reaching for package os.
package iox

import (
	"io"
	"os"
)

// Streams is the bundle of standard I/O streams a subsystem may need
// to wire into subprocesses, prompts, or progress writers. It's a
// plain value type so callers can compose it freely (e.g. swap Err to
// an io.MultiWriter for tests) without going through a setter.
type Streams struct {
	// In is the input reader. Use io.NopCloser(strings.NewReader("")) or
	// similar in tests that don't expect any reads.
	In io.Reader
	// Out is the primary writer (subcommand progress, brew output, etc).
	Out io.Writer
	// Err is the diagnostic writer (warnings, prompts, captured stderr
	// passthrough).
	Err io.Writer
}

// System returns the process's real OS streams. This is the only place
// outside cmd/melvin-config that should touch os.Stdin/Stdout/Stderr;
// every other package takes a Streams value as an input.
func System() Streams {
	return Streams{In: os.Stdin, Out: os.Stdout, Err: os.Stderr}
}
