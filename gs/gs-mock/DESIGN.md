# gs-mock Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`gs-mock` is an external `gs` tool in the tooling layer of the four-layer
stack (stdlib → spring → starter → gs). It generates type-safe Go mock
code for interfaces, plain functions, and struct methods, with native
support for generics.

## 1. Responsibilities & Boundaries

- Parse Go source in a target directory, discover selected interfaces /
  functions / methods, and emit mock code that plugs into a
  `gsmock` runtime (`go-spring.org/gs-mock/gsmock`).
- Ships two APIs a test author uses: `Handle` mode (full callback owns
  the call), and `When … Then / Return` mode (declarative expectations).
- Does not touch the container. Generated mocks are plain Go values you
  hand-inject in tests; nothing depends on `spring/`.

## 2. Key Abstractions & Seams

- **External-tool protocol.** Binary name `gs-mock`, lives beside `gs`.
  `gs-mock --version` prints a two-line description + version; `gs mock`
  dispatches to it via `tool.Call`.
- **Runtime library (`gsmock`).** The generated code targets a fixed,
  type-safe API surface backed by generics (up to 7 params, 4 returns).
  This is the seam the CLI writes to; both sides must be released
  together.
- **Filter syntax.** `-i "Reader,Writer"` includes those; a `!` prefix
  excludes (`-i "!Logger"`). Empty defaults to all interfaces in the
  scanned package.
- **Output routing.** `-o path.go` writes to disk; missing `-o` streams
  to stdout for pipe-through use in `go generate`.

## 3. Constraints

- Regenerate mocks whenever the source interface changes — the generated
  file is not tracked by `go generate` implicitly; the tool is the
  authoritative writer.
- Generic parameter count caps (7 params / 4 returns) are set by the
  runtime helpers; do not exceed them without extending `gsmock` first.

## 4. Trade-offs & Alternatives Rejected

- **`gomock` / `mockgen` not adopted.** Those pre-date Go generics and
  fall back to `reflect.Value` at call sites; `gs-mock` keeps types
  through generics so IDEs autocomplete and the compiler catches
  argument-count drift.
- **No wrapper around gomock.** Codegen is written directly for the
  `gsmock` runtime so both parts evolve as one.
