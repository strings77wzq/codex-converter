```markdown
# codex-converter Development Patterns

> Auto-generated skill from repository analysis

## Overview

This skill documents the development patterns, coding conventions, and workflows used in the `codex-converter` Go codebase. It covers how to structure code, write tests, and contribute new features or fixes using established repository practices. This guide is intended for contributors seeking to maintain consistency and quality in the project.

## Coding Conventions

### File Naming

- Use **camelCase** for file names.
  - Example: `streamHandler.go`, `convertStream.go`

### Import Style

- Use **relative imports** within the project.
  - Example:
    ```go
    import (
        "internal/proxy"
        "internal/convert"
    )
    ```

### Export Style

- Use **named exports** for functions, types, and variables.
  - Example:
    ```go
    // In stream.go
    func ConvertStream(input io.Reader) (string, error) {
        // implementation
    }
    ```

### Commit Message Style

- Commit types are mixed, but `docs` prefixes are common for documentation changes.
- Commit messages are concise, averaging around 58 characters.

## Workflows

### Add Feature Design and Implementation Docs

**Trigger:** When proposing and planning a new feature or major change  
**Command:** `/new-feature-docs`

1. **Create a design spec markdown file** in `docs/superpowers/specs/`:
    - File name format: `{date}-{feature}-design.md`
    - Example: `docs/superpowers/specs/2024-06-01-streaming-design.md`
2. **Create an implementation plan markdown file** in `docs/superpowers/plans/`:
    - File name format: `{date}-{feature}.md`
    - Example: `docs/superpowers/plans/2024-06-01-streaming.md`

**Example directory structure:**
```
docs/
  superpowers/
    specs/
      2024-06-01-streaming-design.md
    plans/
      2024-06-01-streaming.md
```

---

### Test and Fix Backend Code

**Trigger:** When fixing or improving backend logic and ensuring it is tested  
**Command:** `/fix-and-test-backend`

1. **Edit backend implementation file(s):**
    - Example: `internal/proxy/handler.go`, `internal/convert/stream.go`
2. **Edit or add corresponding test file(s):**
    - Example: `internal/proxy/handler_test.go`, `internal/convert/stream_test.go`
3. **Run tests** to verify correctness:
    ```sh
    go test ./internal/...
    ```
4. **Commit changes** with a clear message.

**Example test file:**
```go
// internal/convert/stream_test.go
package convert

import "testing"

func TestConvertStream(t *testing.T) {
    result, err := ConvertStream(mockInput)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if result != expected {
        t.Errorf("got %v, want %v", result, expected)
    }
}
```

## Testing Patterns

- **Test framework:** Not explicitly specified; uses Go's built-in `testing` package.
- **Test file pattern:** Files are named with `_test.go` suffix.
  - Example: `handler_test.go`, `stream_test.go`
- **Test structure:** Standard Go test functions.
  - Example:
    ```go
    func TestFunctionName(t *testing.T) {
        // test logic
    }
    ```

## Commands

| Command               | Purpose                                                      |
|-----------------------|--------------------------------------------------------------|
| /new-feature-docs     | Start a new feature or enhancement design and implementation |
| /fix-and-test-backend | Fix or enhance backend logic and add corresponding tests     |
```
