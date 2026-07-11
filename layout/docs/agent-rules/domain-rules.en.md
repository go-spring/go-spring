# Domain Layering Rules (Itemized)

Hard rules for layering, boundaries, errors, transactions, and testing under domain layering, distilled from `docs/layout-guide/domain-layout.md`, for AI collaborators to follow directly. See `docs/layout-guide/domain-layout.md` for explanations, rationale, and counter-examples.

## Four-Layer Boundaries

Dependency direction is one-way: `api → application → domain ← infra`. `pkg` / `consts` may be referenced by any layer and must not depend on business packages in return.

| Layer | Allowed | Forbidden |
|---|---|---|
| `api` | `application`, `pkg`, `consts`, protocol-generated code | depending on `domain` / `infra`; returning `domain` objects; encoding business rules |
| `application` | `domain`, `infra`, sibling application services within the same BC (single-directional, no cycles), `pkg`, `consts` | depending on protocol-generated code or `api/server`; implementing core business rules; passing external models into `domain` |
| `domain` | its own package, `pkg`, `consts/errno`, `errutil` | depending on `api` / `application` / `infra` / ORM / RPC SDK / IDL / DTO |
| `infra` | `domain`, `pkg`, `consts`, external SDKs | depending on `api`; orchestrating business use cases; deciding business rules |

Additional points:

- `api/controller/<biz>/` holds protocol-agnostic business entry points; `api/server/<proto>/handler.go` aggregates controllers via embedding. **Do not** put method bodies in `handler.go`.
- MQ consumer entry lives in `api/server/mqsvr`; MQ producer lives in `infra/mq`.
- `infra/client/<system>/` is itself the anti-corruption layer. **Do not** propose extracting a separate `acl/`.
- Blank imports that exist only to trigger `init()` registration **must** be consolidated in the composition root `init.go`, and **must not** be scattered across business packages such as `infra/repo`. A business package only directly imports the symbols it uses (e.g. `gorm.io/gorm`); the bean-registering starter belongs in `init.go`.
- Sibling subdomain collaboration within a BC goes through `application` services in a single direction; cross-BC or external system calls **must** go through `infra/client/`.
- **Do not** predefine interfaces for single-implementation dependencies. Only extract interfaces when there is a real trigger — multi-implementation, read/write separation, caching decorator, etc.

## Error Boundaries

- `domain` **must not** depend on DB / RPC / SDK error types. `api` only maps to protocol responses; it must not encode business decisions.

## Transactions, Idempotency, Domain Events

- One `application` service method equals one transaction unit. **Do not** leak the transaction object into `domain` or across services.
- Idempotency lives in `application` (checked by business ID / request ID). `infra/repo` may back it up with unique indexes or optimistic locks.
- Cross-aggregate / cross-BC consistency **must not** use distributed transactions; use domain events, compensation tasks, or eventual consistency.
- Domain events are produced by aggregate roots, collected by `application` within the transaction boundary, and published after persistence succeeds (the publish side is a pending implementation for the project to fill in per its event mechanism — this is a gap, not a design contradiction).

## Testing Strategy

- `domain` — pure unit tests; construct aggregates directly and verify invariants.
- `application` — unit tests, replace `infra` dependencies with Go-Spring Mock. **Do not** predefine interfaces for every dependency just for testing.
- `infra/repo` — integration tests against a real DB or containerized middleware.
- `infra/client` — contract tests focused on model and error mapping.
- `api` — end-to-end tests that boot the full server and drive the protocol entry.

## Landing Checklist (Pre-Submit Self-Check)

- Business rules are concentrated in `domain`, not scattered across `api` / `application`.
- `api` only does protocol adaptation, parameter validation, and DTO conversion.
- `application` expresses a single use case with clear transaction and idempotency boundaries.
- `infra/client` isolates external protocol, model, and error semantics; `infra/repo` does not leak ORM models upward.
- Errors carry context via `errutil.Explain` / `errutil.Stack`; error codes go through `consts/errno`.
- No interfaces, `manager`s, `common` packages, or abstractions introduced without a trigger.
- Tests cover `domain` invariants along with key orchestration and infrastructure semantics.

## AI Collaboration

- For aggregate boundaries, state machines, invariants, and cross-BC collaboration, **align with a human first**. Do not decide on the team's behalf.
- Mechanical work like converter / assembler / DTO / test cases / dependency checks may be loosened, but inputs and outputs still need to be spelled out.
- On delivery, always state: which files changed, where the rules landed in `domain`, what `application` orchestrated, whether any interface was introduced and its trigger, and the test results.
