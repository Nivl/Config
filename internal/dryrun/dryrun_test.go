package dryrun

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNullReporter_Silent — every method on the NullReporter is a
// no-op. Production code paths call reporter methods unconditionally
// and rely on this silence.
func TestNullReporter_Silent(t *testing.T) {
	r := NewNullReporter()
	r.Section("foo")
	r.FileChange("/a", []byte("x"), []byte("y"), "summary")
	r.FileNoOp("/a", "ok")
	r.Symlink("/a", "/b", "no-op")
	r.Shellout("brew", []string{"install", "git"}, "")
	r.FinalSummary()
	assert.NotNil(t, r)
}

// TestRealReporter_FileChange — FileChange emits the one-liner and a
// unified diff of before/after. A fresh write (empty before) uses
// /dev/null as the old-name.
func TestRealReporter_FileChange(t *testing.T) {
	var out bytes.Buffer
	r := NewReporter(&out)
	r.FileChange("/tmp/x", []byte("old\n"), []byte("new\n"), "would change")

	got := out.String()
	assert.Contains(t, got, "[dry-run] /tmp/x — would change")
	assert.Contains(t, got, "--- /tmp/x")
	assert.Contains(t, got, "+++ /tmp/x (proposed)")
	assert.Contains(t, got, "-old")
	assert.Contains(t, got, "+new")
}

// TestRealReporter_FileChangeFreshWrite — empty before content
// produces "--- /dev/null" in the diff header.
func TestRealReporter_FileChangeFreshWrite(t *testing.T) {
	var out bytes.Buffer
	r := NewReporter(&out)
	r.FileChange("/tmp/x", nil, []byte("hello\n"), "fresh write")

	got := out.String()
	assert.Contains(t, got, "--- /dev/null")
	assert.Contains(t, got, "+++ /tmp/x (proposed)")
}

// TestRealReporter_FileNoOp — FileNoOp emits the one-liner without
// a diff.
func TestRealReporter_FileNoOp(t *testing.T) {
	var out bytes.Buffer
	r := NewReporter(&out)
	r.FileNoOp("/tmp/x", "in sync")

	got := out.String()
	assert.Equal(t, "[dry-run] /tmp/x — in sync\n", got)
	assert.NotContains(t, got, "---")
	assert.NotContains(t, got, "+++")
}

// TestRealReporter_Symlink — Symlink emits the one-liner with the
// decision word and the link target.
func TestRealReporter_Symlink(t *testing.T) {
	var out bytes.Buffer
	r := NewReporter(&out)
	r.Symlink("/home/u/.emacs.d", "/repo/.emacs.d", "would-create")

	assert.Equal(t,
		"[dry-run] /home/u/.emacs.d — would-create -> /repo/.emacs.d\n",
		out.String())
}

// TestRealReporter_Shellout — Shellout emits the command + args.
func TestRealReporter_Shellout(t *testing.T) {
	var out bytes.Buffer
	r := NewReporter(&out)
	r.Shellout("brew", []string{"install", "git"}, "install git")

	assert.Equal(t,
		"[dry-run] would run: brew install git — install git\n",
		out.String())
}

// TestRealReporter_ShelloutNoSummary — Shellout without a summary
// omits the "— summary" suffix.
func TestRealReporter_ShelloutNoSummary(t *testing.T) {
	var out bytes.Buffer
	r := NewReporter(&out)
	r.Shellout("gpg-agent", []string{"--daemon"}, "")

	assert.Equal(t,
		"[dry-run] would run: gpg-agent --daemon\n",
		out.String())
}

// TestRealReporter_Section — Section emits a header line.
func TestRealReporter_Section(t *testing.T) {
	var out bytes.Buffer
	r := NewReporter(&out)
	r.Section("configgen")

	assert.Contains(t, out.String(), "== configgen ==")
}

// TestRealReporter_FinalSummary — counters are accurate across mixed
// method calls.
func TestRealReporter_FinalSummary(t *testing.T) {
	var out bytes.Buffer
	r := NewReporter(&out)
	r.FileChange("/a", []byte("x"), []byte("y"), "")
	r.FileChange("/b", []byte("x"), []byte("y"), "")
	r.FileNoOp("/c", "ok")
	r.Symlink("/d", "/e", "would-create")
	r.Symlink("/f", "/g", "would-back-up-then-create")
	r.Shellout("brew", []string{"install"}, "")
	r.FinalSummary()

	got := out.String()
	assert.Contains(t, got,
		"2 files would change, 1 backups, 2 symlinks, 1 shellouts.")
	assert.Contains(t, got,
		"Re-run without --dry-run to apply.")
}
