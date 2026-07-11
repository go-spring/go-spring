<!-- Shared top-level layout section, embedded under "## Common directory structure" in domain-layout.md. Can be read standalone; the host doc supplies the surrounding heading. -->

The top-level skeleton below applies to every Go-Spring single-service project. `idl/` and `internal/` are the fixed top-level directories; `internal/` is laid out using domain layering, described in the next sections.

### Top-level directories

```
.
├── conf/               # Runtime config (app.properties, etc.)
│   └── app.properties
├── docs/               # Project docs
├── idl/                # IDL definitions, split by protocol
│   ├── http/           # HTTP interface definitions
│   ├── grpc/           # gRPC proto
│   └── thrift/         # Thrift IDL
├── internal/           # Service internals; domain-layered, see the next sections
├── logs/               # Runtime log directory (placeholder)
├── public/             # Static assets (placeholder)
├── main.go             # Program entry, only wiring and lifecycle start
├── gs.json             # Go-Spring project metadata (name, version, ...)
├── go.mod
├── go.sum
└── README.md
```

### Top-level responsibilities

| Directory / File | Responsibility |
|---|---|
| `conf/` | Runtime configuration decoupled from code; swapped per environment. |
| `docs/` | Project docs including skeleton and layering notes; not part of the build output. |
| `idl/` | Outbound protocol contracts, split by protocol (`http` / `grpc` / `thrift`). No implementation code lives here. |
| `internal/` | Service internals; the `internal` package semantics forbid outside imports. Domain-layered. |
| `logs/` | Runtime log placeholder written by the logging component. |
| `public/` | Static assets (frontend build, templates) served by the HTTP layer. |
| `main.go` | Program entry — only DI wiring and lifecycle startup; carries no business logic. |
| `gs.json` | Go-Spring project metadata (name, version, ...). |

### Notes

- **`main.go` vs `internal/init.go`**: `main.go` only imports `internal/...` to trigger the registration chain, starts the IoC container, and drives the lifecycle. Actual route / job / consumer registration happens in each layer's `init.go` via side-effect imports. `main.go` carries no business assembly code.
- **IDL generated artifacts**: `idl/` holds only protocol contract sources (`.thrift` / `.proto` / HTTP IDL). Generated Go code (stubs, clients, models) is written back to `idl/<protocol>/gen/` and managed alongside its source. It does not land in `internal/`. Business code imports `idl/<protocol>/gen/...` to consume the generated code; hand-editing the generated files is forbidden.
