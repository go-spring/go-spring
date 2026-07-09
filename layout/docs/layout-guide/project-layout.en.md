<!-- Shared top-level layout section, embedded under "## Common directory structure" in <form>-layout.md. Can be read standalone; the host doc supplies the surrounding heading. -->

The top-level skeleton below is form-agnostic. `<form>` is the current project form identifier (e.g. `domain`, `mvc`, `modulith`). The docs write `idl-<form>/` and `internal-<form>/` only to signal "these two directories are affected by the form"; **the generated directories do not carry the suffix** — they end up as `idl/` and `internal/`. The form is chosen via the form field in `gs.json`. Different forms may layer the inside of `internal/` differently, but the top-level structure stays the same.

### Top-level directories

```
.
├── conf/               # Runtime config (app.properties, etc.)
│   └── app.properties
├── docs/               # Project docs
├── idl-<form>/         # IDL definitions, split by protocol
│   ├── http/           # HTTP interface definitions
│   ├── grpc/           # gRPC proto
│   └── thrift/         # Thrift IDL
├── internal-<form>/    # Service internals; form decides the layering, see the next sections
├── logs/               # Runtime log directory (placeholder)
├── public/             # Static assets (placeholder)
├── main.go             # Program entry, only wiring and lifecycle start
├── gs.json             # Go-Spring project metadata (name, version, form, ...)
├── go.mod
├── go.sum
└── README.md
```

### Top-level responsibilities

| Directory / File | Responsibility |
|---|---|
| `conf/` | Runtime configuration decoupled from code; swapped per environment. |
| `docs/` | Project docs including skeleton and form notes; not part of the build output. |
| `idl-<form>/` | Outbound protocol contracts, split by protocol (`http` / `grpc` / `thrift`). No implementation code lives here. |
| `internal-<form>/` | Service internals; the `internal` package semantics forbid outside imports. Layering depends on the form. |
| `logs/` | Runtime log placeholder written by the logging component. |
| `public/` | Static assets (frontend build, templates) served by the HTTP layer. |
| `main.go` | Program entry — only DI wiring and lifecycle startup; carries no business logic. |
| `gs.json` | Go-Spring project metadata (name, version, form, ...). |

### Notes

- **`main.go` vs `internal/init.go`**: `main.go` only imports `internal/...` to trigger the registration chain, starts the IoC container, and drives the lifecycle. Actual route / job / consumer registration happens in each layer's `init.go` via side-effect imports. `main.go` carries no business assembly code.
- **IDL generated artifacts**: `idl/` holds only protocol contract sources (`.thrift` / `.proto` / HTTP IDL). Generated Go code (stubs, clients, models) is written back to `idl/<protocol>/gen/` and managed alongside its source. It does not land in `internal/`. Business code imports `idl/<protocol>/gen/...` to consume the generated code; hand-editing the generated files is forbidden.

### Form identifier

A project keeps a single form. `<form>` is only a label in the docs; **the generated directories do not carry the `-<form>` suffix**:

- Form candidates: `domain`, `mvc`, `modulith` (extended by the same rule).
- Docs write: `idl-<form>/`, `internal-<form>/`.
- Generated result: uniformly `idl/`, `internal/`.
- The form field only affects the inside of `internal/`; it does not change the top-level directory names.
