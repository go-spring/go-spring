# gs-http-gen Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`gs-http-gen` is an external `gs` tool in the tooling layer of the
four-layer stack (stdlib → spring → starter → gs). It generates HTTP
server and multi-language client code, plus OpenAPI / Swagger documents,
from a project's IDL (Interface Definition Language) files.

## 1. Responsibilities & Boundaries

- Read IDL under `idl/http/` (invoked by `gs gen` for that protocol
  subdirectory) and generate:
  - Go data models + validation
  - HTTP route binding (regular + streaming/SSE)
  - Server-side handlers scaffolding
  - Client-side stubs in Go / PHP / Java (configurable via `--language`)
  - Swagger 2.0 (`--swagger`) or OpenAPI 3.0 (`--openapi`) documents
- Only owns HTTP. Every other protocol lives in its own generator
  (gRPC / Thrift / etc.); each protocol's IDL lives in its own
  `idl/<framework>[-<protocol>]/` directory per the layout convention.

## 2. Key Abstractions & Seams

- **External-tool protocol.** Binary named `gs-http-gen`, lives next to
  `gs`. `gs-http-gen --version` prints description + version; `gs gen`
  dispatches to it (see `gs/gs/cmd/proto`) when `idl/http/` is present.
- **Subcommand-free cobra root.** All modes are flags on the root
  command: `--server`, `--client`, `--swagger`, `--openapi`, mutually
  exclusive between doc-gen and code-gen (`--swagger`/`--openapi` cannot
  combine with `--server`/`--client`).
- **`--go_package`** controls the emitted Go package name (default
  `proto`). `--output` picks the output directory (default `.`).
- **IDL surface.** The tool ships its own IDL grammar (`standard@v1.idl`
  in-tree). It supports constants, enums, structs, `oneof`, generics,
  field embedding for reuse — richer than raw JSON schemas so services
  and clients share the same source of truth.

## 3. Constraints

- Doc generation and code generation are mutually exclusive per
  invocation. Errors out early if both are requested.
- One protocol per generator: `gs-http-gen` does not handle Thrift/gRPC
  IDL. Extending it to more protocols would violate the layout rule that
  each protocol keeps an independent IDL + generator + native types.

## 4. Trade-offs & Alternatives Rejected

- **`protoc` not reused for HTTP.** HTTP semantics (path, query, header,
  streaming) don't map cleanly to `.proto`; `gs-http-gen`'s IDL is
  purpose-built so the emitted handlers stay idiomatic Go.
- **Client codegen in multiple languages, server codegen only in Go.**
  Cross-team consumers commonly need PHP/Java clients but implement
  services in Go; the surface is asymmetric on purpose.
- **Docs are a separate mode, not always-on.** Emitting `.yaml` alongside
  every `--server` build would pollute PR diffs; users opt in.
