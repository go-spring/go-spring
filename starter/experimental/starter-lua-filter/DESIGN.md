# starter-lua-filter Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-lua-filter` is a **Contributor**-archetype starter (see
[starter/DESIGN.md](../DESIGN.md) §2.3) that lets an application drop Lua
scripts into its HTTP pipeline as programmable filters — the equivalent of
what Kong / APISIX / Envoy / OpenResty do at the gateway data plane.

## 1. Positioning — programmable HTTP filter, not a scripting bean

Go is compiled and has no JVM culture of "refreshable scripting beans" (the
Groovy-bean niche in Spring). Where Go microservices actually reach for Lua
is the gateway data plane — raw request/response manipulation without a
redeploy. In-process dynamic logic is better served by Go middleware, CEL,
or WASM.

So the starter's scope is deliberately **HTTP-handler-level programmable
filters**, not a general-purpose scripting engine. Timeouts / memory limits,
container-bean injection, and hot-reload are shaped by that positioning —
they extend a gateway filter, not a scripting bean.

## 2. Seam — `*gs.HttpServeMux`

Mounting rides on `spring/gs/http.go`'s `*gs.HttpServeMux`: the framework
supplies a default mux only under `OnMissingBean[*HttpServeMux]`, so the
application registers a wrapped mux that inserts filters. The mux is
framework-neutral — gin/echo/hertz all collapse to `http.Handler` at some
point — so no per-framework adapter is needed. `spring/web` is an empty
placeholder today, so this is the natural home for a filter seam.

## 3. Implementation choices

- **gopher-lua**, pure Go, no CGO — matches the starter family's
  cross-compilation-friendly stance.
- **Precompile once, pool VMs.** `parse.Parse` + `lua.Compile` produces a
  proto at construction time. Every request borrows an `LState` from a
  `sync.Pool`, calls `install` to rebind host API (`req` / `resp` / `deny`
  / `log`), runs `PCall`, then `SetTop(0)` returns it. No background
  goroutine — the destroy hook is `nil`.
- **Sandbox.** `SkipOpenLibs` opens only base / table / string / math and
  overwrites `dofile` / `loadfile` / `load` / `loadstring` with nil, so a
  filter cannot reach the filesystem or `eval` arbitrary code.
- **Multi-instance via `gs.Group("${spring.lua.filter}", ...)`.** Each
  configured filter becomes a named bean; the mux picks one by name with
  `gs.TagArg("<name>")`. Adding a filter is a config-only change.

## 4. Constraints

- Scripts must respect the fixed host-API surface (`req`, `resp`, `deny`,
  `log`). Anything richer is intentional — expanding the surface expands
  the sandbox.
- The starter does not schedule background goroutines, so it registers no
  destroy hook. Adding one later (e.g. for hot-reload watchers) means
  adding a Close path.

## 5. Trade-offs / Alternatives Rejected

- **General scripting bean — rejected.** Groovy-style refreshable beans do
  not fit compiled Go; the increment over Go middleware / CEL / WASM only
  shows up at the gateway edge.
- **Per-framework middleware adapters — rejected for now.** Every Go web
  framework already exposes `http.Handler` interop; the mux seam already
  covers them uniformly. A future extension can add framework-native
  middleware if a real need lands.
- **Hot-reload not implemented in v1.** `spring/gs_dync`'s Refresh is
  ready to plug in when scripted rules need to change without a restart;
  the seam is compatible.
