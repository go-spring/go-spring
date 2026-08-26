# contract Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`contract` is the Go-Spring equivalent of Spring Cloud Contract: one
declarative artifact (a request paired with the response it must produce)
drives both the producer's verification and the consumer's stub. It lives in
the cloud module rather than stdlib/testing so that future protocol engines
needing protocol SDKs (gRPC, Thrift) can grow as sibling packages under this
directory; the HTTP engine itself still depends only on `net/http` and
`encoding/json`.

## 1. Responsibilities and Boundaries

- Define one `Contract` shape (`Name`, `Request`, `Response`) that both ends
  read.
- Provide `VerifyHandler(t, handler, contracts)` — the producer test that
  replays every contract's request against a real `http.Handler` and asserts
  the actual response matches — plus `Verify(t, baseURL, contracts)` for a
  live server reached over a socket.
- Provide `StubServer(contracts)` — the consumer test double that answers
  incoming HTTP calls using the contracts' promised responses.
- Refuse to invent a Go-only DSL. Contracts on disk are JSON so a Java
  producer can round-trip; a caller who prefers YAML unmarshals it and passes
  `[]Contract`.
- Refuse rich matcher grammar. Requests match by present fields only (empty
  map / nil body = no constraint), plus a `Body` JSON structural equality;
  responses match by status, headers and body structural equality.

## 2. Key Abstractions and Seams

- **`Contract` = one JSON file.** Both `Verify` and `StubServer` consume the
  same struct, so a consumer stub can never encode a response the producer
  does not actually return.
- **Present-only matching for requests.** An empty `Query` / `Headers` map
  imposes no constraint; a nil `Body` skips body inspection. This lets a
  contract pin only the parts that matter.
- **JSON structural equality for bodies** — the same helper the assertion
  library uses. Numeric precision and key order are not part of the match.
- **`StubServer` returns a plain `*httptest.Server`.** The consumer test dials
  it like any HTTP server; no `contract`-specific client is needed.
- **`LoadFS(fsys, glob)` helper** loads contract files from an `fs.FS`, so
  tests can embed contracts via `go:embed` and both consumer and producer
  read from the same embedded set.

## 3. Constraints

- **Dependency-free by choice, not by layer rule.** No YAML parser, no
  gomock, no gomega — the HTTP engine needs nothing beyond `net/http` and
  `encoding/json`.
- **Contract file is the single source of truth.** Both sides read the same
  JSON; a consumer that hand-writes a stub response inline defeats the point.
- **The stub answers deterministically.** Multiple contracts with the same
  request shape resolve to the first match. Contract authors keep the request
  shapes disjoint.
- **Only HTTP today.** gRPC / Thrift engines, when they arrive, are sibling
  packages under this directory. Message-broker contracts are not modelled
  here; a broker test uses its broker's own in-memory test driver directly.
- **Sibling protocol packages do not share types with this one.** A grpc or
  thrift engine under this directory keeps its own Contract/envelope types and
  SDK imports; what is shared is the directory and the present-only matching
  semantic, not code.

## 4. Trade-offs and Alternatives Rejected

- **JSON on disk over a Go DSL.** A Go-only DSL cannot round-trip with a Java
  Spring Cloud Contract producer, defeating the interop premise.
- **JSON, not YAML.** YAML would require a parser dependency (or a
  hand-rolled one that will inevitably lag the spec); consumers who prefer
  YAML unmarshal it themselves and pass in `[]Contract`. The package stays
  dependency-free even though the cloud module could afford the parser.
- **Structural equality over rich matchers.** Real contracts either match a
  fixed body or they should not be pinning the body at all.
- **`VerifyHandler` uses `http.Handler`, not a live server.** Running the
  tests in process is fast and keeps the producer's mounted middleware in
  play; `Verify` covers the live-server case against a real base URL, sharing
  one assertion loop.
