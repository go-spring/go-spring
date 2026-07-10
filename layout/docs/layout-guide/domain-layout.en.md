# Domain Layered Architecture

This doc covers the internal domain-layered architecture of a Go-Spring single-service project under the `domain` form. The top-level directories are described in a separate doc; this one focuses on the domain model, four-layer boundaries, code ownership, evolution constraints, and rules for collaborating with AI inside `internal/`.

It answers four questions:

- **When domain layering fits**: which projects need it, which can stay lighter.
- **How domain code is organized**: from protocol entry, use case orchestration, domain rules down to infrastructure implementation, each with a clear home.
- **How boundaries stay clean over time**: dependency direction, transactions, errors, tests, naming, and evolution triggers stay consistent.
- **How to collaborate with AI**: give AI enough context and demand that it generate, modify, and self-check code by the layering rules.

## How to read this doc

Recommended order:

1. When choosing an architecture for a new project, read "Architecture positioning" and "Scope of use" first, and decide whether domain layering is needed.
2. When laying out a domain-layered skeleton, follow "Domain layered directory structure" for `internal/`.
3. When writing business code, refer to "Code ownership cheat sheet", "Layer and boundary constraints", and "End-to-end flow example".
4. When reviewing code, focus on "Error handling", "Transactions and idempotency", "Testing strategy", and "Common misuses".
5. When letting AI help implement, use the context checklist and task template in "Collaborating with AI" directly.

## Architecture positioning

This architecture targets single-service Go projects with high domain complexity. It keeps the parts of DDD tactical design that best constrain complexity — aggregate roots, rich models, value objects, domain events, clear use case orchestration, strict dependency direction — while combining Go's implicit interfaces, Go-Spring IoC, and intra-service layering. It trims out premature abstraction and heavy ceremony.

### Scope of use

| Scenario | Applicable? | Basis |
|---|---|---|
| Simple CRUD, admin panels, small tools | Not preferred | Few business rules; a plain handler / service / repo split is lighter. |
| Explicit state machines, money, inventory, permissions, fulfillment, etc. | Applicable | Invariants need long-term concentration and belong in `domain`. |
| One service accepts multi-protocol entries (HTTP / RPC / MQ / Job) | Applicable | Inbound protocols need isolation in `api`, keeping use cases and domain models clean. |
| Depends on multiple external systems or third-party SDKs | Applicable | External models and error semantics need anti-corruption via `infra/client`. |
| Subdomain boundaries stabilizing; may split services in the future | Applicable | Organize by BC or subdomain inside the service first — lower cost to split later. |
| Requirements still evolving quickly; terminology unstable | Use with care | Keep a simpler structure first; migrate to domain layering after rules settle. |

The core principle: complexity **must** come from the business itself, not from architectural ceremony. **Do not** pre-create interfaces, ports, adapters, or cross-layer abstractions by default; introduce new structure only when a real change point already exists.

### Relationship to DDD

This architecture adopts DDD's key constraints without pursuing full ceremony. Whatever helps constrain domain complexity is kept by default; anything that only exists for form or premature abstraction is omitted until a real trigger appears.

| DDD element | Adoption |
|---|---|
| Aggregate roots and rich models | **Required**: business rules and invariants live in `domain`; **forbidden** to degrade domain objects to anemic field bags. |
| Value objects | **Required**: express immutable domain concepts — money, time windows, addresses, status combinations. |
| Domain events | **Recommended, on demand**: aggregates produce events; `application` collects them inside the transaction boundary and publishes them after persistence, or hands them to the project's event mechanism. |
| Layered architecture | **Required**: `api`, `application`, `domain`, `infra` responsibilities are separated; **reverse dependencies forbidden**. |
| Repository interfaces | **Not preset by default**: `application` may depend on `infra/repo` concrete types directly; extract an interface only when a real change point appears — multiple impls, read/write split, caching decorators, etc. |
| Anti-corruption layer | **Required** across BCs or external systems; **not enforced** for sibling subdomains inside the same BC. |
| Explicit interface declaration | **Not by default**: single-impl dependencies use concrete types; declare an interface only when there is polymorphism, substitution, or a stable cross-package contract. |

