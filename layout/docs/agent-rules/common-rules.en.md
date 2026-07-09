# Go-Spring Shared Conventions

This file applies to the Go-Spring framework repository and to all projects built on Go-Spring.

## Design Principles

Keep it simple. Don't add defensive code for edge cases unless driven by external input, a real bug, or a clear requirement.

Follow existing code patterns and project style unless the user explicitly asks otherwise.

## Coding Style

See [coding-style.md](../coding-style/coding-style.md), covering naming, formatting and organization, error handling, testing, concurrency, Go idioms, and more.

## Code Hygiene

- Code must pass `modernize` and `go fix` with no findings.

## Scripts

- When a script is needed, prefer bash, then python.
