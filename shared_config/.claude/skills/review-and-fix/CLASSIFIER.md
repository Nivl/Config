# Commit-class file patterns

The pattern tables sub-step 7's commit classifier matches a changed path against. The classifier
itself, its three classes, its first-match rule and its classify-up tie-break stay in SKILL.md
sub-step 7. Read this file before you classify.

Two clauses here classify a file UP, into `logic`, against the bucket its path would otherwise put
it in. Both are repeated in SKILL.md's stubs, so an unread file cannot lose them. A shared helper
under a test path is `logic`, and `tsconfig*.json` and `package.json` are `logic`.

## Test-file patterns

A test file is one whose name matches any of:

- `*.test.<code-ext>`
- `*.spec.<code-ext>`
- `*_test.<code-ext>`
- `test_*.<code-ext>`

Or one that sits under any of these directories:

`__tests__`, `__mocks__`, `__snapshots__`, `tests`, `test`, `spec`

Add the equivalent convention for whatever language the repo uses, so a genuine test file is not
missed.

`<code-ext>` means a source-code extension, so `parse.spec.ts` is a test file and
`openapi.spec.yaml` is not.

Directory names match a path SEGMENT anywhere in the path, not just at the repo root, so
`packages/api/tests/parse.test.ts` counts.

One shape does not qualify. A file under a test path that is not itself a test, such as a shared
helper or fixture module, is NOT a test file and IS `logic`, because non-test code may import it.

## The never-logic list

A file here never makes a commit `logic` on its own, and never disqualifies a commit from `prose`
or `test`.

- **Documentation and data:** `*.md`, `*.mdx`, `*.txt`, `docs/**`, `*.snap`, `LICENSE`. Markdown
  is on this list unconditionally. A fenced code block inside a `*.md` file does not change its
  class, including in a repo whose tooling extracts and runs those fences.
- **Build, lint and CI configuration:** `jest.config.*`, `vitest.config.*`,
  `playwright.config.*`, `pytest.ini`, `conftest.py`, `.eslintrc*`, `.oxlint/**`, `.prettierrc*`,
  `.github/workflows/**`, `Dockerfile*`, `*.lock`, `pnpm-lock.yaml`. Match the config basenames
  wherever they sit.
- **`*.json`, except `tsconfig*.json` and `package.json`.** Those two are `logic`, because one
  decides what the compiler emits and the other decides which code is installed.

Test-runner configuration is deliberately on this list rather than forced to `logic`, so do not
classify it up. Nothing in production imports one, so what it can break is which tests run rather
than what the app does. The cost that buys, and why row 4 is not the place to catch it, is in
[RATIONALE.md](RATIONALE.md).

## What counts as `logic`

Any change to executable code. That includes a string or number literal that logic reads, a moved
statement, and an import.

Application configuration is `logic` too, meaning whatever the running app reads. In this repo
that is `config/src/**`, `config/*.yml` and `config/*.yaml`, plus any constants or env-default
module the app imports.

A code path behind a flag that is currently off is `logic` as well. The flag is flipped remotely,
so no commit marks the moment that path goes live.
