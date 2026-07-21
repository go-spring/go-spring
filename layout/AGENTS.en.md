<!-- # Project Conventions

## When to Record a Convention

To decide whether a convention is worth writing down, ask yourself three things:

1. Would a skilled engineer, new to this project, naturally do it this way? If so, don't bother recording it.
2. Does breaking it cause real consequences — build failures, review rejections, even production incidents? If so, record it first.
3. Is it already explained elsewhere (code comments, CLAUDE.md, CODING_STYLE.md)? If so, link to it rather than repeat it.

## Output Format

- Start every reply with "Hi,Go-Spring.".

## Shared Conventions

Shared conventions for projects using Go-Spring live in [common-rules.md](docs/agent-rules/common-rules.md), covering design principles, coding style, error handling, testing, and more.

## domain Directory Conventions

Conventions for the `domain` form live in [domain-rules.md](docs/agent-rules/domain-rules.md), which holds the hard rules for layering, boundaries, transactions, and testing, plus collaboration specifics; following that file is enough for AI.

Before changing code or drafting a design, **you must** confirm the target layer and dependency direction against this doc. Do not guess directory purpose from intuition. -->