## Domain layered directory structure

The baseline structure below shows the recommended layout inside `internal/` under the `domain` form. `order` and `user` are just illustrative business domains — replace with your project's actual BCs, subdomains, or core business objects.

```
internal/
├── api/                                # Inbound adapter: protocol, routing, request entry, jobs, MQ triggers
│   ├── job/                            # Scheduled jobs, background jobs — non-request entries
│   ├── controller/                     # Protocol entry handlers, split vertically by business domain
│   │   ├── order/                      # Order domain
│   │   │   ├── converter/              # IDL request/response models ↔ application DTO converters
│   │   │   └── order_controller.go     # Order business entry implementation
│   │   └── user/                       # User domain
│   │       ├── converter/              # IDL request/response models ↔ application DTO converters
│   │       └── user_controller.go      # User business entry implementation
│   └── server/                         # Protocol entry aggregation
│       ├── httpsvr/                    # HTTP protocol adapter
│       │   ├── middleware/             # HTTP-specific middleware chain
│       │   ├── handler.go              # HTTP handler; composes business controllers, implements protocol interface
│       │   └── httpsvr.go              # HTTP server lifecycle
│       ├── thriftsvr/                  # Thrift RPC adapter
│       │   ├── middleware/             # Thrift interceptors and tracing
│       │   ├── handler.go              # Thrift handler; composes business controllers, implements protocol interface
│       │   └── thriftsvr.go            # Thrift server lifecycle
│       ├── grpcsvr/                    # gRPC adapter
│       │   ├── middleware/             # gRPC Unary / Stream interceptors
│       │   ├── handler.go              # gRPC handler; composes business controllers, implements protocol interface
│       │   └── grpcsvr.go              # gRPC server lifecycle
│       └── mqsvr/                      # MQ consumer entry and lifecycle
│           ├── middleware/             # MQ-specific middleware chain
│           ├── handler.go              # MQ consumer handler; composes controllers, implements consume interface
│           └── mqsvr.go                # MQ consumer registration, start, stop
├── application/                        # Application layer: orchestrates domain objects and infrastructure
│   ├── order/                          # Order domain
│   │   ├── assembler/                  # DTO ↔ Entity / aggregate converters
│   │   ├── dto/                        # DTOs exposed by application
│   │   └── order_service.go            # Order application service — use case entry
│   └── user/                           # User domain
│       ├── assembler/
│       ├── dto/
│       └── user_service.go
├── domain/                             # Domain layer: core business rules and logic
│   ├── order/                          # Order domain
│   │   ├── order.go                    # Aggregate root and entities
│   │   ├── order_value.go              # Value objects
│   │   └── order_event.go              # Domain events
│   └── user/                           # User domain
│       ├── user.go
│       ├── user_value.go
│       └── user_event.go
├── infra/                              # Infrastructure impl: external systems and middleware
│   ├── repo/                           # Repository impl; wraps DB / cache
│   │   ├── order/                      # Order repo impl
│   │   │   └── order_repo.go
│   │   └── user/                       # User repo impl
│   │       └── user_repo.go
│   ├── client/                         # External service / SDK calls; anti-corruption layer
│   │   └── uranus/                     # Uranus platform client
│   └── mq/                             # MQ producer; the consumer entry stays in api/server/mqsvr
├── pkg/                                # Service-local utilities; no business semantics
│   ├── stringutil/
│   ├── timeutil/
│   └── safego/
├── consts/                             # Globally shared constants
│   └── errno/                          # Global error code definitions
└── init.go                             # Side-effect imports and registration aggregation entry
```

### Code ownership cheat sheet

When unsure where a piece of code belongs, decide by responsibility first, then verify by dependency direction.

