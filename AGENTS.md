<!-- # Project Conventions

## When to Record a Convention

To decide whether a convention is worth writing down, ask yourself three things:

1. Would a skilled engineer, new to this project, naturally do it this way? If so, don't bother recording it.
2. Does breaking it cause real consequences — build failures, review rejections, even production incidents? If so, record it first.
3. Is it already explained elsewhere (code comments, CLAUDE.md, CODING_STYLE.md)? If so, link to it rather than repeat it.

## How Knowledge Is Layered

Constraints are layered, and references flow one way. Follow this when adding or
reorganizing any rule:

- **One home per rule.** Every rule lives in exactly one document. Cross-cutting
  rules — those spanning directories or defining relationships between them — home
  in the entry docs: [ARCHITECTURE.md](ARCHITECTURE.md) for code/directory
  boundaries, this file for conventions. Module-local rules home in that module's
  own `DESIGN.md`; do not lift them into an entry doc, or the rule drifts away from
  the code it governs.
- **Relocate, don't summarize.** To consolidate a duplicated rule, move it to its
  home and replace the original with a link. Never write a summary and keep the
  original too — that produces copies that drift apart.
- **References point one way.** Entry docs point down to detailed docs. A detailed
  doc may point back up to the single owner of a cross-cutting rule, but avoid
  bidirectional link loops.

## Output Format

- Start every reply with "Hi,Go-Spring.".

## Shared Conventions

Shared conventions for projects using Go-Spring live in [layout/docs/agent-rules/common-rules.en.md](layout/docs/agent-rules/common-rules.en.md), covering design principles, coding style, error handling, testing, and more.

## Project Structure

The directory-boundary map, the one-way four-layer dependency model
(`stdlib`/`log` → `spring` → `starter-*`/`gs-*`), the "where does new code go?"
decision guide, and per-layer non-goals all live in
[ARCHITECTURE.md](ARCHITECTURE.md) — its single owner.

## Coding Style

- Every source file must carry the Apache License header; see [LICENSE_HEADER](LICENSE_HEADER) for the template. -->
