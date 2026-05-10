package prompt

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/Nivl/config/internal/claude/sync/state"
	"github.com/Nivl/config/internal/dryrun"
)

// errNoInput is returned when stdin reaches EOF before the user makes
// a choice. It is a package-level sentinel so callers and tests can
// match it with errors.Is.
var errNoInput = errors.New("no input; aborting conflict resolution")

// Renderer renders the conflict's human-readable details on demand.
// Summary is shown immediately (above the 6-option menu); Diff is
// shown only when the user picks option 3.
//
// The settings.Renderer implementation MUST include the short base
// SHA in Summary's output, e.g.:
//
//	base   (sha 5aff5c0): {"model":"opus"}
//	local                : {"model":"haiku"}
//	remote               : {"model":"sonnet"}
//
// On first sync (no anchor SHA), render the literal "none".
type Renderer interface {
	Summary(w io.Writer)
	Diff(w io.Writer)
}

// Request bundles everything Resolve needs for one conflict.
//
//nolint:govet // fieldalignment: logical grouping takes priority over struct packing
type Request struct {
	// Kind selects which sub-object of decisions.json the cache lookup
	// hits.
	Kind Kind
	// Key is the canonical identifier. For settings, it is the
	// compact-JSON path array (e.g. `["permissions","allow"]`). For
	// files, it is the relative path under .claude/.
	Key string
	// Header is the headline shown above the details and menu, e.g.
	// "Conflict in settings.json at .permissions.allow".
	Header string
	// Renderer produces the details lazily.
	Renderer Renderer
}

// Prompter is the public interface; tests inject FakePrompter.
type Prompter interface {
	// Resolve returns the chosen Choice. May return an error on EOF or
	// on I/O failures writing the menu / persisting a remembered choice.
	Resolve(ctx context.Context, req Request) (Choice, error)
	// Remember persists a "always-keep-local" or "always-take-remote"
	// choice to the decisions cache. Idempotent.
	Remember(kind Kind, key string, c Choice) error
}

// NewPrompter returns the production Prompter. mergeResolution is the
// caller-resolved override value (one of "keep-local" / "take-remote"
// / "skip", or empty for "no override; consult decisions cache and
// prompt"). The cmd layer resolves this from --merge-resolution /
// CLAUDE_MERGE_RESOLUTION before constructing the prompter; the
// prompt package itself does not read env. Stdout/stderr writers are
// injected for testability.
//
// When dryRun is true, Remember suppresses writes to decisions.json.
// reporter.FileChange is called unconditionally in Remember so dry-run
// output is complete.
func NewPrompter(paths state.Paths, in io.Reader, out io.Writer, mergeResolution string,
	dryRun bool, reporter dryrun.Reporter) Prompter {
	return &realPrompter{
		paths:           paths,
		in:              bufio.NewReader(in),
		out:             out,
		mergeResolution: mergeResolution,
		dryRun:          dryRun,
		reporter:        reporter,
	}
}

// realPrompter is the production Prompter implementation. Unexported
// so consumers always go through the Prompter interface.
type realPrompter struct {
	paths    state.Paths
	in       *bufio.Reader
	out      io.Writer
	reporter dryrun.Reporter
	// mergeResolution is the pre-resolved override; empty means "no
	// override, fall through to decisions cache + interactive prompt."
	mergeResolution string
	dryRun          bool
}

// Resolve implements Prompter via three-tier resolution: caller
// override, decisions cache, then interactive prompt.
func (p *realPrompter) Resolve(_ context.Context, req Request) (Choice, error) {
	// Tier 1: caller-supplied override.
	if c, ok := parseMergeResolution(p.mergeResolution); ok {
		fmt.Fprintf(p.out, "%s -> %s (override)\n", req.Header, c.String())
		return c, nil
	}

	// Tier 2: decisions cache. On corrupt decisions.json, fall
	// through to interactive — ignore the error.
	if d, err := state.LoadDecisions(p.paths); err == nil {
		var cached state.Choice
		switch req.Kind {
		case KindSettings:
			cached = d.Settings[req.Key]
		case KindFiles:
			cached = d.Files[req.Key]
		}
		switch cached {
		case state.ChoiceLocal:
			fmt.Fprintf(p.out, "%s -> keep-local (remembered)\n", req.Header)
			return ChoiceKeepLocal, nil
		case state.ChoiceRemote:
			fmt.Fprintf(p.out, "%s -> take-remote (remembered)\n", req.Header)
			return ChoiceTakeRemote, nil
		}
	}

	// Tier 3: interactive prompt.
	return p.promptInteractive(req)
}

