package state

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Nivl/config/internal/errutil"
)

// ReadLastSyncSHA returns the SHA in LastSyncFile, or "" if missing
// or empty. Trims trailing whitespace/newline so callers get the
// bare SHA. An empty SHA signals "no prior sync" — callers treat
// the base as missing.
func ReadLastSyncSHA(p Paths) (string, error) {
	data, err := os.ReadFile(p.LastSyncFile)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read last-sync-commit: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// WriteLastSyncSHA writes sha to LastSyncFile atomically (tempfile +
// rename). The on-disk content is the bare SHA followed by a single
// newline, matching what `git rev-parse HEAD > file` produces.
func WriteLastSyncSHA(p Paths, sha string) (err error) {
	tmp, err := os.CreateTemp(filepath.Dir(p.LastSyncFile), "last-sync-*.tmp")
	if err != nil {
		return fmt.Errorf("create tempfile: %w", err)
	}
	defer errutil.RunAndSetError(
		func() error { return CleanupTempFile(tmp) },
		&err, "cleanup tempfile",
	)
	if _, err := tmp.WriteString(sha + "\n"); err != nil {
		return fmt.Errorf("write SHA: %w", err)
	}
	// os.CreateTemp produces 0o600; we want 0o644 (umask default for
	// shell writes) so a `ls -l` on the state dir is consistent.
	if err := tmp.Chmod(0o644); err != nil {
		return fmt.Errorf("chmod tempfile: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close tempfile: %w", err)
	}
	if err := os.Rename(tmp.Name(), p.LastSyncFile); err != nil {
		return fmt.Errorf("rename to last-sync-commit: %w", err)
	}
	return nil
}
