# testing Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`testing` is the zero-dependency assertion library used by every Go-Spring
module for its own tests. It provides the two-mode assertion (`assert` /
`require`) split that Java's AssertJ + JUnit assumptions cover, and adds two
higher-level helpers — `container` for testcontainers-style integration tests
and `contract` for Spring Cloud Contract–style consumer-driven tests. It lives
in the stdlib layer so no runtime module depends on it in production, and the
whole package graph pulls only the Go standard library.

## 1. Responsibilities and Boundaries

- Offer a fluent, type-specific assertion API for Go tests (`That`, `Error`,
  `Number`, `String`, `Slice`, `Map`, `Panic`) as an alternative to `stretchr/
  testify` and `gomega` — no third-party dependency.
- Split fail-fast from fail-continue into two thin wrapper packages
  (`require` / `assert`) sharing one implementation in `internal`, so a test
  author picks behaviour by the import.
- Provide a testcontainers-style Docker helper (`container`) that shell-outs
  to the local `docker` CLI, keeping the stdlib layer free of Docker SDKs.
- Provide a JSON-contract driven testing pair (`contract`) with a producer
  `Verify` and a consumer `StubServer`, so consumer and producer verify the
  same file.

## 2. Key Abstractions and Seams

- **`internal.TestingT` interface** — the minimal `*testing.T` surface the
  assertion library needs. Every assertion function accepts it, so the same
  library works in a normal `*testing.T`, in a subtest, and in an outer
  harness that fakes it (used by the `testcase` shared suite itself).
- **`fatalOnFailure` bool** — the only real behaviour difference between
  `assert` and `require`. The two thin packages set it and delegate; the
  fluent API and every check live in `internal`.
- **`testcase` package = shared assertion test suite.** It is a
  `package testcase_test` collection that exercises the shared engine through
  both entry points, so `assert` and `require` cannot diverge in their
  checks. It is intentionally test-only and holds no exported code.
- **`container` shell-outs to `docker`**, not a Docker SDK — the seam is the
  local docker CLI. `presets.go` provides ready-made containers (Redis,
  Postgres, ...) as literal `Container` values.
- **`contract` has three files** — `contract.go` defines the JSON shape;
  `verify.go` drives it against a real producer; `stub.go` replays it as a
  consumer-side stub server. All three read the same JSON, so the contract
  file is the single source of truth.

## 3. Constraints

- **`stdlib/testing` and its subpackages must import only the Go standard
  library** (plus each other and `stdlib/errutil` for consistent error
  formatting inside the shared engine). Any other dependency would leak into
  every module's test binary.
- **`internal` is not exported** — assertion mechanics are shared between
  `assert` and `require`, but every callable API must go through the mode
  wrappers so the fail-fast/continue distinction stays explicit at call
  sites.
- **`container` requires a working `docker` CLI on PATH.** A missing CLI is a
  test skip (documented in `container`'s design), not a silent success.
- **`contract` matchers stay minimal.** JSON structural equality plus regex
  hints — enough to exchange with a Java Spring Cloud Contract producer;
  richer matchers stay out until a case forces them.

## 4. Trade-offs and Alternatives Rejected

- **Rebuild rather than depend on testify.** Two-mode fluent assertion is
  simple enough that owning it avoids a mandatory third-party dependency for
  every stdlib consumer. The API is intentionally close to testify for muscle
  memory, but the implementation is ours.
- **Shell out to `docker` rather than import `moby/moby` / a Go SDK.**
  Testcontainers-Go is fine for downstream projects, but the SDK would leak
  into every stdlib consumer's `go.sum`. Shell-out is portable and honest
  about what it does.
- **JSON contract file over a Go DSL.** A Go-only DSL cannot round-trip with
  a Java producer, defeating the "consumer-driven contract" premise. Plain
  JSON is the interop format.
- **Test-only `testcase` over parallel duplicated suites.** Duplicating the
  suite across `assert` and `require` invites drift; running one suite
  through both entry points forces parity.
