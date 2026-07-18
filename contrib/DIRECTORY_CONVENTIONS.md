# contrib Directory Conventions

This document describes the directory and file conventions that every example
project under `contrib/` follows. These are **runnable examples**, not reusable
starter modules — each one demonstrates how a third-party framework is wired the
Go-Spring way.

## 1. Grouping

`contrib/` is grouped by framework/theme, then by variant:

```
contrib/
├── <framework>/            # e.g. kitex, goframe, go-kratos, go-zero, dubbo-go
│   └── <variant>/          # a runnable project (see §2)
├── registry/               # grouped by registry backend
│   └── <backend>/          # consul, etcd, nacos, polaris, zookeeper
└── observability/          # single project (+ observability-gorm) with backend stacks
```

The **variant** axis differs per framework and is one of:

- **protocol** — `http`, `grpc`, `ws`, `tcp`, `triple`, `dubbo`, `jsonrpc`, `rest`, `trpc`
- **IDL/codec** — `thrift`, `protobuf`, `generic`
- **role-named** — go-zero uses `greet-api` / `greet-rpc` / `greet-ws`

Rule: each variant that cannot coexist in one module (different IDL system,
codegen toolchain, or native types) becomes its own subdirectory with its own
`go.mod`.

## 2. Project layout (a single runnable variant)

```
contrib/<framework>/<variant>/
├── README.md               # English (canonical)
├── README_CN.md            # Chinese translation
├── .gitignore              # optional
├── go.mod                  # every project owns its own module
├── go.sum
├── idl/                    # IDL + generated code + codegen script
│   ├── <name>.proto        # or .thrift / .api  — the IDL source
│   ├── <name>.pb.go        # generated (DO NOT EDIT)
│   ├── <name>.<x>.go       # framework stubs, generated (DO NOT EDIT)
│   └── gen-code.sh         # regenerates idl/*.go from the IDL
├── provider/               # the long-lived server
│   ├── handler.go          # business handler + ServiceRegister bean
│   ├── main.go             # gs.Run()
│   └── conf/app.properties # provider config (server role)
├── consumer/               # one-shot client: calls provider, asserts, exits
│   ├── main.go             # client bean + gs.Run()
│   └── conf/app.properties # consumer config (client role)
├── scripts/
│   └── smoke-test.sh       # brings up backends+provider, runs consumer, tears down
├── docker/                 # optional: config for backend stack (prometheus, promtail, ...)
└── docker-compose.yml      # optional: local backends (registry, observability)
```

### Standard directories

| Dir         | Purpose                                                              |
|-------------|---------------------------------------------------------------------|
| `idl/`      | IDL source, generated code, and `gen-code.sh`                       |
| `provider/` | long-lived server; `main.go` + `handler.go` + `conf/app.properties` |
| `consumer/` | one-shot client that calls the provider, asserts, then exits        |
| `scripts/`  | `smoke-test.sh` (and sometimes `gen-code.sh`)                       |
| `docker/`   | backend stack config files (optional)                              |

### Standard files

- `README.md` + `README_CN.md` — every project ships both; English is canonical.
- `provider/main.go` & `consumer/main.go` — entrypoints, both driven by `gs.Run()`.
- `provider/conf/app.properties` & `consumer/conf/app.properties` — config lives
  in properties, not hard-coded in `main()`. Server vs client role is set here.
- `scripts/smoke-test.sh` — the standard smoke test name across all projects.
- `idl/gen-code.sh` — regenerates code from the IDL. Generated files are checked
  in and marked **DO NOT EDIT**.

## 3. Naming conventions

- **Module path**: `go-spring.org/<framework>/<variant>` (e.g.
  `go-spring.org/kitex/thrift`, `go-spring.org/registry/consul`). A few flat
  variants use `go-spring.org/<framework>-<variant>` (e.g.
  `go-spring.org/go-kratos-http`). go-zero uses bare names (`greetapi`,
  `greetrpc`, `greetws`).
- **Smoke test**: always `scripts/smoke-test.sh`.
- **Codegen script**: `gen-code.sh`, normally under `idl/` (a few projects put it
  under `scripts/`).

## 4. Deviations by design

- **observability/** — one triple app × multiple backend stacks under
  `stacks/<n>-<name>/` (classic / collector / lgtm / elastic). Stack numbering
  intentionally skips values (no 4/6).
- **go-zero/** — uses role-named variants (`greet-api`/`greet-rpc`/`greet-ws`)
  and generates extra provider files (`logic.go`, `routes.go`,
  `servicecontext.go`, `greethandler.go`) per go-zero's own scaffolding.
- **trpc-go/** — extracted into a reusable module: see `starter/starter-trpc`.
- **registry/** — grouped by backend rather than protocol; some use kitex IDL
  (`idl/echo/...`) with hand-wired `provider/server.go`.

## 5. What NOT to add

contrib examples exist only for **smoke testing + integration demonstration**.
Do not add deployment scaffolding (`build.sh`, `bootstrap.sh`, extra `script/`
dirs). Keep it to `smoke-test.sh` / `gen-code.sh` and source.
