// Package configgen materializes per-machine config files in the
// user's home directory: ~/.zshrc (SetupZshrc), ~/.gitconfig
// (SetupGitconfig), and ~/.gnupg/gpg-agent.conf (SetupGpg). Each
// function early-returns silently when its target already exists, so
// running setup repeatedly never clobbers a user-edited file.
//
// SetupGpg shells out to killall + gpg-agent; tests inject a fake
// CmdRunner. See cmdrunner.go for the abstraction.
package configgen
