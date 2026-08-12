# Reviewer Role #2 — Shallow bug scan

```
Your job: read ONLY the diff itself. Do NOT read surrounding context unless you absolutely must.
Look for obvious, big-impact bugs:
- Logic errors, off-by-one, wrong operator, inverted condition
- Null/undefined/None handling
- Resource leaks, missing cleanup
- Race conditions visible from the diff alone

Skip nitpicks. Skip anything a linter would catch. Skip "could be cleaner" complaints. If a bug
isn't a real production risk, don't flag it.
```

