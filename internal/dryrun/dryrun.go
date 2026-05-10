// Package dryrun provides the reporting interface used when
// melvin-config setup is invoked with --dry-run. Production code
// paths call a NullReporter whose methods are no-ops; dry-run runs
// swap in a real Reporter that emits per-file one-liners and unified
// diffs to a configured writer.
package dryrun

import (
	"fmt"
	"io"

	"github.com/aymanbagabas/go-udiff"
)

// Reporter receives notifications from chokepoints in setup's
// subsystems. Production carries a NullReporter; --dry-run swaps in
// the stderr-emitting impl returned by NewReporter.
type Reporter interface {
	// Section emits a header for a subsystem (e.g. "configgen").
	Section(name string)
	// FileChange records that target would change from before to
	// after; summary is a one-liner explaining the reason. The
	// reporter emits a unified diff for files that would change.
	FileChange(target string, before, after []byte, summary string)
	// FileNoOp records that target is already in sync; summary is a
	// one-liner.
	FileNoOp(target, summary string)
	// Symlink records a symlink-install decision. decision is one of
	// "no-op", "would-create", "would-back-up-then-create".
	Symlink(target, linkTo, decision string)
	// Shellout records that a side-effecting subprocess invocation
	// would run.
	Shellout(command string, args []string, summary string)
	// FinalSummary emits the end-of-run counter line.
	FinalSummary()
}

// NewReporter returns a Reporter that writes the one-liner +
// unified-diff format to out. The returned reporter tracks
// per-category counters and emits a summary line via FinalSummary.
func NewReporter(out io.Writer) Reporter {
	return &realReporter{out: out}
}

// NewNullReporter returns a no-op Reporter used by production runs
// when --dry-run is not set. Every method is a no-op; the production
// code paths can call reporter.X(...) unconditionally without
// nil-checks.
func NewNullReporter() Reporter {
	return nullReporter{}
}

// nullReporter implements Reporter as a no-op.
type nullReporter struct{}

func (nullReporter) Section(string)                            {}
func (nullReporter) FileChange(string, []byte, []byte, string) {}
func (nullReporter) FileNoOp(string, string)                   {}
func (nullReporter) Symlink(string, string, string)            {}
func (nullReporter) Shellout(string, []string, string)         {}
func (nullReporter) FinalSummary()                             {}

// realReporter implements Reporter by writing structured output to
// out. The counters tally file changes / symlinks / shellouts for
// the final summary line.
type realReporter struct {
	out               io.Writer
	filesChanged      int
	backupsScheduled  int
	symlinksScheduled int
	shelloutsReported int
}

// Section prints "== <name> ==" on its own line.
func (r *realReporter) Section(name string) {
	_, _ = fmt.Fprintf(r.out, "\n== %s ==\n", name)
}

// FileChange prints the one-liner "[dry-run] <target> — <summary>"
// followed by a unified diff produced by udiff.Unified. Fresh writes
// (empty before) use "/dev/null" as the old-name.
func (r *realReporter) FileChange(target string, before, after []byte, summary string) {
	r.filesChanged++
	_, _ = fmt.Fprintf(r.out, "[dry-run] %s — %s\n", target, summary)
	oldName := target
	if len(before) == 0 {
		oldName = "/dev/null"
	}
	diff := udiff.Unified(oldName, target+" (proposed)", string(before), string(after))
	if diff != "" {
		_, _ = fmt.Fprint(r.out, diff)
	}
}

// FileNoOp prints the one-liner "[dry-run] <target> — <summary>"
// without a diff.
func (r *realReporter) FileNoOp(target, summary string) {
	_, _ = fmt.Fprintf(r.out, "[dry-run] %s — %s\n", target, summary)
}

// Symlink prints the one-liner with the decision word and the link
// destination. Backup decisions bump the backupsScheduled counter
// for the final summary; non-no-op decisions bump
// symlinksScheduled.
func (r *realReporter) Symlink(target, linkTo, decision string) {
	if decision == "would-back-up-then-create" {
		r.backupsScheduled++
	}
	if decision != "no-op" {
		r.symlinksScheduled++
	}
	_, _ = fmt.Fprintf(r.out, "[dry-run] %s — %s -> %s\n", target, decision, linkTo)
}

// Shellout prints "[dry-run] would run: <command> <args...> — <summary>".
func (r *realReporter) Shellout(command string, args []string, summary string) {
	r.shelloutsReported++
	_, _ = fmt.Fprintf(r.out, "[dry-run] would run: %s", command)
	for _, a := range args {
		_, _ = fmt.Fprintf(r.out, " %s", a)
	}
	if summary != "" {
		_, _ = fmt.Fprintf(r.out, " — %s", summary)
	}
	_, _ = fmt.Fprintln(r.out)
}

// FinalSummary prints the counter line and a re-run hint.
func (r *realReporter) FinalSummary() {
	_, _ = fmt.Fprintf(r.out,
		"\n== summary ==\n%d files would change, %d backups, %d symlinks, %d shellouts.\nRe-run without --dry-run to apply.\n",
		r.filesChanged, r.backupsScheduled, r.symlinksScheduled, r.shelloutsReported)
}
