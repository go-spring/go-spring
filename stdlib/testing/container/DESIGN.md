# container Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`container` is a testcontainers-style helper that starts a real dependency
(redis, mysql, ...) in a Docker container from inside a Go test and tears it
down when the test ends. It lives in the stdlib layer, so it must not import a
Docker SDK.

## 1. Responsibilities and Boundaries

- Start a container from a `Request`, wait for it to be ready (log substring,
  TCP port, or user-supplied wait strategy), register cleanup on the test,
  and hand back the dynamically mapped host:port.
- Provide ready-made `presets.go` values for common images (Redis, Postgres,
  MySQL, ...) so a test can spell out one line, not a dozen fields.
- Refuse to become a container SDK. The seam is the local `docker` CLI; the
  package composes commands and parses text output only.
- Refuse to hide missing Docker. `SkipIfNoDocker(tb)` is the explicit skip
  helper — a test that forgets to call it will `Fatalf` on the first `docker
  version` failure, which is a louder failure than a silent pass.

## 2. Key Abstractions and Seams

- **`TB` interface, not `*testing.T` directly.** Exposing a narrow
  `Helper/Cleanup/Fatalf/Logf/Skipf` interface keeps the stdlib package free
  of an unconditional `testing` import and lets callers fake it.
- **`docker` CLI as the seam.** All operations shell out via `os/exec`. Proxy
  variables are inherited from the environment, mirroring the repository's
  existing check.sh convention.
- **`Wait` strategies plug in per-image.** A preset supplies the strategy the
  image needs (a log line for redis, a port probe for mysql), so a test does
  not have to know the image's ready condition.

## 3. Constraints

- **Zero-dependency stdlib rule.** Only `net`, `os/exec`, `bytes`,
  `strings`, `time` and each other. No Docker SDK, no testcontainers-go.
- **Compose-v2 not required.** The docker CLI convention across this
  repository (see the tempest environment memory) is plain `docker run`, no
  `docker compose` subcommand. This helper follows suit.
- **Cleanup is best-effort.** Container removal errors are logged, not
  fatal — the test has already succeeded/failed by the time cleanup runs.
- **No parallel container reuse across tests.** Each `Run` starts and stops
  its own container. Cross-test sharing would tangle cleanup ownership and
  is intentionally out of scope.

## 4. Trade-offs and Alternatives Rejected

- **Shell out to `docker`, not import `moby/moby` / testcontainers-go.** A
  Docker SDK would pull a large transitive tree into every stdlib consumer's
  `go.sum`. Shell-out is portable and matches how the repository already
  runs Docker.
- **`SkipIfNoDocker` over autoskipping in `Run`.** An implicit skip inside
  `Run` would let a mis-configured CI silently pass every integration test.
  Requiring an explicit call makes the choice visible.
- **Curated presets in-tree over an image registry.** A Go-side registry of
  images ends up mirroring dockerhub badly; a handful of curated presets
  covers what stdlib consumers need, and everything else is a plain
  `container.Run(t, container.Request{...})`.
