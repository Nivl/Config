package userinput

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Nivl/config/internal/dryrun"
)

// Personal resolves PERSONAL_COMPUTER. Pre-resolved-value fast-path:
// when prearg is non-empty, return it verbatim and skip persistence.
// The cmd layer is responsible for resolving --personal /
// PERSONAL_COMPUTER and passing the result as prearg — this package
// does not read env. Otherwise prompt with `Is this for a personal
// computer (y/n)? `, parse the first byte (y/Y → "true", n/N →
// "false", else loop), then ensure homeDir/.zprofile contains the
// `export PERSONAL_COMPUTER=<value>` line via an append-with-grep
// guard.
//
// When dryRun is true the .zprofile append is suppressed and the
// would-be change is announced through reporter instead. The prompt
// itself still runs — the value is needed for the rest of the dry-run
// plan (it gates personal-only casks, claude sync mode, etc.).
func Personal(prearg string, in io.Reader, out io.Writer, homeDir string, dryRun bool, reporter dryrun.Reporter) (string, error) {
	if prearg != "" {
		return prearg, nil
	}

	value, err := promptYesNo(in, out, "Is this for a personal computer")
	if err != nil {
		return "", fmt.Errorf("yes-no prompt: %w", err)
	}

	if err := ensureZprofileLine(homeDir, value, dryRun, reporter); err != nil {
		return "", fmt.Errorf("ensure zprofile line: %w", err)
	}
	return value, nil
}

// promptYesNo loops on `<prompt> (y/n)? `. y/Y → "true", n/N →
// "false", anything else prints "Invalid value\n" to out and loops.
// EOF returns errNoInput.
func promptYesNo(in io.Reader, out io.Writer, prompt string) (string, error) {
	r := bufio.NewReader(in)
	for {
		if _, err := fmt.Fprintf(out, "%s (y/n)? ", prompt); err != nil {
			return "", fmt.Errorf("write prompt: %w", err)
		}
		line, err := readLine(r)
		if err != nil {
			return "", fmt.Errorf("read line: %w", err)
		}
		switch firstByte(line) {
		case 'y', 'Y':
			return "true", nil
		case 'n', 'N':
			return "false", nil
		}
		if _, err := fmt.Fprintln(out, "Invalid value"); err != nil {
			return "", fmt.Errorf("write retry hint: %w", err)
		}
	}
}

// ensureZprofileLine appends `export PERSONAL_COMPUTER=<value>` to
// homeDir/.zprofile when the file doesn't already contain that exact
// line. If the file has the line with a different value (e.g.
// "=false" but the user just answered "yes"), the new line is
// appended below — zsh sources .zprofile top-to-bottom so the later
// assignment wins. The duplicate line is intentional: an in-place
// rewrite would risk mangling unrelated user content.
//
// Uses a leading "\n" before the export so the line never ends up
// appended to an existing line without separation.
//
// Under dryRun the file write is suppressed and the would-be change
// (or no-op when the line is already present) is announced through
// reporter.
func ensureZprofileLine(homeDir, value string, dryRun bool, reporter dryrun.Reporter) error {
	zprofile := filepath.Join(homeDir, ".zprofile")
	existing, err := os.ReadFile(zprofile) //nolint:gosec // dotfile path under user-supplied homeDir is by-design
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read zprofile: %w", err)
	}

	wanted := "export PERSONAL_COMPUTER=" + value
	for line := range strings.SplitSeq(string(existing), "\n") {
		if line == wanted {
			if dryRun {
				reporter.FileNoOp(zprofile, "PERSONAL_COMPUTER already set to "+value)
			}
			return nil
		}
	}

	appended := append([]byte(nil), existing...)
	appended = append(appended, '\n')
	appended = append(appended, wanted...)
	appended = append(appended, '\n')
	if dryRun {
		reporter.FileChange(zprofile, existing, appended, "append PERSONAL_COMPUTER="+value)
		return nil
	}

	f, err := os.OpenFile(zprofile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644) //nolint:gosec // 0644 is the conventional default for shell rc files
	if err != nil {
		return fmt.Errorf("open zprofile: %w", err)
	}
	defer f.Close() //nolint:errcheck // best-effort; close error after successful append is non-fatal
	if _, err := fmt.Fprintf(f, "\nexport PERSONAL_COMPUTER=%s\n", value); err != nil {
		return fmt.Errorf("append zprofile: %w", err)
	}
	return nil
}
