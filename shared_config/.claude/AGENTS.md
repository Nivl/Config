## Git invocation

Run plain `git ...` (e.g. `git status`, `git log`, `git diff`) from inside the target repo. To work on a _different_ repo (a worktree, a fixture in a temp dir), `cd` into it first **as its own Bash call** — the Bash tool's CWD persists across calls — then run plain `git`. Do **not** use top-level `git -C <path>`: the `git-deny-dash-c.py` hook denies it. (Subcommand-level `-C` such as `git diff -C` / `git blame -C` for copy detection is fine — only the top-level directory `-C` is blocked.)

## No redundant `cd`

Never prepend `cd <path> && ...` to a command when `<path>` is already the shell's current working directory. The Bash tool's CWD persists across calls, so the `cd` is pure noise — and worse, it changes the command string enough to miss the user's allowlist and trigger an extra permission prompt. If you genuinely need a different directory, run `cd` as its own separate Bash call, or for non-git tools use a per-command flag (`make -C`, `npm --prefix`, `pytest --rootdir=`). For git, `cd` first — top-level `git -C` is denied by a hook (see "Git invocation").

This holds even when the `cd` is _not_ redundant. Claude Code force-prompts (`cd-compound-redirect`) any compound command that combines `cd` with output redirection — **including `2>&1` and pipes**, e.g. `cd app && rushx lint:typecheck 2>&1 | tail -5`. Run the `cd` as its own Bash call first, then the command separately (`cd app` → `rushx lint:typecheck 2>&1 | tail -5`), so no single command mixes `cd` with a redirect. In a monorepo where you must run from a package subdir (e.g. `rushx`), this is the most common avoidable prompt. Subagents must follow this too.

## Code comments

Commenting code is good. Keep doing it. But every comment must earn its place. Follow these rules:

- **Explain _why_, not _what_.** Capture intent, a constraint, an edge case, or the reason for an unobvious choice. Do not narrate code that already reads clearly. `// increment the counter` above `count++` is noise. You can explain the what, if it help explaining the why.
- **No changelog or diff comments.** A comment describes the code as it stands now, never how it got there. Ban `// added this`, `// changed from X`, `// new logic`, `// was previously ...`, `// TODO: remove old impl`. That history lives in git, not in the source.
- **Don't restate the line below.** If the comment is just an English paraphrase of the next statement, delete it — the code is the source of truth and the comment will rot the moment the code changes.
- **No ticket numbers.** Never reference the current ticket or issue in a comment. The reader has the code, not your tracker.
  - Good: `// some comment.`
  - Bad: `// some comment (TICKET-0123).`
- **Keep them short and focused.** One or two lines is the norm. Only write a long paragraph when the logic is genuinely complex and a short note cannot carry it.
- **Write plainly.** Use short sentences. Avoid semicolons. Aim for a middle-school or high-school reading level, while still using normal software engineering terms.
