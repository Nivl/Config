package appsetup

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPrintRemainingTasks_GithubAuthed — when authed, the SSH-upload
// reminder is suppressed; EasyRes + PGP always print.
func TestPrintRemainingTasks_GithubAuthed(t *testing.T) {
	out := &bytes.Buffer{}
	PrintRemainingTasks(out, "/home/u", true)

	s := out.String()
	assert.Contains(t, s, "Things left to do:")
	assert.NotContains(t, s, "Upload")
	assert.NotContains(t, s, "pbcopy")
	assert.Contains(t, s, "(optional) Install EasyRes if needed: http://easyresapp.com")
	assert.Contains(t, s, "(optional) Import PGP Key from Enpass with 'gpg --import private.key'")
}

// TestPrintRemainingTasks_GithubNotAuthed — when not authed, the
// SSH-upload reminder is included with the homeDir-prefixed paths.
func TestPrintRemainingTasks_GithubNotAuthed(t *testing.T) {
	out := &bytes.Buffer{}
	PrintRemainingTasks(out, "/home/u", false)

	s := out.String()
	assert.Contains(t, s, "Things left to do:")
	assert.Contains(t, s, "\t* Upload /home/u/.ssh/default to your Cloud VCS: 'pbcopy < /home/u/.ssh/default.pub'")
	assert.Contains(t, s, "(optional) Install EasyRes if needed: http://easyresapp.com")
	assert.Contains(t, s, "(optional) Import PGP Key from Enpass with 'gpg --import private.key'")
}

// TestPrintRemainingTasks_Ordering — when !githubAuthed, the
// SSH-upload reminder appears first (before EasyRes + PGP).
func TestPrintRemainingTasks_Ordering(t *testing.T) {
	out := &bytes.Buffer{}
	PrintRemainingTasks(out, "/home/u", false)

	s := out.String()
	uploadIdx := strings.Index(s, "Upload")
	easyresIdx := strings.Index(s, "EasyRes")
	pgpIdx := strings.Index(s, "PGP Key")

	require := assert.New(t)
	require.Less(uploadIdx, easyresIdx, "Upload should appear before EasyRes")
	require.Less(easyresIdx, pgpIdx, "EasyRes should appear before PGP")
}
