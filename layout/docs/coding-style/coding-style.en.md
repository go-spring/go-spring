# Go-Spring Coding Principles and Style Guide

This document defines the coding principles, idioms, and style conventions for the Go-Spring project. All contributors must follow them when modifying or adding code.

---

## 0. Simple Over Clever

- **Clarity first**: Aim for clear, intuitive, maintainable code; reject show-off tricks.
- **Avoid over-engineering**: No excessive abstraction or implicit behavior; complexity should come from the problem itself, not the implementation.
- **Reader-friendly**: Pick the most direct, understandable approach; write for the reader, not the author.
- **Rewrite rule**: If an implementation needs extra explanation to understand, rewrite it in a simpler way.

## 1. Package and Design Principles

- **Single responsibility**: Each package focuses on one functional area — small and focused, easy to test and maintain.
- **Visibility control**: Strictly separate public APIs (`PascalCase`) from internal implementation (`camelCase`); expose what is necessary, hide the internals.
- **Usable zero value**: Guarantee zero-value safety via defaults, lazy initialization, or internal fallbacks; avoid "panic when uninitialized".
- **Avoid global state**: Don't use global variables; obtain configuration, singletons, and clients through Go-Spring's IoC/DI injection.
- **Startup-time injection**: IoC/DI wiring completes at startup; don't introduce runtime dynamic injection.
- **Explicit bean conflict resolution**: Beans of the same name and type must not rely on implicit override; select explicitly via mutually exclusive conditions (`Condition`).
	- **Prefer default bean names**: When registering a bean with `gs.Provide`, do not pass an explicit name unless the bean must be resolved by that name (multi-instance config matching, disambiguating same-type beans, etc.). In all other cases, let the container assign the default name.
- **Starter first**: Before wiring an external component (Redis / MySQL / Kafka, etc.), check whether `starter/` already provides one; reuse it instead of writing initialization from scratch.

### 1.1 Extensibility and Extension Points

Extensibility is a judgment call, not a reflex. It lives in tension with Section 0 (Simple Over Clever) and Section 7 (YAGNI): a speculative extension point is over-engineering, while a missing one forces a rewrite later. Resolve the tension with a threshold, not a slogan.

- **Leave a seam only when it crosses a line — otherwise don't**: One-off internal logic gets no extension point. Add one only when **any** of these holds: (a) it's part of an outward contract that other modules or downstream projects depend on; (b) a second implementation is already foreseeable (a second backend / driver / strategy), not merely imaginable; (c) users are expected to replace the default behavior. Absent these, write the direct implementation and refactor when a real second case arrives.
- **When you do open a seam, keep it single and narrow**: Open the one widest seam for a concern (e.g. `RoundTripper` for HTTP) rather than cutting extension points at several layers, and expose only the minimal method set needed to swap the implementation. Narrower is easier to implement and harder to misuse.
- **Framework code plays by stricter rules**: The above is the *application-level* stance. Go-Spring itself (`stdlib/` / `spring/` / `starter/`) is a framework that must serve every scenario, so there extension points are *mandatory* and built-ins must ride the very seams they expose. That duty, and the catalog of accepted extension-point shapes (driver registry, seam interface, Provider/Contributor, functional hook), live in the framework's `ARCHITECTURE.md` §5 and `starter/DESIGN.md` §2 — reuse those shapes rather than inventing a new mechanism.

### 1.2 Abstraction Ownership and Configuration Seams

- **Abstractions belong to the consumer, not the implementer**: Define an interface on the side that uses it (or in a neutral `stdlib` package), never in the implementation package; implementations depend on the abstraction, not the reverse.
- **Program to interfaces at API boundaries**: Public API parameters and fields take interfaces or function types where substitution matters, so callers can inject stand-ins and tests can inject fakes.
- **Configuration is an extension point**: Multiple implementations of one capability share a single config prefix (`spring.kafka`, `spring.lock`, …); switching implementation changes only the `import`, never the config keys or business code.
- **Fail fast at the seam**: Selecting an unknown driver / implementation must error at startup, never silently fall back to a default.

### 1.3 API Evolution and Backward Compatibility

- **Internal code: change it outright**: For non-exported code and code not yet consumed outside its module, just change it. Don't keep renamed `_vars`, re-exports, or `// removed` comments as compatibility shims (mirrors the root `CLAUDE.md` rule).
- **Outward contracts: change with care**: An exported API that downstream projects depend on is a contract. Prefer adding over breaking; evaluate blast radius before a breaking change.
- **Deprecate, don't ambush**: When a stable outward API genuinely must go, mark it with a `// Deprecated:` comment pointing to the replacement and allow a transition window before removal. This deprecation flow applies only to published stable APIs — it is the deliberate exception to "change it outright" above, not a contradiction of it.

