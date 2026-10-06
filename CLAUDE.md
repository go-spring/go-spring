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

## AI Collaboration Rules

### Proposals and Decisions

- Give each proposal one complete technical trade-off analysis (risks and costs included); once the user decides, execute it fully and without commentary, and do not re-argue (2026-09-21).
- In architecture decisions, "this feels awkward", "one mental model", and "an evolution seam in the signature" are legitimate benefits in their own right — do not overrule them with "the engineering-optimal solution" or "line count".
- Point out proactively when you notice you have reversed your own position (for example, arguing to delete a mechanism you had earlier deleted and then defended).
- Answer design questions in order: (1) first judge whether the requirement itself is sound (validate against a real scenario, not against code); (2) then judge whether the change itself is sound — "break compatibility" is an allowed conclusion; (3) only if the first two hold, discuss how compatibility lands; (4) always state the cost of a change, or it is one-sided advocacy (2026-09-28).
- Never defend the current code: "that's how it is / that's how the code is written" is not an argument (2026-09-28).
- The user grows visibly impatient when the same question goes back and forth: converge fast; do not split one decision into several confirmations.

### Prerequisites for Reviewing a Design

- Before judging a design, align on the working model: first establish "whom does this system declare itself to serve, and by what model does it work", then judge; with the model undefined, no implementation is right or wrong (2026-10-03).
- When a premise of yours is disproved, re-derive the conclusion instead of swapping in another premise to defend the same conclusion.

### Answering Style

- Answer only what was asked: if asked for facts, give facts only (call sites, dependency edges, line numbers); output analysis only when explicitly requested, and deliver it in one pass (2026-09-28).
- Do not end with probing questions such as "do you need me to ...?"; the user will raise their own requests.
- When unsure whether to expand, default to not expanding.

### Self-Directed Evolution

- Do not wait to be assigned work; find it. Keep interrogating an implementation for a better solution, even trying several and picking the best ("best of the best").
- Sacred core (no breaking changes): `stdlib/`, `log/`, and the `gs` core of `spring/` — additive helpers are fine, breaking changes are not. Freely reshapeable: the ecosystem (`starter/`, `contrib/`, the `gs/` tooling, `examples/`).
- The previous bullet is a self-imposed constraint, not a boundary to hold up against the user: when the user gives an explicit instruction, the user wins (2026-09-21).
- Expose problems through "cross-backend / cross-module consistency": where an implementation deviates from its family's contract is usually a real defect.
- The repository docs (`ARCHITECTURE.md`, `starter/DESIGN.md`, each package's `DESIGN.md`) encode the contract — read them first as a ruler, and fix doc drift alongside the code.
- Verify with build + vet + test; add new tests only for testable backends, and honestly state the coverage gap for backends that have none (they need live infra).
- A self-owned bug you have pinpointed (root cause found, fix clear, reproducible) must be fixed directly, even if it sits in a module marked "mature core"; "boundary / sacred ground" is not an excuse.
- Distinguish real boundaries from fake ones: a real boundary means the fix needs something unavailable here (for example an intranet GOPROXY); a fake boundary is "this file is in the sensitive area I labeled" while the fix is local and verifiable.
- When the trail is uncertain, escalate (add instrumentation, read more) rather than treating the uncertainty as a reason to hand back; do not let the work drift toward the easy audit and away from the hard fix.

### Editing and Rewriting

- By default, edit in place on the current branch; do not create a worktree (unless the user explicitly asks), and do not proactively offer the worktree option (2026-07-22).
- When the user says "forget the previous rationale, describe only the current behavior", it is a hard constraint: read only the current working-tree files; do not use `git show HEAD:<file>`, `git log -p`, or dig through deleted history for old versions (2026-10-02).
- Old docs are not source material or a writing anchor; after writing, self-check that every sentence has a basis in the current code.
- Never write `Why` / design-trade-off sentences; the only permitted "past" is format convention such as the license header, assert-library usage, and the `README.md` + `README_CN.md` pairing.
- When you only need format conventions, look at sibling packages of the same kind — for format, not for content.

## Shared Conventions

Shared conventions for projects using Go-Spring live in [layout/docs/agent-rules/common-rules.en.md](layout/docs/agent-rules/common-rules.en.md), covering design principles, coding style, error handling, testing, and more.

## Project Structure

The directory-boundary map, the one-way four-layer dependency model
(`stdlib`/`log` → `spring` → `starter-*`/`gs-*`), the "where does new code go?"
decision guide, and per-layer non-goals all live in
[ARCHITECTURE.md](ARCHITECTURE.md) — its single owner.

## Coding Style

- Every source file must carry the Apache License header; see [LICENSE_HEADER](LICENSE_HEADER) for the template. -->
