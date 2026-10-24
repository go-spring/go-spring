# Project status: unreleased

[English](STATUS.md) | [中文](STATUS_CN.md)

Go-Spring is **not released yet**. The whole stack — `stdlib`, `log`, `spring`,
`cloud`, the starters and the tooling — is still being polished. APIs, package
boundaries and configuration contracts may change without a compatibility
period.

## Breaking changes are allowed

During this phase, **correctness and principle take precedence over backward
compatibility**:

- Prefer the design that follows the project's principles over one that keeps an
  old API alive.
- A large-scale breaking change is allowed when it removes a class of bugs or
  restores a boundary. Do not add shims, aliases or deprecation layers just to
  preserve an unreleased shape.
- When changing an API, update every call site in the workspace, the tests and
  the docs in the same change. The workspace must build and pass tests as one
  unit.
- State the reason in the change description: which principle or defect drove
  it, and what was removed. No compatibility note is needed for unreleased code.

Stability statements elsewhere in the docs — for example "the sacred core does
not take breaking changes" — describe the **post-release** commitment. They do
not apply to the current polishing phase.

When the project is released, this file will be replaced by a versioning and
compatibility policy.
