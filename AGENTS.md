# Project Conventions

## When to Record a Convention

To decide whether a convention is worth writing down, ask yourself three things:

1. Would a skilled engineer, new to this project, naturally do it this way? If so, don't bother recording it.
2. Does breaking it cause real consequences — build failures, review rejections, even production incidents? If so, record it first.
3. Is it already explained elsewhere (code comments, CLAUDE.md, CODING_STYLE.md)? If so, link to it rather than repeat it.

## Output Format

- Start every reply with "Hi,Go-Spring.".

## Shared Conventions

Shared conventions for projects using Go-Spring live in [layout/docs/agent-rules/common-rules.en.md](layout/docs/agent-rules/common-rules.en.md), covering design principles, coding style, error handling, testing, and more.

## Project Structure

- No `go.mod` at the repo root; each subproject owns its own Go module.
- Four-layer dependency structure, flowing one way and never in reverse:
  - **`stdlib/`** — foundation layer, **zero dependencies**, common utilities shared across modules.
  - **`spring/`** — core container layer, implements IoC container and dependency injection; depends on no third-party business package (Redis, GORM, ...).
  - **`starter-*/`** — integration layer, integrates third-party services (Redis, GORM, ...) on top of core abstractions.
  - **`gs-*/`** — tooling layer, project scaffolding, code generation, and other dev tools.

## Coding Style

- Every source file must carry the Apache License header; see [LICENSE_HEADER](LICENSE_HEADER) for the template.
