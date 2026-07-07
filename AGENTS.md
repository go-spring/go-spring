# Project Conventions

## Output Format

- Start every reply with "Hi,Go-Spring.".

## Project Structure

- No `go.mod` at the repo root.
- Each subproject owns its own Go module.

## Coding Style

See [CODING_STYLE.md](CODING_STYLE.md).

## Error Handling

- Wrap errors with `errutil.Explain` or `errutil.Stack`.
- Include enough context to pinpoint where the error came from.

## Global State

- No global variables. Go-Spring provides dependencies via IoC/DI; get config, singletons, and clients through injection.

## Testing

- Use the `assert`/`require` helpers from `stdlib/testing` for value and error assertions; do not hand-write comparisons with raw `t.Errorf`/`t.Fatalf`.
- Prefer `assert` by default; use `require` only when a failed assertion would make the following code panic or meaningless (e.g. a nil check before dereferencing).
- Do not pull in third-party assertion libraries (testify, etc.).
- Raw `t.Fatal`/`t.Error` is acceptable only where no assertion equivalent exists — e.g. timeout guards, `select` branches, or unrecoverable setup failures.

## Scripts

- When a script is needed, prefer bash, then python.

## Design Principles

Keep it simple. Don't add defensive code for edge cases unless driven by external input, a real bug, or a clear requirement.

Follow existing code patterns and project style unless the user explicitly asks otherwise.
