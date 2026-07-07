# Claude Best Practices

Experience accumulated from working with Claude (and other AI coding assistants) in Go-Spring, split into two categories by how firmly it is established:

- **Hard conventions**: applied without judgment, maintained in `CLAUDE.md` / Skills / hooks so they take effect automatically.
- **Soft conventions**: depend on judgment and vary by scenario; hard-coding them tends to backfire, so they live in the docs as a reference.

## Hard conventions

Established rules are maintained in the repository root `CLAUDE.md`: [go-spring/go-spring · CLAUDE.md](https://github.com/go-spring/go-spring/blob/master/CLAUDE.md).

Once a piece of experience becomes "no context needed, no trade-off needed, just do it", promote it there and remove it from the soft conventions.

## Soft conventions

Experience that depends on judgment and varies by scenario, deliberately not hardened into rules:

- Whether to split a PR or bundle it into one depends on the cohesion of the change, not the line count.
- Whether to abstract: three lines of similar code are usually better than a premature abstraction; the right time to converge depends on the situation.
- Whether to ask first or act directly depends on how ambiguous and reversible the task is.

<!-- Keep appending: experience that needs trade-offs and depends on context -->

## Maintenance conventions

- Annotate soft conventions with their origin where possible (which collaboration, what problem triggered it) to make it easier to judge whether they are stale.
- Once a soft convention is mature and can be executed mechanically, move it into `CLAUDE.md` or a Skill and remove it from this document.
- If a rule frequently backfires, demote it from `CLAUDE.md` back to a soft convention and record the reason.
