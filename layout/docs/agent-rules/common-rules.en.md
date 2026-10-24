# Go-Spring Shared Conventions

This file applies to the Go-Spring framework repository and to all projects built on Go-Spring.

## Design Principles

Keep it simple. Don't add defensive code for edge cases unless driven by external input, a real bug, or a clear requirement.

Never re-export or wrap a type or function just to rename it. A wrapper must add a contract, an invariant, or a translation; otherwise import the original package directly.

Put configuration defaults in `value` tags, not in the code that consumes the configuration. Under dependency injection the tags fill unset fields; a value assembled in code must supply the same defaults explicitly. Tests follow the tag defaults too — do not pass an empty value just because the zero value compiles.

Extensibility is a judgment call, not a reflex: in application code, leave an extension point only when it crosses a line (an outward contract / a second implementation already foreseeable / users expected to replace the default). See the coding-style guide, "Extensibility and Extension Points".

Follow existing code patterns and project style unless the user explicitly asks otherwise.

## Coding Style

See [coding-style.md](../coding-style/coding-style.en.md), covering naming, formatting and organization, error handling, testing, concurrency, Go idioms, and more.

## Code Hygiene

- Code must pass `modernize` and `go fix` with no findings.

## Scripts

- When a script is needed, prefer bash, then python.