| Code content | Location | Criterion |
|---|---|---|
| HTTP / RPC / MQ / Job entry, routing, protocol handler | `api/server/*`, `api/job` | Only handle the inbound protocol and lifecycle; no business rules. |
| Request validation, protocol model ↔ DTO conversion | `api/controller/<biz>/converter` | Input comes from IDL / protocol; output is application DTO. |
| Flow of a business use case | `application/<biz>/*_service.go` | Coordinates aggregates, repositories, external services, transactions, and idempotency. |
| DTO ↔ domain object conversion | `application/<biz>/assembler` | Conversion happens at the application/domain boundary. |
| State transition, money calculation, stock deduction, permission checks | `domain/<biz>` | These are domain invariants; must be expressed by aggregate roots or domain objects. |
| Immutable domain concept | `domain/<biz>/*_value.go` | Business-semantic; **must** be immutable, not freely mutated from outside. |
| Facts that have already occurred in the domain | `domain/<biz>/*_event.go` | Events are produced by aggregates; names express past-tense facts. |
| DB / cache persistence impl | `infra/repo/<biz>` | Wraps GORM, SQL, Redis, etc. |
| RPC / SDK / third-party platform calls | `infra/client/<system>` | Cross-BC or external systems; must adapt protocol, model, and errors. |
| MQ producer | `infra/mq` | Outbound message capability; the consumer entry stays in `api/server/mqsvr`. |
| Business-agnostic utility functions | `pkg/<func>` | Referenced by multiple layers; must not carry business concepts. |
| Error codes, global constants | `consts/errno`, `consts/*` | Cross-layer and stable; do not scatter in business code. |

## Layer and boundary constraints

The four layers are the core lever for controlling complexity: `api` handles inbound protocol adaptation, `application` orchestrates use cases, `domain` carries business rules, `infra` provides outbound technical implementations. Code ownership goes by responsibility, not by current file location.

### Four-layer responsibilities

| Layer | Responsibility | Forbidden |
|---|---|---|
| `api` | Accept HTTP / RPC / MQ / Job inbound traffic; do protocol parsing, request validation, request/response model conversion. | **Do not** call `infra` directly; **do not** carry business rules; **do not** return `domain` objects. |
| `application` | Organize the use case flow; orchestrate `domain` aggregates and `infra` capabilities; own transaction boundary, idempotency, and domain event publishing. | **Do not** implement core business rules; **do not** leak the transaction object; **do not** pass external models into `domain`. |
| `domain` | Define aggregate roots, entities, value objects, domain events; carry business invariants and domain behavior. | **Do not** depend on `api` / `application` / `infra`; **do not** know about DB, RPC, SDK, IDL, DTO. |
| `infra` | Implement DB, cache, RPC, SDK, MQ producer, etc.; hide external system differences. | **Do not** orchestrate use cases; **do not** decide business rules; **do not** face protocol entries directly. |

### Dependency direction

Dependency direction must be clear and checkable. `pkg` and `consts`, having no business semantics, may be referenced by every layer, but must not depend on business packages in return.

```
                ┌────────────────────────┐
                │       pkg / consts     │
                └───────────▲────────────┘
                            │
     ┌──────────────┬───────┴──────┬──────────────┐
     │              │              │              │
  ┌───────┐    ┌───────────┐    ┌────────┐    ┌────────┐
  │  api  │───▶│application│    │ domain │◀───│ infra  │
  └───────┘    └─────┬─────┘    └────▲───┘    └───▲────┘
                     │               │            │
                     └───────────────┴────────────┘
```

Allowed dependencies:

| Source | Allowed | Forbidden |
|---|---|---|
| `api` | `application`, `pkg`, `consts`, protocol-generated code | `domain`, `infra` |
| `application` | `domain`, `infra`, sibling application services within the same BC (single-directional, acyclic), `pkg`, `consts` | protocol-generated code, `api/server` |
| `domain` | its own package, `pkg`, `consts/errno`, `errutil` | `api`, `application`, `infra`, ORM, RPC SDK, IDL, DTO |
| `infra` | `domain`, `pkg`, `consts`, external SDK / DB / MQ deps | `api`, protocol entry handlers |

Notes:

- `infra/repo` may depend on `domain` because repository implementations translate storage models into domain aggregates.
- `application` may depend on `infra` concrete types — that is the default simplification for Go-Spring projects.
- Sibling `application` subdomains within a BC may collaborate in one direction; **cyclic dependency is forbidden**. Once bidirectional calls appear, switch to domain events, read-only query services, or redraw use case boundaries.
- `domain` must remain the most stable. Any external protocol, storage structure, or SDK type entering `domain` means the boundary has been breached.

