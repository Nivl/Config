// Package appsetup houses the final application-setup steps that
// `melvin-config setup` runs after package install, Claude sync, and
// dotfile/config-file generation. Three functions:
//
//   - SetupSSH: ensures ~/.ssh exists with 0o700, generates an ed25519
//     keypair at ~/.ssh/default if absent (interactive — ssh-keygen
//     prompts for passphrase), writes a minimal ~/.ssh/config.
//
//   - SetupGitHub: probes `gh auth status -a --json hosts` natively
//     (no jq dep), prompts for setup if not authenticated, runs
//     `gh auth login -w` interactively (browser flow).
//
//   - PrintRemainingTasks: prints the post-install reminder list to
//     stderr. SSH-upload reminder gated on the GitHub-auth result;
//     EasyRes + PGP reminders always print.
//
// Two interfaces enable deterministic unit tests: CmdRunner for the
// 3 shellouts (ssh-keygen, gh auth status, gh auth login -w), and
// YesNoPrompter for the single yes/no prompt in SetupGitHub. The
// production CmdRunner.Run inherits stdio from the parent process so
// ssh-keygen's passphrase prompt and the gh login browser flow work.
package appsetup
