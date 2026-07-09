# Go-Spring Shared Conventions

This file applies to the Go-Spring framework repository and to all projects built on Go-Spring.

## Design Principles

Keep it simple. Don't add defensive code for edge cases unless driven by external input, a real bug, or a clear requirement.

Follow existing code patterns and project style unless the user explicitly asks otherwise.

## Coding Style

See [coding-style.md](../coding-style/coding-style.md).

## Code Hygiene

- Code must pass `modernize` and `go fix` with no findings.
- No commented-out dead code; delete obsolete code instead of commenting it out.

## Global State

- Avoid global variables. Go-Spring provides dependencies via IoC/DI; get config, singletons, and clients through injection.

## Error Handling

- **Do not** construct errors with `errors.New` / `fmt.Errorf` directly. Route everything through `errutil`.
- Add business semantics with `errutil.Explain(err, "...")`. Track call paths with `errutil.Stack(err, "Xxx")`.
- Include enough context to pinpoint where the error came from.

## Testing

- Use the `assert`/`require` helpers from `stdlib/testing` for value and error assertions.
- Prefer `assert` by default; use `require` only when a failed assertion would make the following code panic or meaningless (e.g. a nil check before dereferencing).
- Do not pull in third-party assertion libraries (testify, etc.).
- Use raw `t.Error`/`t.Fatal` only where no assertion equivalent exists — e.g. timeout guards, `select` branches, or unrecoverable setup failures.

## Scripts

- When a script is needed, prefer bash, then python.
