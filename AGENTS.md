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

## Scripts

- When a script is needed, prefer bash, then python.

## Design Principles

Keep it simple. Don't add defensive code for edge cases unless driven by external input, a real bug, or a clear requirement.

Follow existing code patterns and project style unless the user explicitly asks otherwise.