### Cross-domain collaboration and the anti-corruption layer

Distinguish two kinds of boundary first:

- **Sibling subdomains inside the same BC**: **recommended** to collaborate single-directionally via `application` services — e.g. `order` orchestrating `user`'s query service. **Do not** import the other side's `domain` model directly. Once cycles or bidirectional dependencies appear, switch to domain events, read-only query services, or redraw use case boundaries.
- **Cross-BC or external systems**: **must** be isolated by an anti-corruption layer under `infra/client/`. Business layers only program against local domain models; they must not be aware of external protocols, naming, error codes, or SDK types.

The anti-corruption layer takes on at least three duties:

- **Protocol adaptation**: keep HTTP / RPC / SDK call details inside `infra/client/<system>`.
- **Model mapping**: convert external models into local domain-understandable value objects, DTOs, or query results.
- **Error translation**: wrap external errors into contextualized internal errors, mapping to `consts/errno` when necessary.

The trigger is simple: crossing a BC boundary, external model semantics diverging from the local domain, or involving network IO / RPC / third-party SDK all **require** an `infra/client/`.

## Domain modeling conventions

The domain layer is not a field-definition folder; it is where business rules land. When writing `domain` code, express behavior with methods first, rather than letting outer layers mutate fields freely.

### Aggregate root

The aggregate root is the only entry for external mutation of aggregate state. It maintains invariants inside its aggregate — legality of state transitions, matching amounts, allowed stock deductions.

Conventions:

- **Recommended**: name aggregate root methods after business actions, e.g. `Pay`, `Cancel`, `ConfirmReceipt`, rather than exposing `SetStatus`.
- **Required**: the aggregate root maintains its aggregate's invariants; outside callers can only trigger state changes through aggregate root methods.
- **Forbidden**: aggregate roots must not start transactions, access repositories, or call external services.
- **Allowed**: aggregate roots may produce domain events, but **must not** publish them themselves.

### Entities and value objects

Entities have identity and lifecycle; value objects express immutable business concepts.

Conventions:

- **Required**: use ID to identify an entity — same-field objects may still be different entities.
- **Required**: value objects must be immutable; on change, create a new value.
- **Required**: value objects must validate themselves — e.g. money cannot be negative, time windows must be well-ordered.
- **Forbidden**: never use DTOs, ORM models, or IDL-generated types as entities or value objects.

### Domain events

Domain events represent facts that have already happened in the domain, usually produced by an aggregate root during state change.

Conventions:

- **Recommended**: name events as past-tense facts, e.g. `OrderPaid`, `OrderCanceled`.
- **Recommended**: event fields contain only stable information the subscribers need; **do not** expose the whole aggregate.
- **Required**: `application` collects events inside the transaction boundary and publishes them after persistence succeeds, or hands them to the project event mechanism.
- **Recommended**: use domain events for decoupling across aggregates, subdomains, or asynchronous compensation. **Do not** use events to bypass explicit synchronous use case orchestration.

## End-to-end flow example

Take "user places order" — one full request goes through the four layers as follows:

```
HTTP request
   │
   ▼  api/server/httpsvr/handler.go          protocol parsing, route dispatch
   │
   ▼  api/controller/order/order_controller  request validation, request → application DTO
   │
   ▼  application/order/order_service        use case orchestration, transaction, orchestrating domain and infra
   │        │
   │        ├── domain/order                  invariant checks, produce domain events
   │        │
   │        ├── infra/repo/order              persist the order aggregate
   │        │
   │        └── infra/client/uranus           call external systems, anti-corruption
   │
   ▼  application/order/assembler             Entity / aggregate → DTO
   │
   ▼  api/controller/order/converter          DTO → protocol response
   │
   ▼  HTTP response
```

Key flow rules:

