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

## Design Principles

Keep it simple. Don't add defensive code for edge cases unless driven by external input, a real bug, or a clear requirement.
