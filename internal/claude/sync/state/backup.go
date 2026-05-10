package state

import (
	"fmt"
	"io"
	"os"
	"time"
)

// BackupFile writes a copy of path to "<path>.<YYYYMMDDHHMMSS>.bkp"
// where the timestamp is the current local time. No-op if path doesn't
// exist.
func BackupFile(path string) error {
	return BackupFileAt(path, time.Now())
}

// BackupFileAt is the testable variant — accepts an explicit
// timestamp. The timestamp is local time (system zone), not UTC.
func BackupFileAt(path string, now time.Time) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	dst := path + "." + now.Format("20060102150405") + ".bkp"
	return copyFileWithMode(path, dst, info.Mode().Perm())
}

// copyFileWithMode copies src to dst, preserving the given mode bits.
// Ownership and timestamps are not preserved.
func copyFileWithMode(src, dst string, mode os.FileMode) (err error) {
	in, err := os.Open(src) //nolint:gosec // path is a sync-managed local file
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer func() {
		if cerr := in.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("close source: %w", cerr)
		}
	}()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode) //nolint:gosec // dst is a sync-managed local file path derived from the source
	if err != nil {
		return fmt.Errorf("create backup: %w", err)
	}
	defer func() {
		if cerr := out.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("close backup: %w", cerr)
		}
	}()
	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy bytes: %w", err)
	}
	return nil
}