- **Input conversion**: protocol models become `application/*/dto` in `api/controller/*/converter`. `domain` and `infra` **must not** know protocol models.
- **Use case orchestration**: `application` fetches aggregates, invokes domain behavior, persists, calls external services, handles transactions and idempotency.
- **Business rules**: all invariant checks **must** be carried by `domain` aggregate roots, entities, or value objects.
- **Output conversion**: `domain` objects become DTOs in `application/*/assembler`, then response models in `api/controller/*/converter`.
- **External calls**: `application` calls `infra/client` local-domain-semantic methods. **Do not** handle external protocol models directly.

## Layered error handling conventions

Errors flow one-way along the layers. Each layer handles only the errors it recognizes; when crossing a layer, add context — never swallow the original error.

| Layer | Error source | Handling |
|---|---|---|
| `domain` | Business-rule errors — illegal state, out-of-range amount, insufficient stock | Use `consts/errno` to express business error codes; add context via `errutil.Explain` when needed. |
| `infra` | Technical errors — DB disconnection, RPC timeout, SDK exception | Preserve the call path via `errutil.Stack`; map to a technical error code in `consts/errno` when necessary. |
| `application` | Orchestration failures, transaction failures, composed cross-dependency errors | Add business context via `errutil.Explain` — order ID, user ID, external system name. |
| `api` | Parameter errors, protocol errors, downstream errors | Uniformly map `errno` codes to protocol responses — HTTP status, Thrift exception, gRPC status. |

Key conventions:

- Every returned error **must** carry enough context to pinpoint the failing use case, key ID, or external dependency.
- `errutil` handles stack and context; `errno` handles error code definitions. They are orthogonal.
- `domain` **must not** depend on DB / RPC / SDK error types, and **must not** make business decisions based on technical errors.
- `api` only presents errors to the outside; **do not** patch business decisions in the protocol layer.

## Transactions and idempotency

Transactions and idempotency are use case boundary concerns owned by `application`. They must not scatter into `api`, `domain`, or `infra`.

- **Transaction boundary**: one `application` service method equals one transaction unit; begin and commit inside the method. **Do not** expose the transaction object upward.
- **Single-aggregate strong consistency**: multiple state changes within an aggregate root commit in the same transaction. `domain` only operates on in-memory aggregates and **must not** know about the transaction object.
- **No distributed transactions across aggregates**: for consistency across multiple aggregates or BCs, **prefer** domain events, compensation tasks, or eventual consistency. Avoid long transactions and cross-DB 2PC.
- **Idempotency belongs to `application`**: the entry checks idempotency by business ID / request ID; `infra/repo` may back it up with unique indexes or optimistic locking.
- **No nested transactions across services**: for composite flows, **prefer** promoting to a new orchestration service instead of having services call each other and wrapping a transaction outside.

## Extension and evolution

The default form stays simple. Only when complexity actually appears and the trigger is explicit, adopt the corresponding evolution path.

| Default | Trigger | Evolution |
|---|---|---|
| `application` depends on `infra/repo` concrete types | Multiple repository implementations appear — primary/replica, cache decorator, read/write split, swap storage | Extract a repository interface. If it expresses aggregate persistence semantics, put it in `domain/<biz>`; if it is only an application orchestration port, put it in the consumer's package. |
| Sibling subdomains collaborate via `application` services within the same BC | Multi-team, multi-repo collaboration, or frequently shifting subdomain boundaries | Extract a stable contract; introduce an anti-corruption layer when needed. |
| Keep the four-layer directories inside the service | A subdomain needs independent deployment, release, or scaling | Extract the subdomain into a standalone service, reusing the same layered structure. |
| Tests default to Go-Spring Mock-based dependency replacement | Strong test-framework-independence requirement, or critical dependencies need multi-impl swapping | Extract interfaces only for real change points; keep other dependencies concrete. |
| `infra/client` wraps external systems directly | External model is complex, call scenarios vary, error semantics unstable | Split the internals of `infra/client/<system>` into mapper, transport, error adapter — but do not leak them upward. |

Interface extraction rules:

