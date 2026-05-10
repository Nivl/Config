@RTK.md

## Git invocation

Use plain `git ...` (e.g. `git status`, `git log`, `git diff`) when the shell's working directory is already inside the target repo — don't prefix with `git -C <path>` as a safety habit. Only use `git -C <path>` when operating on a _different_ repo (a fixture in a temp dir, a worktree at another path, etc.).

## No redundant `cd`

Never prepend `cd <path> && ...` to a command when `<path>` is already the shell's current working directory. The Bash tool's CWD persists across calls, so the `cd` is pure noise — and worse, it changes the command string enough to miss the user's allowlist and trigger an extra permission prompt. If you genuinely need a different directory, prefer a per-command flag (`git -C`, `make -C`, `npm --prefix`, `pytest --rootdir=`) or run `cd` as its own separate Bash call.