// promptInteractive shows the 6-option menu and loops until a terminal
// choice. EOF on stdin returns an error (avoids an infinite loop on
// closed stdin).
func (p *realPrompter) promptInteractive(req Request) (Choice, error) {
	for {
		// Print the header + details + menu.
		fmt.Fprintln(p.out)
		fmt.Fprintln(p.out, req.Header)
		req.Renderer.Summary(p.out)
		fmt.Fprintln(p.out)
		fmt.Fprintln(p.out, "  1) Keep local")
		fmt.Fprintln(p.out, "  2) Take remote")
		fmt.Fprintln(p.out, "  3) View diff")
		fmt.Fprintln(p.out, "  4) Keep local AND remember")
		fmt.Fprintln(p.out, "  5) Take remote AND remember")
		fmt.Fprintln(p.out, "  6) Skip (do not advance last-sync-commit)")
		fmt.Fprintf(p.out, "  (clear remembered choices: rm %s)\n", p.paths.DecisionsFile)

		line, err := p.in.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) && line == "" {
				return ChoiceSkip, errNoInput
			}
			if !errors.Is(err, io.EOF) {
				return ChoiceSkip, fmt.Errorf("read input: %w", err)
			}
		}

		// Take the first non-whitespace byte so a leading space or
		// tab before a digit still selects the option.
		first := firstByte(line)
		switch first {
		case '1':
			return ChoiceKeepLocal, nil
		case '2':
			return ChoiceTakeRemote, nil
		case '3':
			req.Renderer.Diff(p.out)
			// loop
		case '4':
			if err := p.Remember(req.Kind, req.Key, ChoiceKeepLocal); err != nil {
				return ChoiceSkip, fmt.Errorf("remember keep-local: %w", err)
			}
			return ChoiceKeepLocal, nil
		case '5':
			if err := p.Remember(req.Kind, req.Key, ChoiceTakeRemote); err != nil {
				return ChoiceSkip, fmt.Errorf("remember take-remote: %w", err)
			}
			return ChoiceTakeRemote, nil
		case '6':
			return ChoiceSkip, nil
		default:
			fmt.Fprintln(p.out, "Invalid value")
		}
	}
}

// firstByte returns the first non-whitespace byte of line, or 0 if
// line is empty/whitespace.
func firstByte(line string) byte {
	for i := range len(line) {
		if line[i] != ' ' && line[i] != '\t' && line[i] != '\n' && line[i] != '\r' {
			return line[i]
		}
	}
	return 0
}

// Remember implements Prompter via an atomic update of
// decisions.json. When dryRun is true the write is suppressed;
// reporter.FileChange is called unconditionally so dry-run output is
// complete.
func (p *realPrompter) Remember(kind Kind, key string, c Choice) error {
	d, err := state.LoadDecisions(p.paths)
	if err != nil {
		return fmt.Errorf("load decisions: %w", err)
	}
	if d.Version == 0 {
		d.Version = 1
	}
	var stored state.Choice
	switch c {
	case ChoiceKeepLocal:
		stored = state.ChoiceLocal
	case ChoiceTakeRemote:
		stored = state.ChoiceRemote
	case ChoiceSkip:
		// Skip is never cached — treat as a no-op.
		return nil
	}
	switch kind {
	case KindSettings:
		d.Settings[key] = stored
	case KindFiles:
		d.Files[key] = stored
	}

	// Read existing bytes for the reporter diff.
	existing, _ := os.ReadFile(p.paths.DecisionsFile)
	newData, _ := state.MarshalJq(d)
	newData = append(newData, '\n')
	p.reporter.FileChange(p.paths.DecisionsFile, existing, newData, "persist remembered conflict decision")
	if p.dryRun {
		return nil
	}
	return state.SaveDecisions(p.paths, d)
}