- **Forbidden**: extract interfaces for "future possibilities".
- **Forbidden**: extract interfaces solely for unit tests.
- **Recommended**: place ordinary interfaces in the consumer's package. Repository interfaces belong in `domain` only when they express a domain persistence contract.
- Once extracted, the interface **must** describe stable semantics, not just mirror a single impl's method list.

## Engineering conventions

Beyond layering and boundaries, standardize naming, assembly, registration entries, and lifecycle management.

### File naming

Whether a filename carries a domain prefix depends on whether the file's types are recognized by outside packages.

- **Prefix recommended**: types are injected, embedded, or constructed across packages — controller, service, SDK impl. Example: `order_controller.go`, `order_service.go`, `order_sdk.go`.
- **No prefix recommended**: files only serve in-package helper roles — converter, assembler, dto. Example: `converter.go`, `dto.go`, `assembler.go`.

### Handler assembly

`api/server/<proto>/handler.go` uses embedding to aggregate business types under `api/controller/`. It carries no business method bodies itself. New business methods are added in the controller package; the handler picks them up via embedding. Even when the protocol interface grows to many methods, the handler file does not swell with business.

### pkg organization

`pkg/` carries only business-agnostic utilities. **Recommended**: split by function — `stringutil/`, `timeutil/`, `sliceutil/`, `safego/`. **Forbidden**: umbrella packages like `common/`, `goutil/`, `helper/`.

### init.go imports

`init.go` only handles side-effect imports for entries or registration, categorized by traffic direction:

- **Inbound entries**: `api/server/*`, `api/job`, etc. **must** be written in `init.go` to trigger route / job / consumer registration.
- **Outbound dependencies that are imported directly**: **do not** write them in `init.go` — the upstream import triggers initialization naturally.
- **Outbound capabilities with no direct import but with registration logic in `init()`**: **must** be written in `init.go` — e.g. MQ producers or plugins requiring auto-registration, or a starter pulled in solely to register beans.
- **Collect blank imports in one place**: every side-effect (blank) import that exists only to trigger `init()` registration **must** be consolidated in the composition root `init.go`, and **must not** be scattered across business packages such as `infra/repo`. A business package only directly imports the symbols (types, functions) it actually uses — e.g. a repo using GORM imports `gorm.io/gorm`, but `starter-gorm-mysql`, which registers `*gorm.DB`, belongs in `init.go`. This makes enabled components visible at a glance and keeps component removal a one-file edit.

### Assembly and lifecycle

All cross-layer dependencies are wired via the Go-Spring IoC container. Do not hold dependencies long-term through package-level singletons or globals.

- **Bean declaration is co-located**: **recommended** to declare beans in the file where their implementation lives; **not recommended** to gather them into a central assembly file, which would form a central dependency.
- **Startup order is the container's job**: business code **must not** control initialization order explicitly. When a hard order is required, **prefer** `PostConstruct` / `Destroy` hooks.
- **Graceful shutdown per layer**: `api/server/*` drains and closes inbound connections; `infra/*` releases outbound resources — connection pools, producers, subscriptions.
- **Do not bypass the container**: dependencies between `application` and `infra` **must** be obtained by IoC injection. **Do not** `New*()` inside a package and hold the result long-term.

## Layered testing strategy

The testing strategy maps to the layers. Each layer uses the test type that best fits its responsibility. Do not cover unit logic with integration tests, or pretend unit tests are integration tests.

| Layer | Test type | Dependency handling |
|---|---|---|
| `domain` | Pure unit tests | No external deps; construct aggregates, entities, value objects directly; verify invariants and domain behavior. |
| `application` | Unit tests | Replace `infra` deps with Go-Spring Mock; focus on orchestration, transaction, idempotency branches. |
| `infra/repo` | Integration tests | Connect to a real DB or containerized middleware; verify persistence and query semantics. |
| `infra/client` | Contract tests | Use test doubles for third-party SDKs / services; focus on model and error mapping. |
| `api` | End-to-end tests | Boot the full server, drive requests through the protocol entry, verify the full chain. |

Additional principles:

