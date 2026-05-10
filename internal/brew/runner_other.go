//go:build !darwin

package brew

import "context"

// IsAppRunning is not supported on non-darwin platforms. It returns false, nil
// so that internal/packages orchestration code can be unit-tested on Linux
// CI without spurious skips. Production code only runs on macOS.
func (r *runner) IsAppRunning(_ context.Context, _ string) (bool, error) {
	return false, nil
}
