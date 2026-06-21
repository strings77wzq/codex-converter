---
name: test-and-fix-backend-code
description: Workflow command scaffold for test-and-fix-backend-code in codex-converter.
allowed_tools: ["Bash", "Read", "Write", "Grep", "Glob"]
---

# /test-and-fix-backend-code

Use this workflow when working on **test-and-fix-backend-code** in `codex-converter`.

## Goal

Implements a backend bugfix or enhancement together with corresponding tests.

## Common Files

- `internal/proxy/handler.go`
- `internal/proxy/handler_test.go`
- `internal/convert/stream.go`
- `internal/convert/stream_test.go`

## Suggested Sequence

1. Understand the current state and failure mode before editing.
2. Make the smallest coherent change that satisfies the workflow goal.
3. Run the most relevant verification for touched files.
4. Summarize what changed and what still needs review.

## Typical Commit Signals

- Edit backend implementation file(s) (e.g., .go source files)
- Edit or add corresponding test file(s) (e.g., _test.go files)

## Notes

- Treat this as a scaffold, not a hard-coded script.
- Update the command if the workflow evolves materially.