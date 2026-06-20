---
name: add-feature-design-and-implementation-docs
description: Workflow command scaffold for add-feature-design-and-implementation-docs in codex-converter.
allowed_tools: ["Bash", "Read", "Write", "Grep", "Glob"]
---

# /add-feature-design-and-implementation-docs

Use this workflow when working on **add-feature-design-and-implementation-docs** in `codex-converter`.

## Goal

Adds both a design spec and an implementation plan for a new feature or enhancement.

## Common Files

- `docs/superpowers/specs/{date}-{feature}-design.md`
- `docs/superpowers/plans/{date}-{feature}.md`

## Suggested Sequence

1. Understand the current state and failure mode before editing.
2. Make the smallest coherent change that satisfies the workflow goal.
3. Run the most relevant verification for touched files.
4. Summarize what changed and what still needs review.

## Typical Commit Signals

- Create a design spec markdown file in docs/superpowers/specs/
- Create an implementation plan markdown file in docs/superpowers/plans/

## Notes

- Treat this as a scaffold, not a hard-coded script.
- Update the command if the workflow evolves materially.