### 1.4 The Shape of a Cross-Cutting Concern

Choose the shape by asking whether the concern needs to *do something after* the call, or *own the call's lifetime*:

- **Before-only, may short-circuit** → a plain function, called on the first line of the method (e.g. `security.Require(ctx, "orders:write") error`).
- **Anything after the call** (commit/rollback, recover, timing, restoring state, rewriting the response) — including `@AfterReturning` / `@AfterThrowing` concerns like "emit on success" / "report on failure" → a wrapping decorator `func(cfg) func(ctx, proceed) error`, or a middleware chain.

Decide by "is there work after the call", not by "Java writes it as AOP". A wrapper is ordinary function nesting — it does **not** justify a shared interceptor-chain protocol (see `ARCHITECTURE.md` §6.3). The cost is real either way: the functional form is opt-in, so forgetting it at the top of a method silently loses the guard; the wrapping form is enforced by construction. A "must never be missed" concern should consider the wrapping form even if it is purely a precondition (`Require` is a deliberate acceptance of the functional form's cost).

### 1.5 Audit Criteria

Criteria for reviewing whether an existing abstraction / API / seam should exist. These resolve the ambiguities the rules above leave open.

- **"An extension point needs two real consumers" applies only to protocols / interfaces / seams.** Leaf APIs aimed at application code (composition helpers, sentinel errors, convenience functions) cannot be judged by an in-repo grep — their target consumers are, by definition, outside the repository (2026-08-26).
- **Documented boundaries are exempt — read the docs first.** A deletion of an API that any `DESIGN.md` / `README` promised is a broken contract, even with zero in-repo consumers; a verifier must read the docs before judging.
- **Deleting beats privatizing.** A zero-consumer exported surface is deleted outright; privatizing is the fallback that silently changes behavior.
- **Merging parallel structures: check the semantic payload first.** A unified signature must not drop the semantics a closure carried.
- **The grep range must include `examples/`.**
- **"mirrors / keep in sync" cross-reference comments are a greppable drift signal** — mark them as a shared contract.
- **"An exception to the unified contract" first asks whether it carries tech-stack-specific semantics** (e.g. neo4j encryption determined by the URI scheme is a deliberate fork, not a dedup miss).
- **A claim that a wrapper overrides a user callback must first verify whether the inner escapes to the user surface** (construction-time wrapping + private fields = unreachable).
- **A docs sweep and its code migration must ship in the same PR.**
- **Stop condition: one consecutive round with no surviving proposal is convergence.**

## 2. Naming Conventions

- **Package names**: All lowercase, short and descriptive (`errutil`, `assert`, `gs`); no underscores or camelCase, and don't repeat the package's contents.
- **Identifiers**: Follow Go conventions — `PascalCase` for public, `camelCase` for internal.
- **Constants**: Use `UPPER_SNAKE_CASE`.
- **Error variables**: Predefined errors start with the `Err` prefix (`ErrNotFound`).
- **Interfaces**: Single-method interfaces end with the `-er` suffix (`Handler`, `Provider`); use descriptive nouns for large interfaces.
- **Variables**: Concise yet meaningful; avoid unnecessarily long names.
- **Method receivers**: Short and consistent (1–2 letters); don't use `me`/`this`/`that`.
- **Boolean method names**: Prefer dropping the `Is` prefix — use the state adjective directly (`Enabled()`, `Healthy()`). Keep `Is` only when the remaining word cannot stand alone as a yes/no question without ambiguity (`IsLeader()` — `Leader()` reads like it returns who the leader is; `IsZero()`). Criterion: delete `Is` and see whether the name still works.
- **File names**: `function name + the instance qualifier the directory already expresses` → drop the qualifier. Criterion: keep the qualifier only when dropping it leaves nothing meaningful (`gorm.go` in `starter-gorm/`, `pprof.go` in `starter-pprof/`, `server.go` in `starter-oauth2-server/`) — that is the "package's main file" convention. Go gives special meaning only to `_test` / `_GOOS` / `_GOARCH`. Do **not** rename: `example-*/example.go` (`example` marks "runnable sample", not an instance qualifier), generated code (`pb/service.pb.go`, kitex `k-echo.go`, `service.triple.go`), or Go-idiomatic `cloud/<x>/<x>.go`. Renaming also updates the references in USAGE / DESIGN / README.

## 3. Code Formatting and Organization

- **Standard Go formatting**: Strictly follow `gofmt`.
- **Import grouping**: `standard library` → `external dependencies` → `internal dependencies`, separated by blank lines.
- **Function length**: Prefer small functions; a function should do one thing well.
- **Line length**: Reasonably compact; avoid overly long single lines, but don't rigidly cap line count.
- **Blank lines**: Use blank lines to separate logical blocks; avoid dense stacking.
- **Code cleanup**: Delete obsolete code outright; don't keep commented-out dead code.

### 3.1 Passing Arguments

When a compound-expression argument meets **any** of the following, extract it into a semantically named variable before passing it in:

- **Multiple nesting levels** (≥2 levels — splitting makes each step's intent clear).
- **Reused** (≥2 times — deduplicate along the way).
- **Naming adds information** (the expression alone doesn't reveal the business meaning).

Single-level calls used only once with a self-explanatory function name may stay inline; forced extraction only adds noise (echoing "reader-friendly" in Section 0). For example, keep `fmt.Sprintf(..., strings.ReplaceAll(name, "/", "_"))` inline; split `path.Dir(filepath.ToSlash(strings.TrimPrefix(file, "./")))`.

### 3.2 In-File Code Organization

Goal: readable top to bottom in one pass, minimizing jumps.

Must follow (raised in review):

- **A type forms one section**: A type + its methods + private helpers that serve only it are placed contiguously, with no unrelated functions in between.
- **Place helper definitions nearby**: Small functions used only in one place, and dedicated constants/variables, sit next to their user rather than floating to the top of the file.
- **On ownership conflicts, favor the type**: When a helper is used both by a type's methods and by other functions, put it in that type's section.
- **Extract a whole section into a same-named new file when it meets any of**: independently testable, reused in multiple places, or noticeably exceeding one screen.

Preferences (exceptions allowed):

- **Roughly top-down**: Entry points / main flow first, details later.
- When conflicting with Go idioms (bottom-up, consolidated const blocks), prefer project consistency.

This rule can't be checked automatically; it's a writing and review orientation, not a hard gate.

## 4. Error-Handling Philosophy

The project uniformly uses the **dual-semantic error-wrapping pattern** of `stdlib/errutil`:

> **Project rule**: Don't construct errors directly with `errors.New`/`fmt.Errorf`; always wrap through `errutil`.

- **Explanatory wrapping** (`errutil.Explain`) — adds business semantics: `errutil.Explain(err, "failed to connect to database")`. Wrapping an existing error keeps the message byte-for-byte (`fmt.Errorf("%s: %w", msg, err)`); constructing in place uses `errutil.Explain(nil, "component: detail")` (nil degrades to `fmt.Errorf`).
- **Sentinel exception**: `var ErrX = errors.New(...)` keeps `errors.New` (there is no sentinel constructor in `errutil`). To add detail onto an existing sentinel, use `errutil.Explain(Sentinel, "detail %q", x)` — the message order changes (`errors.Is` is unaffected), so grep for tests that lock the order before changing.
- **Do not use `errutil.Stack`** — its `>>` is call-path semantics and appears only in layout templates, `log/plugin.go`, and the IDL parser. Do not use "consistent with other examples" as an excuse to fall back to `fmt.Errorf`; when you meet non-compliant old code, the direction is to convert it to `errutil`.
- **Fail fast, return early**: Return business errors as early as possible; for unrecoverable programming errors during initialization, panic directly.
- **The panic boundary for constructors and validation**: Constructors (`New*`, returning a runtime component with a lifecycle) always return an `error`, never panic. Option/DSL value builders (`WithX`, `FixedRate` — functions returning a configuration value, always used inside an inline expression, with no error channel) and init-time registration (`Register*`) may panic to fail fast. The criterion: is the returned thing a runtime component, or a wiring-time configuration value?
- **Preserve the unwrap chain**: `errutil` internally guarantees `%w` semantics, fully supporting `errors.Is()`/`errors.As()`.
- **Sufficient context**: Error messages should carry enough context to locate the source.
- **Panic recovery reports through `stdlib/goutil`**: `OnPanic func(ctx, PanicInfo{Panic, Stack})` is a single-slot function pointer (direct assignment replaces it, last writer wins) — do **not** build a chained `RegisterOnPanic`. `goutil.ReportPanic(ctx, r)` is the public reporting entry for an already-recovered panic, and `goutil.SafeRun(ctx, f)` routes a panic through the normal error path. New recover points always use `ReportPanic` / `SafeRun`; `stdlib` must never import `log` (the bridge is `log` assigning `goutil.OnPanic` in its own init).

## 5. Log and Metric Leveling

### 5.1 Log Leveling

- **Every key action node gets a log line**: state transitions, lifecycle events, and cross-boundary actions all leave a trace — the difference is only the level. The rule is not "too frequent to log" but "frequent means a quieter level".
- **Arrange by level**: successful high-frequency access records at `Debug` (off by default, opened when investigating); rare normal flow (startup, elected, config refresh) at `Info`; abnormal but self-healing/degraded at `Warn`; failures needing human intervention at `Error`. The level carries the consequence; the fields carry the dimensions.
- **Structured first**: keys share names and dimensions with the metrics/spans; messages go in `log.Msg`. Pure-dimension access records may omit the message text, but failure event lines carry both `log.Err` and `log.Msg`.

### 5.2 Which Nodes Must Report Metrics

One-line criterion: **logs answer "what happened"; metrics answer "how much / how long / how is it right now"** — a node that needs trending, rate calculation, or alerting must have a metric. Three shapes:

- **Operation nodes** (per-request/per-message cross-boundary actions): `<family>.operation.total` + `<family>.operation.duration` with an exclusive status axis (summed over status = the operation count, no double counting) and explicit duration buckets.
- **Event nodes** (rare but semantically major state transitions): a dedicated counter named after the event itself (`lock.lost.total`, `resilience.client.breaker.state_change`) — not the total/duration template; an event is not an operation.
- **State nodes** (where "how is it right now" matters): a gauge (`messaging.operation.active`, `config.refresh.last_success_timestamp`, breaker state).

Dimension discipline: unbounded cardinality (keys, addresses, destination values) never enters a metric — spans and logs only. The status axis carries the outcome. Pure low-frequency config-change events (e.g. governance policy applied) are fine with a log line alone.

## 6. Documentation and Comments

- **Package docs**: Every public package must have a package comment explaining "what", "why", and use cases, without dwelling on implementation details.
- **Function docs**: Every exported function must have a comment — description, parameters and return values, error conditions (if any); complex cases may include examples.
- **Self-documenting code**: Use clear naming and simple structure so the code explains itself; don't add unnecessary comments.
- **AI-collaboration comments**: When you need to constrain AI behavior, add special comments, e.g. `// AI: do NOT refactor this function`.
- **Comments describe the current contract only** — not history, positioning, or analogy. Delete clauses like "historically it was", "same shape as package X", "following the X pattern"; keep the behavior contract itself (2026-08-30).
- **No per-method comments on interface implementations** (e.g. `propagate.Carrier`, otel `TextMapCarrier`): the contract is written at the interface, so repeating it at the implementation is describing implementation. A type-level doc saying "which adapter this serves" is enough (2026-09-29).
- **File comments are separated from the `package` clause by a blank line.** A comment explaining "this file" (first line shaped like `<file>.go is/xxx`, `This file ...`) must be split from `package` by a blank line, or the Go toolchain treats it as the package doc comment. Only `// Package X ...` / `// Command X ...` sit directly above `package`. Both forms compile; this is the doc-attribution rule (2026-10-05).
- **Every exported semantic struct field gets its own doc comment**; the type's doc comment keeps only a one-line overall positioning (2026-09-03).

### 6.1 README Files and Structure

- **The Chinese README is `README_CN.md` repo-wide** (`README.zh.md` / `README_zh.md` are gone). The one exception is `layout/`, a self-contained sample project whose docs all pair as `*.en.md` + `*.zh.md` — do not "unify" it (2026-10-02).
- **Chinese README section titles are fully localized**: `## 使用方式`, `### API 列表` (no variants like "API 总览"; the English side is uniformly `### API`), `## 关键设计`, `## 许可证`; inline terms keep English (Apache License 2.0, API, identifiers); the license line is uniformly "Apache License 2.0，详见 [LICENSE](../../LICENSE)。". The English README keeps `## Usage` / `## License` (2026-08-23).
- **The EN and CN READMEs mirror section-for-section**, not just in punctuation.
- **Chinese body punctuation is full-width** (`，` `：` `（）` `。` `、` `——`); code blocks, inline code, links, and English terms are unchanged. Grep for `[一-鿿][,:;()]` must return zero. "The file was originally half-width" is to-be-fixed, not to-be-kept (2026-09-07).
- **A README carries no layering/marketing modifiers** ("belongs to the zero-dependency `stdlib` layer" / "Part of … stdlib layer") — only function / behavior / boundary. Genuine design reasons ("keeps this library zero-dependency") may stay; DESIGN docs may keep layering prose (2026-08-19).
- **In `cloud/**/README*.md`, reference identifiers with backticks** (`` `WrapClientExecutor` ``), not godoc-style `[WrapClientExecutor]` (in Markdown `[X]` is a literal bracket); the only exceptions are Markdown links `[label](url)` and anchors `](#...)`. Go source comments still use `[X]` (2026-10-01).
- **A stdlib package writes one merged README** (`README.md` EN + `README_CN.md`), in a fixed order: language switch line → positioning & scenarios (with a "what it is not" boundary) → Usage → Design → License; no `DESIGN` (2026-08-16). Other modules (`log` / `spring` / `cloud` / `starter` / `contrib`) keep the README + DESIGN four-file set — do not unify them.
- **Docs must not retain stale API signatures or references to non-existent fields / deleted types**; after an API change, sweep every `.md` repo-wide (`temp/` is the user's writing area — leave it).

## 7. Testing Style

- **Utility libraries first**: Prefer `stdlib/testing`'s `assert` (continue on failure) / `require` (abort on failure) assertions; don't pull in third-party libraries like testify. Default to `assert`, and use `require` only when a failure would cause subsequent code to panic or become meaningless (e.g. a nil check before dereferencing). Use the standard library `testing` directly only when you want to avoid extra dependencies.
- **Subtest grouping**: Use `t.Run()` to logically group different scenarios.
- **Table-driven tests**: For multi-input/output scenarios, the table-driven pattern is recommended; table-driven and subtests can be mixed — use whichever is more concise.
- **Boundaries of raw assertions**: Use `t.Error`/`t.Fatal` only where there's no corresponding assertion — e.g. timeout protection, `select` branches, or unrecoverable initialization failures.
- **Tests alongside production**: Test files live in the same package directory as production code; tests are living documentation.
- **Every `X_test.go` must have a matching `X.go`** — no orphan test files without a main file. Shared test infrastructure (TestMain / OTel wiring / metric-assertion helpers) goes either in the test paired with the package's main file (e.g. `starter_test.go`) or in the test of its primary consumer — never in a standalone `observe_test.go` with no main file (2026-10-02).
- **Contract tests** (Spring Cloud Contract-style CDC: provider-side `Verify` + consumer-side `StubServer`) currently live in `cloud/experimental/contract/` (`cloud/contract/` does not exist yet — that is the target once a real consumer appears); a new protocol contract engine joins it as a sibling package. The HTTP engine stays zero-third-party-dependency (a deliberate choice, not a layer rule); do not propose moving it back to `stdlib` or to `contrib` (2026-08-16, 2026-09-10).

## 8. Go Idioms

- **Authentic Go**: Adapt Spring concepts without forcing object orientation; keep Go's natural idioms.
- **Use context correctly**: Request flows must carry `context.Context`; avoid casually using `context.TODO()`/`context.Background()`.
- **Boundary checks**: Validate input at API boundaries to catch errors early.
- **Avoid over-abstraction**: Abstract only when truly needed; follow YAGNI and don't pre-plan for an uncertain future.
- **Minimal dependencies**: Add only necessary external dependencies.
- **Subprocess IO**: When invoking external commands, wire stdout/stderr straight to `os.Stdout/Stderr` to keep output streaming; buffer only when you need to parse the output.
- **Generics**: When a generic helper's type parameter cannot be cleanly inferred, keep the parameter as `T` and convert explicitly at the call site — do not use `any` + a runtime `.(T)` assertion. Note that when `T` is inferred as an interface, passing a concrete value fails to compile (Go never implicitly satisfies an interface for a type argument); convert at the call site (`greet.GreetServiceHandler(&GreetProvider{})`) so the argument has the interface type — compile-time safe, no panic path. `any` + runtime assertion is reserved for cases where the call site genuinely cannot express the conversion. A generic guard function must be a free function (Go methods may not have type parameters).

## 9. Concurrency-Safe Design

- **Not concurrency-safe by default**: Unless explicitly designed for it, concurrency safety is not guaranteed; the caller decides whether to synchronize.
- **Declare explicitly**: If a type supports concurrent access, the documentation must say so.
- **Use synchronization primitives correctly**: `sync/atomic` for atomic flags, `sync.Mutex` to protect complex state, `channel` for communication.
- **Encapsulate concurrency details**: Hide complex synchronization logic behind a clean API.

---

**Overall principle**: Clean, consistent craftsmanship. While preserving Go's natural idioms and performance, balance the familiar Spring programming model, and focus on developer experience through intuitive APIs and thorough documentation.