- `domain` unit tests are the highest priority; invariants **must** be guaranteed at this layer.
- **Do not** predefine interfaces for every `application` dependency just for testing. When Go-Spring Mock can substitute concrete types, **prefer** concrete dependencies.
- Integration tests should **prioritize** `infra/repo` and key `infra/client`, to keep the pyramid from inverting.
- Each new use case **at minimum** ships with domain-rule tests; when a protocol entry is involved, add controller or end-to-end tests as well.

## Collaborating with AI

Domain-layered projects fit AI-assisted skeleton generation, converter completion, test writing, and dependency boundary checks well — but only when clear context is provided. The practical takeaway is: treat AI as an efficient implementer, reviewer, and test completer, not as a decision maker for domain boundaries. Without enough context, AI is prone to putting business rules into `application`, or generating piles of interfaces, managers, and common packages.

### Collaboration boundaries

Boundaries for AI collaboration scale by risk: the clearer the rules and the more mechanical the change, the more it can be handed off; the heavier the domain judgment and the harder to revert, the more a human must set direction first.

| Type | Suggestion | Note |
|---|---|---|
| converter, assembler, DTO, tests, dependency checks | **Can be loosened** | I/O is clear, review is easy, big AI productivity gain. |
| application use case orchestration | **Conditionally loosened** | Provide transaction boundary, idempotency key, error codes, and external deps first. Missing any, ask AI to ask first. |
| Aggregate boundaries, state machines, invariants, cross-BC collaboration | **Must be tightened** | Domain design decisions. AI can propose approaches and risks — humans decide. |
| New interfaces, abstraction layers, `manager` / `processor` structures | **Forbidden by default** | Unless a real trigger exists — otherwise AI tends to "decouple" its way into complexity. |
| Large-scale refactor | **Staged execution** | AI first outputs impact scope and migration steps; modify in small steps and run tests; **do not** cut through many layers at once. |

### Tasks that fit AI

| Task | Fit | Constraint |
|---|---|---|
| Skeleton for a new use case — controller / DTO / service / domain method | Fits with conditions | Business action, aggregates involved, transaction boundary, idempotency key, error codes must be explicit; otherwise AI asks first. |
| Filling converter / assembler | Great fit | Source model, target model, and field semantics must be explicit — do not let AI guess external field meanings. |
| Writing `domain` unit tests | Great fit | Provide invariants, legal state transitions, and counter-examples. |
| Checking dependency direction | Great fit | Ask AI to enumerate import violations, DTO leaks, cross-layer calls. |
| Adding error wrapping, log context, test coverage | Fits | Provide error codes, key IDs, run commands; AI must not swallow underlying errors. |
| Extracting interfaces or refactoring boundaries | Careful | State the trigger explicitly; AI must not abstract for "decoupling" on its own. |
| Designing aggregate boundaries or cross-BC collaboration | Discussion only | AI outputs options, constraints, risks; humans confirm boundaries before code lands. |

### Context checklist for AI

Before letting AI change code, **provide at minimum**:

- **Business action**: what use case is being implemented — "user places order", "pay order", "cancel order".
- **Domain terms**: aggregate roots, entities, value objects, state enums, key business rules.
- **Boundary notes**: what is in the same BC, what is external or cross-BC.
- **I/O**: protocol request/response, application DTO, error codes to return.
- **Transactions and idempotency**: transaction scope, idempotency key, concurrent conflict handling.
- **External deps**: DB tables, cache keys, RPC / SDK, MQ topics, and error mapping requirements.
- **Test requirements**: positive/negative cases, edge cases, test commands to run.

If any item is missing, **recommended** to ask AI to list gaps and assumptions first. Unless the task is only converter / assembler / test completion / dependency check, do not let AI implement with incomplete business rules.

### AI task template

Use the template below to hand tasks to AI:

