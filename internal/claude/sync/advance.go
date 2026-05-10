package sync

import (
	"context"
	"fmt"

	"github.com/Nivl/config/internal/claude/sync/state"
	"github.com/Nivl/config/internal/dryrun"
)

// AdvanceLastSyncCommit overwrites paths.LastSyncFile with the
// current HEAD SHA. Caller is responsible for the HadSkips gate —
// only call this when HadSkips == false. The HadSkips==true warning
// is owned by the caller in Sync.
//
// When dryRun is true the write is suppressed; reporter.FileChange is
// called unconditionally so dry-run output is complete.
func AdvanceLastSyncCommit(ctx context.Context, paths state.Paths,
	git state.Git, dryRun bool, reporter dryrun.Reporter,
) error {
	sha, err := git.HeadSHA(ctx)
	if err != nil {
		return fmt.Errorf("head sha: %w", err)
	}

	existing, _ := state.ReadLastSyncSHA(paths) // empty on absence; whitespace-trimmed
	if existing == sha {
		reporter.FileNoOp(paths.LastSyncFile, "last-sync-commit already at HEAD")
		return nil
	}
	var before []byte
	if existing != "" {
		before = []byte(existing + "\n")
	}
	reporter.FileChange(paths.LastSyncFile, before, []byte(sha+"\n"),
		"advance last-sync-commit to HEAD SHA")
	if dryRun {
		return nil
	}

	if err := state.WriteLastSyncSHA(paths, sha); err != nil {
		return fmt.Errorf("write last-sync sha: %w", err)
	}
	return nil
}
