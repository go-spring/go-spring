# Go-Spring Architecture & Boundaries

[English](ARCHITECTURE.md) | [中文](ARCHITECTURE_CN.md)

This is the **authoritative map** of what lives in each top-level directory, what
must *not* live there, and how new code is routed to the right place. Its job is
to keep the repository from drifting: when in doubt about *where* something
belongs or *whether* it belongs at all, this document decides.

It is a map, not an encyclopedia. Per-module design rules live in their own
`DESIGN.md` files; this document links to them rather than repeating them (see
[CLAUDE.md](CLAUDE.md), "When to Record a Convention": link, don't repeat).

## 1. The Layered Model

The repo has **no `go.mod` at the root**. Every subproject owns its own module
and dependency graph. Modules are organized into layers, and **dependencies flow
one way only — never in reverse**:

```
  foundation        core            integration            tooling
 ┌──────────┐   ┌──────────┐   ┌──────────────┐   ┌──────────────┐
 │ stdlib/  │──▶│          │──▶│ starter/     │   │ gs/          │
 │ log/     │   │ spring/  │   │  starter-*   │   │  gs-*        │
 └──────────┘   └──────────┘   └──────────────┘   └──────────────┘
       │              │                │                  │
       └──────────────┴────────────────┴──────────────────┘
                            ▲
              consumed by demos & template (never depended on in reverse)
        contrib/   examples/   layout/            (+ website/ docs/ scripts/ skills/)
```

Verified dependency facts (do not violate):

- `stdlib/` has **zero third-party dependencies** — standard library only. It is a
  general-purpose utility library (a completion of the Go standard library:
  types, encoding, collections, ...); it holds **no capability abstractions**.
- `log/` depends on `stdlib/` (plus an ANTLR parser for its config grammar); it is
  a foundation module, not part of `spring`.
- `spring/` depends on `log/` and `stdlib/`, and on **no third-party business
  package** (no Redis, GORM, Kafka, ...). Beyond the IoC container it hosts the
  framework's **capability abstractions** (interface + driver registry) as
  subpackages, grouped by concern into families — `spring/cloud/*`
  (discovery, loadbalance, resilience, lock, messaging, transaction, event,
  scheduling, batch), `spring/web/*` (httpsvr, httpclt, httpx, security,
  session, validation, i18n), `spring/data/*` (cache, repository, migration),
  `spring/actuator/*` (endpoint, health, podinfo) — whose concrete backends
  live in `starter-*`. `spring/aspect` sits at the root as a core primitive
  (zero deps, widely depended on, alongside `gs`/`conf`). The authoritative
  family map lives in [spring/DESIGN.md](spring/DESIGN.md).
- `starter-*` and `gs-*` sit on top and may pull third-party packages.
- Nothing in a lower layer may import a higher layer. A `starter` importing
  another `starter`, or `spring` importing a `starter`, is a layering violation.

**Internal deps resolve through `go.work`, never `require`.** Adding a `require`
on an in-workspace module sends `go mod tidy` to the proxy and 404s. See the
[go.work](go.work) `use` list for the full module set.

## 2. Directory Responsibility Matrix

| Directory | Layer | Purpose (one line) | Belongs here | Does **not** belong here | Deep dive |
|---|---|---|---|---|---|
| `stdlib/` | foundation | Zero-dependency general-purpose utilities (a completion of the Go standard library) | Pure Go helpers — types, encoding, collections, hashing, text, ... | Any third-party import; capability abstractions / driver registries (those live in `spring/`); container/DI logic | [stdlib/README.md](stdlib/README.md) |
| `log/` | foundation | Structured logging model, config grammar, adapters | The logging model, appenders, field encoding, log config parser | Business logging; hard deps on `spring` | [log/DESIGN.md](log/DESIGN.md) |
| `spring/` | core | IoC container, dependency injection, app lifecycle, built-in HTTP server, conf, and the framework's capability abstractions | Bean model, injection, start/stop state machine, config binding/refresh, minimal HTTP server; capability interfaces + driver registries (cache, lock, discovery, resilience, ...) | Third-party business packages; integration code that wires a real backend; a full web framework (see §4) | [spring/DESIGN.md](spring/DESIGN.md) |
| `starter/` | integration | One module per third-party service/framework, wired into the IoC container | `starter-*` modules following the five archetypes; the family design guide | Business logic; deployment scaffolding; cross-starter shared helper packages | [starter/DESIGN.md](starter/DESIGN.md) |
| `gs/` | tooling | Dev tools: scaffolding (`gs`), GUI, code generation (`gs-http-gen`), mocking (`gs-mock`) | CLI/codegen/tooling that operates *on* projects | Runtime framework code; anything imported by a running app | [gs/README.md](gs/README.md) |
| `contrib/` | demo | Runnable examples showing how third-party frameworks are wired the Go-Spring way | Per-framework runnable variants; smoke tests | Reusable modules (those become `starter-*`); deployment scaffolding | [contrib/DIRECTORY_CONVENTIONS.md](contrib/DIRECTORY_CONVENTIONS.md) |
| `examples/` | demo | End-to-end sample applications built only from published starters | Reference apps (fullstack, bookman, ...) that *consume* the framework | New framework capabilities; code an app shouldn't need to copy | [examples/examples.md](examples/examples.md) |
| `layout/` | template | The project skeleton `gs init` stamps out | Template files, agent rules, per-protocol IDL layout | Framework implementation; anything not meant to be copied into a user project | [layout/DESIGN.en.md](layout/DESIGN.en.md) |
| `website/` | site | Documentation site **source** (Node.js) | Markdown content, site config, assets | Build output (that is `docs/`) | — |
| `docs/` | site | **Published** site output (GitHub Pages; has `CNAME`) | Generated HTML/assets | Hand-authored source (edit `website/` instead) | — |
| `scripts/` | ops | Repo-maintenance scripts | Module checks, release, history audit | App runtime code; per-project build scripts | — |
| `skills/` | agent | Agent skills shipped with the repo (e.g. `gs`) | Skill definitions | Runtime framework code | — |

## 3. Where Does New Code Go? (decision guide)

Answer top-down; the first match wins.

1. **Is it a runnable demo or reference app, not meant to be imported?**
   - Demonstrates a third-party framework wiring → `contrib/<framework>/<variant>/`
   - End-to-end app built from existing starters → `examples/`
2. **Does it integrate a specific third-party service/framework** (Redis, GORM,
   Kafka, a web/RPC framework, a config center, ...)?
   → a `starter-*` module under `starter/`. Pick the archetype from
   [starter/DESIGN.md §2](starter/DESIGN.md) — it fixes lifecycle, port, and
   config-prefix behavior.
3. **Is it a dev-time tool** (scaffolding, codegen, mocking, GUI) that operates
   *on* projects rather than running inside them? → `gs/gs-*`.
4. **Is it container / DI / lifecycle / config / built-in-HTTP logic, or a
   capability abstraction** (interface + driver registry, e.g. cache, lock,
   discovery, resilience) with no third-party business dependency? → `spring/`
   (a subpackage). If it needs a third-party import, the abstraction stays in
   `spring/` but the concrete backend belongs in a `starter`.
5. **Is it a reusable general-purpose utility** (types, encoding, collections,
   ...) with **zero third-party dependencies** and no framework/capability
   concern? → `stdlib/` (or `log/` if it is logging).
6. **Is it documentation?** Author in `website/`; never hand-edit `docs/`.

Two recurring traps:

- *"I'll just add a small helper shared by two starters."* No — cross-starter
  shared helper packages are currently disallowed
  ([starter/DESIGN.md §3](starter/DESIGN.md), "Duplication is currently tolerated
  over premature abstraction"). Duplicate first; a consolidation pass may come
  later.
- *"This abstraction needs a Redis client, I'll put it in stdlib."* No — two
  things are wrong. It is not a pure utility (it is a capability abstraction, so
  its home is `spring/`, not `stdlib/`), and the moment a third-party import is
  required it cannot live in either foundation layer. The pattern is:
  **abstraction + driver registry in `spring/`, concrete backend in a `starter`**
  (see `spring/data/cache`, `spring/cloud/lock`, `spring/cloud/discovery`).

## 4. Scope Red Lines (non-goals)

These are deliberate limits. Expanding past them is drift, not progress.

- **`stdlib/` stays dependency-free.** The value of the foundation layer is that
  any module can use it without inheriting a dependency graph. A single
  third-party import defeats the purpose.
- **`spring/` is not a web framework.** The built-in HTTP server intentionally
  does **not** provide a framework-level context object, parameter binding /
  response auto-serialization, route grouping or priority, or template rendering.
  Those belong in a web-framework `starter` (gin/echo/hertz/...). See
  [OUTLINE.md](OUTLINE.md) §五 "内置 HTTP Server".
- **`starter-*` is integration only.** A starter wires *one* third-party
  service/framework into the container and lifecycle — no business logic, no
  deployment scaffolding, no cross-starter abstractions.
- **`contrib/` and `examples/` are demos, not products.** They exist for smoke
  testing and integration demonstration. Do not add deployment scaffolding
  (`build.sh`, `bootstrap.sh`, extra `script/` dirs); keep to source +
  `smoke-test.sh` / `check.sh` / `gen.sh`.
- **Prefer framework-native mechanisms; unify only where none exists.** Do not
  layer a Go-Spring abstraction over a capability each framework already ships
  (e.g. RPC provider registration). The reasoning and the current
  have-native-vs-candidate breakdown are in
  [starter/DESIGN.md §3](starter/DESIGN.md).

## 5. Extensibility Is the Framework's Contract

Go-Spring exists to serve every team's full range of scenarios; it cannot ship a
fixed feature set and hope it fits. So in the framework layers — `stdlib/`,
`spring/`, `starter/` — **extension points are not optional**:

- **Every capability leaves a seam.** A capability abstraction defines the
  interface + driver registry; concrete behavior plugs in behind it. A scenario
  the framework didn't anticipate must still have a way in — otherwise the
  architecture fails by omission, not by a visible bug.
- **Built-ins ride the same seams they expose.** Go-Spring's own built-in
  implementations must go through the very extension points offered to users,
  never a privileged private path — `spring/data/cache`'s Memory backend,
  `spring/cloud/resilience`'s built-in strategies, and the starter archetypes all
  consume their own registries/interfaces. If a built-in can't be expressed
  through the public seam, the seam is wrong, not the built-in.
- **This is a framework-layer duty, not a universal one.** Downstream business
  code (apps stamped from `layout/`, and `examples/` / `contrib/`) follows YAGNI
  instead: leave a seam only when a real second case crosses the line (see the
  coding-style guide, "Extensibility and Extension Points"). The framework's line
  is crossed almost by definition; a business app's rarely is.

The concrete extension-point shapes (driver registry, seam interface,
Provider/Contributor, functional hook) and the "abstraction in `spring`, backend
in `starter`" rule are catalogued in §2–§3 above and in
[starter/DESIGN.md §2](starter/DESIGN.md).

## 6. Related Documents

- [CLAUDE.md](CLAUDE.md) — when to record a convention; output & coding rules.
- [starter/DESIGN.md](starter/DESIGN.md) — the five starter archetypes and every
  cross-cutting constraint (the deepest ruleset in the repo).
- [contrib/DIRECTORY_CONVENTIONS.md](contrib/DIRECTORY_CONVENTIONS.md) — contrib
  example layout and naming.
- [spring/DESIGN.md](spring/DESIGN.md), [log/DESIGN.md](log/DESIGN.md),
  [layout/DESIGN.en.md](layout/DESIGN.en.md) — per-module internal design.
- [layout/docs/agent-rules/common-rules.en.md](layout/docs/agent-rules/common-rules.en.md)
  — shared design/coding/testing rules for projects built on Go-Spring.
- [MANIFESTO.md](MANIFESTO.md) — the long-term "Process as Code" direction.
