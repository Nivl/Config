package configgen

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/Nivl/config/internal/dryrun"
	"github.com/Nivl/config/internal/managedblock"
)

// gitconfigPriorLine matches the two-line [include] block as
// previously written by SetupGitconfig (outside any managed block).
// Used by managedblock.Upsert for one-shot migration of pre-marker
// files. Matches both lines as a pair to avoid leaving an orphan
// [include] header behind.
var gitconfigPriorLine = regexp.MustCompile( //nolint:gochecknoglobals // immutable regex
	`(?m)^\[include\]\s*\n\s*path\s*=\s*"[^"]*\.melvin/config/.*"\s*$`,
)

// buildGitconfigIdentity returns the [user] + [url] + [commit] block
// previously written inline by writeGitconfigIdentityIfAbsent. It is
// pure so the dry-run composition path can invoke it without touching
// disk.
func buildGitconfigIdentity(personalComputer bool) string {
	var user, commit string
	if personalComputer {
		user = "[user]\n\temail = noreply@melvin.la" +
			"\n\tname = Melvin" +
			"\n\tsigningkey = 2C307E0D0413344B"
		commit = "[commit]" +
			"\n\tgpgsign = true"
	} else {
		user = "[user]\n\temail = melvin@domain.tld" +
			"\n\tname = Melvin" +
			"\n\t# signingkey = <key>"
		commit = "[commit]" +
			"\n\tgpgsign = false"
	}
	urlComment := "# [url \"ssh://git@github.com/\"]" +
		"\n\t# insteadOf = https://github.com/"
	return user + "\n\n" + urlComment + "\n\n" + commit + "\n"
}

// SetupGitconfig materializes ~/.gitconfig:
//
//   - First-install only (file absent): write a per-machine [user]
//     block (email/name plus a signingkey on PERSONAL_COMPUTER=true,
//     or a commented stub otherwise), a commented [url] template,
//     and a [commit] block with the appropriate gpgsign value.
//
//   - Every run: upsert the managed-block region containing the
//     [include] two-liner that pulls in shared_config/.gitconfig
//     via managedblock.Upsert.
//
// When dryRun is true, no disk writes occur. Instead,
// setupGitconfigDryRun composes the full would-be file content in
// memory and reports a single FileChange (or FileNoOp) via reporter.
//
// configDir is unused but retained in the signature for symmetry
// with the other configgen functions.
func SetupGitconfig(homeDir, _ string, personalComputer, dryRun bool, reporter dryrun.Reporter) error {
	target := filepath.Join(homeDir, ".gitconfig")

	if dryRun {
		return setupGitconfigDryRun(target, homeDir, personalComputer, reporter)
	}

	if err := writeGitconfigIdentityIfAbsent(target, personalComputer); err != nil {
		return err
	}

	payload := fmt.Sprintf("[include]\n\tpath = \"%s/.melvin/config/shared_config/.gitconfig\"", homeDir)
	if err := managedblock.Upsert(target, payload, gitconfigPriorLine, managedblock.UpsertOpts{Reporter: reporter}); err != nil {
		return fmt.Errorf("upsert managed block in %s: %w", target, err)
	}
	return nil
}

// setupGitconfigDryRun computes the proposed end state of ~/.gitconfig
// by composing phase-1 identity block (when the file is absent) with
// phase-2's managed-block region. Reports a single FileChange (or
// FileNoOp) against the real on-disk content.
//
// Phase-1 is gated on the file's *existence* (same as
// writeGitconfigIdentityIfAbsent), not on len > 0 — an existing empty
// ~/.gitconfig means the live path will not write the identity block.
func setupGitconfigDryRun(target, homeDir string, personalComputer bool, reporter dryrun.Reporter) error {
	_, statErr := os.Stat(target)
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("stat %s: %w", target, statErr)
	}
	freshInstall := errors.Is(statErr, os.ErrNotExist)

	realExisting, _ := os.ReadFile(target) //nolint:gosec // caller-supplied home-relative config-file path, read-by-design
	virtualExisting := realExisting
	if freshInstall {
		virtualExisting = []byte(buildGitconfigIdentity(personalComputer))
	}
	payload := fmt.Sprintf("[include]\n\tpath = \"%s/.melvin/config/shared_config/.gitconfig\"", homeDir)
	proposed, err := managedblock.ComputeNext(
		string(virtualExisting), payload, gitconfigPriorLine, target,
	)
	if err != nil {
		return err
	}
	if proposed == string(realExisting) {
		reporter.FileNoOp(target, "gitconfig in sync")
		return nil
	}
	summary := "would update gitconfig"
	if freshInstall {
		summary = "would create gitconfig (fresh install)"
	}
	reporter.FileChange(target, realExisting, []byte(proposed), summary)
	return nil
}

// writeGitconfigIdentityIfAbsent writes the per-machine identity +
// [url] stub + [commit] block when target is absent. Existing files
// short-circuit with nil. The [include] section is no longer
// written here — it lives inside the managed block now.
func writeGitconfigIdentityIfAbsent(target string, personalComputer bool) error {
	if _, err := os.Stat(target); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat %s: %w", target, err)
	}

	content := buildGitconfigIdentity(personalComputer)
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil { //nolint:gosec // 0644 conventional umask write
		return fmt.Errorf("write %s: %w", target, err)
	}
	return nil
}
