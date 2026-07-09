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

## 2. Naming Conventions

- **Package names**: All lowercase, short and descriptive (`errutil`, `assert`, `gs`); no underscores or camelCase, and don't repeat the package's contents.
- **Identifiers**: Follow Go conventions — `PascalCase` for public, `camelCase` for internal.
- **Constants**: Use `UPPER_SNAKE_CASE`.
- **Error variables**: Predefined errors start with the `Err` prefix (`ErrNotFound`).
- **Interfaces**: Single-method interfaces end with the `-er` suffix (`Handler`, `Provider`); use descriptive nouns for large interfaces.
- **Variables**: Concise yet meaningful; avoid unnecessarily long names.
- **Method receivers**: Short and consistent (1–2 letters); don't use `me`/`this`/`that`.

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

- **Explanatory wrapping** (`errutil.Explain`) — adds business semantics: `errutil.Explain(err, "failed to connect to database")`.
- **Path wrapping** (`errutil.Stack`) — tracks the call chain: `errutil.Stack(err, "InitService")`.
- **Fail fast, return early**: Return business errors as early as possible; for unrecoverable programming errors during initialization, panic directly.
- **Preserve the unwrap chain**: `errutil` internally guarantees `%w` semantics, fully supporting `errors.Is()`/`errors.As()`.
- **Sufficient context**: Error messages should carry enough context to locate the source.

## 5. Documentation and Comments

- **Package docs**: Every public package must have a package comment explaining "what", "why", and use cases, without dwelling on implementation details.
- **Function docs**: Every exported function must have a comment — description, parameters and return values, error conditions (if any); complex cases may include examples.
- **Self-documenting code**: Use clear naming and simple structure so the code explains itself; don't add unnecessary comments.
- **AI-collaboration comments**: When you need to constrain AI behavior, add special comments, e.g. `// AI: do NOT refactor this function`.

## 6. Testing Style

- **Utility libraries first**: Prefer `stdlib/testing`'s `assert` (continue on failure) / `require` (abort on failure) assertions; don't pull in third-party libraries like testify. Default to `assert`, and use `require` only when a failure would cause subsequent code to panic or become meaningless (e.g. a nil check before dereferencing). Use the standard library `testing` directly only when you want to avoid extra dependencies.
- **Subtest grouping**: Use `t.Run()` to logically group different scenarios.
- **Table-driven tests**: For multi-input/output scenarios, the table-driven pattern is recommended; table-driven and subtests can be mixed — use whichever is more concise.
- **Boundaries of raw assertions**: Use `t.Error`/`t.Fatal` only where there's no corresponding assertion — e.g. timeout protection, `select` branches, or unrecoverable initialization failures.
- **Tests alongside production**: Test files live in the same package directory as production code; tests are living documentation.

## 7. Go Idioms

- **Authentic Go**: Adapt Spring concepts without forcing object orientation; keep Go's natural idioms.
- **Use context correctly**: Request flows must carry `context.Context`; avoid casually using `context.TODO()`/`context.Background()`.
- **Boundary checks**: Validate input at API boundaries to catch errors early.
- **Avoid over-abstraction**: Abstract only when truly needed; follow YAGNI and don't pre-plan for an uncertain future.
- **Minimal dependencies**: Add only necessary external dependencies.

## 8. Concurrency-Safe Design

- **Not concurrency-safe by default**: Unless explicitly designed for it, concurrency safety is not guaranteed; the caller decides whether to synchronize.
- **Declare explicitly**: If a type supports concurrent access, the documentation must say so.
- **Use synchronization primitives correctly**: `sync/atomic` for atomic flags, `sync.Mutex` to protect complex state, `channel` for communication.
- **Encapsulate concurrency details**: Hide complex synchronization logic behind a clean API.

---

**Overall principle**: Clean, consistent craftsmanship. While preserving Go's natural idioms and performance, balance the familiar Spring programming model, and focus on developer experience through intuitive APIs and thorough documentation.