```text
Please implement "<business action>" under internal.

Business rules:
- Aggregate root: <name>
- State transitions: <from → to>
- Invariants: <money, stock, permission, time window rules>
- Error codes: <errno name or new-request>

Layering requirements:
- api only converts protocol models and application DTOs; must not call infra.
- application handles transactions, idempotency, orchestrating domain and infra.
- domain carries business rules; must not import api / application / infra / IDL / ORM / SDK.
- infra only wraps DB, cache, RPC, SDK, or MQ producers.
- If business rules, error codes, transaction boundary, or external deps are unclear, ask first — do not fabricate.

External deps:
- repo: <data to read/write>
- client: <external systems to call>
- mq: <messages to send, or none>

Test requirements:
- domain unit tests cover: <rules>
- application tests cover: <orchestration and error branches>
- Run command: go test ./...

On delivery, please state:
- Files changed.
- Which rules landed in domain, which orchestration landed in application.
- Whether an interface was added; if so, the trigger.
- Test results.
```

### AI self-check list

Before delivery, ask AI to run through the "Landing checklist" at the end of this doc, and additionally confirm:

- Whether it documented which aggregate, entity, or value object holds the business rule in `domain`.
- Whether it documented what orchestration, transaction, idempotency, or event publishing `application` did.
- Whether it listed the assumptions used; if any are uncertain, whether it stopped and asked instead of implementing.
- Whether a new interface was added; if so, the real trigger.
- Whether it listed test coverage points and actual run results.

### AI collaboration comments

When a piece of code has an explicit constraint and might be misedited by AI later, use a local comment to constrain behavior, e.g.:

```go
// AI: do NOT move this validation to application; it is a domain invariant.
```

Such comments are **only for critical boundaries**. **Do not** sprinkle them everywhere. Say "what must not be done" and "why", avoiding noise that just restates the code.

## Landing checklist

Before merging, check quickly against the following:

- Are business rules concentrated in `domain`, not scattered across `api` or `application`?
- Does `api` only do protocol adaptation, parameter validation, and DTO conversion?
- Does `application` clearly express one use case with transaction and idempotency boundaries?
- Does `infra/client` isolate external protocol, model, and errors?
- Does `infra/repo` hide storage details, without leaking ORM models upward?
- Are errors wrapped with `errutil.Explain` / `errutil.Stack` to keep context?
- No trigger-less interfaces, managers, common packages, or over-abstractions?
- Do the tests cover `domain` invariants plus key orchestration and infrastructure semantics?

## Common misuses and anti-examples

Rules are easy to remember; the architecture usually degrades through "looks-like-it-works" patterns. During reviews, watch for the following:

| Anti-example | Problem | Correction |
|---|---|---|
| `domain` imports `infra/repo`, `gorm`, RPC SDK, or IDL types | Business rules coupled to tech impl or external protocols; violates dependency direction. | Keep domain objects and business rules in `domain`; persistence, protocols, SDKs are converted by outer layers. |
| `api/controller` calls `infra/repo` or `infra/client` directly | Bypasses `application`; use case orchestration and protocol entry mixed. | All cross-layer calls go through an `application` service. |
| `application` implements price calculation, state machine, stock rules | `domain` is hollowed out into a CRUD data structure. | Move rules back into the aggregate root, entity, or value object; `application` only orchestrates. |
| `api` returns `domain.Entity` directly | Domain model leaks to protocol layer; changes drag on external contracts. | Convert twice — `application` DTO and `api` converter. |
| `infra/repo` returns an ORM model to `application` | Storage structure leaks into the use case layer; domain objects lose meaning. | repo returns a domain aggregate or the query result `application` needs. |
| Cross-BC direct import of the other side's `domain` model | Breaks BC isolation; splitting later becomes hard. | Call via `infra/client/` anti-corruption; both sides only face their own domain models. |
| Interfaces pre-defined for single-impl dependencies | Adds cognitive cost, and the interface usually cannot predict future variation. | Keep concrete-type dependencies; extract an interface when a real polymorphism need appears. |
| Adding `common/`, `goutil/`, `helper/` under `pkg/` | Utility scope disappears; grows unmaintainable. | Split by function; each subpackage carries one kind of capability. |
| Transaction object passed from `application` into `domain`, or across multiple services | Transaction boundary leaks; the call chain becomes hard to understand and test. | One application service owns the whole transaction boundary. |
| AI generates many `manager` / `processor` / `adapter`s with unclear responsibilities | Names hide responsibility; directory complexity rises. | Return to the four layers; name by business action, aggregate, or external system. |
