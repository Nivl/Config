package appsetup

import (
	"fmt"
	"io"
	"path/filepath"
)

// PrintRemainingTasks prints the post-install reminder list to out.
// The EasyRes line is always printed (we no longer probe mdfind to
// see whether it is already installed).
//
// Output:
//   - "Things left to do:\n"
//   - If !githubAuthed: "\t* Upload <homeDir>/.ssh/default to your
//     Cloud VCS: 'pbcopy < <homeDir>/.ssh/default.pub'\n"
//   - Always: "\t* (optional) Install EasyRes if needed: http://easyresapp.com\n"
//   - Always: "\t* (optional) Import PGP Key from Enpass with 'gpg --import private.key'\n"
func PrintRemainingTasks(out io.Writer, homeDir string, githubAuthed bool) {
	_, _ = fmt.Fprintln(out, "Things left to do:")
	if !githubAuthed {
		privateKey := filepath.Join(homeDir, ".ssh", "default")
		pubKey := privateKey + ".pub"
		_, _ = fmt.Fprintf(out, "\t* Upload %s to your Cloud VCS: 'pbcopy < %s'\n", privateKey, pubKey)
	}
	_, _ = fmt.Fprintln(out, "\t* (optional) Install EasyRes if needed: http://easyresapp.com")
	_, _ = fmt.Fprintln(out, "\t* (optional) Import PGP Key from Enpass with 'gpg --import private.key'")
}
