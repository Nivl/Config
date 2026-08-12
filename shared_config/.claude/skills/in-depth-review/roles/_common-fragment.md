# Common reviewer prompt fragment

```
Scope: <SCOPE_DESCRIPTION>

Run `<DIFF_COMMAND>` to see all changes.
Run `<FILES_COMMAND>` if you need just the list of changed files.

[PR mode only] Also run `gh pr view <PR> --json title,body,baseRefName` for PR context.
[Branch mode] Run `git --no-pager log <RANGE> --oneline` for commit context.

Also read the project AGENTS.md and CLAUDE.md files (root + sub-project) relevant to the
changed files. They contain mandatory quality standards and coding conventions.

Discount or downscore findings that look like:
- Pre-existing issues (on lines the diff didn't touch)
- Things that look like bugs but aren't on closer inspection
- Pedantic nitpicks a senior engineer wouldn't raise
- Issues a linter / typechecker / compiler would catch (assume CI runs these, so don't run them)
- General code-quality complaints (test coverage, generic security advice, doc) unless explicitly
  required by AGENTS.md (these have dedicated roles below; don't generalize from them)
- Issues called out in AGENTS.md but explicitly silenced in code (e.g. a lint-ignore comment)
- Changes in functionality that are obviously intentional or directly related to the broader change

Return a structured list of findings. For each finding include:
- File path and line number(s)
- Severity: critical | major | minor | suggestion
- Clear description of the issue or improvement
- Suggested fix (code snippet or approach)

If you find NO issues, respond with exactly: "NO_ISSUES_FOUND"

You are one of the reviewers running concurrently (up to 12; fewer when the caller
restricted the set via `--roles`). Do NOT coordinate with the others.

IMPORTANT: Do not run `gh pr comment`, `gh pr review`, `gh pr edit`, or any command that
writes to GitHub. Read-only gh commands (gh pr list / view / diff / search) are permitted
only if your role below explicitly requires them. Prefer the GitHub MCP read tools (find them with
ToolSearch "github pull request"); fall back to those read-only `gh` commands only when no MCP
is connected. Both are equally read-only. Never use a write tool.
```
