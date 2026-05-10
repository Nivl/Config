package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBackupFileAt_TimestampByteIdentical verifies the backup
// filename format: <path>.YYYYMMDDHHMMSS.bkp.
func TestBackupFileAt_TimestampByteIdentical(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "settings.json")
	require.NoError(t, os.WriteFile(src, []byte("original"), 0o644))

	now := time.Date(2026, 5, 11, 14, 30, 45, 0, time.Local) //nolint:gosmopolitan // local time is deliberate
	require.NoError(t, BackupFileAt(src, now))

	want := src + ".20260511143045.bkp"
	assert.FileExists(t, want)
	got, _ := os.ReadFile(want)
	assert.Equal(t, "original", string(got))
}

// TestBackupFile_NoopOnMissingFile verifies BackupFile returns nil
// when the source does not exist, without creating an empty backup.
func TestBackupFile_NoopOnMissingFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "missing.json")
	require.NoError(t, BackupFile(src))

	entries, _ := os.ReadDir(dir)
	assert.Empty(t, entries, "no backup should be created for missing source")
}

// TestBackupFileAt_PreservesMode verifies the backup retains the
// source file's mode bits.
func TestBackupFileAt_PreservesMode(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "settings.json")
	require.NoError(t, os.WriteFile(src, []byte("x"), 0o600))

	now := time.Date(2026, 5, 11, 14, 30, 45, 0, time.Local) //nolint:gosmopolitan // local time is deliberate
	require.NoError(t, BackupFileAt(src, now))

	info, err := os.Stat(src + ".20260511143045.bkp")
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}
