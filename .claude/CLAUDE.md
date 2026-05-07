@RTK.md

## Git invocation

Use plain `git ...` (e.g. `git status`, `git log`, `git diff`) when the shell's working directory is already inside the target repo — don't prefix with `git -C <path>` as a safety habit. Only use `git -C <path>` when operating on a *different* repo (a fixture in a temp dir, a worktree at another path, etc.).
