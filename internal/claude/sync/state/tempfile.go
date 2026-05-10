package state

import (
	"errors"
	"fmt"
	"os"
)

// CleanupTempFile is the idempotent defer-cleanup for the
// CreateTemp → Write → Close → Rename atomic-write pattern. It closes
// the fd if still open (no-op if already closed) and removes the file
// if still present (no-op if already renamed away). Safe to call
// regardless of which step succeeded.
func CleanupTempFile(tmp *os.File) error {
	if cerr := tmp.Close(); cerr != nil && !errors.Is(cerr, os.ErrClosed) {
		return fmt.Errorf("close tempfile: %w", cerr)
	}
	if rmErr := os.Remove(tmp.Name()); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
		return fmt.Errorf("remove tempfile: %w", rmErr)
	}
	return nil
}
