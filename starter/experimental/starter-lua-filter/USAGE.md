# starter-lua-filter Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). Every behavior claim is verified against
the starter source (`starter.go`, `filter.go`, `config.go`) and the runnable self-asserting
[example/](example/) (`example/check.sh`, zero external dependencies). Lua language semantics and
the gopher-lua runtime are [gopher-lua's documentation](https://github.com/yuin/gopher-lua) — this
document covers the go-spring wiring, the host API, and the sandbox.

**Activation**: any `spring.lua.filter.instances.*` key — `gs.Group("${spring.lua.filter}")` creates one
`*Filter` bean per `spring.lua.filter.instances.<name>` entry, named `<name>` (starter.go:39). No
`enabled` switch; no key, no bean.

---

## 1. Complete worked project

A service whose `/hello` and `/admin` routes sit behind a Lua guard: every response is tagged,
`/admin` requires a token, and the rules hot-reload at runtime. File tree (this is the shape
`example/` runs):

```
demo/
├── go.mod
├── main.go
├── conf/
│   └── app.properties
└── scripts/
    └── guard.lua
```

**go.mod**:

```
require (
    go-spring.org/spring            v1.3.x
    go-spring.org/starter-lua-filter latest
)
```

**main.go**:

```go
package main

import (
    "net/http"

    "go-spring.org/spring/gs"
    StarterLuaFilter "go-spring.org/starter-lua-filter"
)

func main() {
    // Provide a *gs.HttpServeMux whose handler is the business mux wrapped by
    // the "guard" Lua filter. gs registers its default HttpServeMux only when
    // none is present, so this custom one wins: every request on :9090 runs
    // the Lua script first, regardless of the framework behind it.
    gs.Provide(func(guard *StarterLuaFilter.Filter) *gs.HttpServeMux {
        mux := http.NewServeMux()
        mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
            _, _ = w.Write([]byte("hello"))
        })
        mux.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
            _, _ = w.Write([]byte("admin ok"))
        })
        return &gs.HttpServeMux{Handler: guard.Wrap(mux)}
    }, gs.TagArg("guard"))
    gs.Run()
}
```

**scripts/guard.lua** (verbatim from the example, `example/scripts/guard.lua`):

```lua
-- observe: log every incoming request through the go-spring log pipeline.
log("incoming " .. req.method .. " " .. req.path)

-- mutate: tag every response so clients can see the filter ran.
resp.set_header("X-Lua-Filter", "guard")

-- gate: block /admin unless the request carries the expected token.
-- deny() writes the response and short-circuits the chain; always return
-- right after calling it.
if req.path == "/admin" then
    if req.header("X-Token") ~= "sesame" then
        deny(403, "forbidden: bad token")
        return
    end
end
```

**conf/app.properties** — the complete surface:

```properties
# One filter per spring.lua.filter.instances.<name> entry; bean name = <name>.
spring.lua.filter.instances.guard.script=./scripts/guard.lua
```

The listener is the built-in gs HTTP server (`spring.http.server.addr`, default `:9090`,
`spring/gs/http.go:66`) — the filter itself opens no port.

**Verify**:

```bash
go run . &
curl -i :9090/hello                              # 200, X-Lua-Filter: guard
curl -i :9090/admin                              # 403 forbidden: bad token
curl -i -H 'X-Token: sesame' :9090/admin         # 200 admin ok
```

External dependencies: none (the Lua VM is in-process).

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-lua-filter
  └─ gs.Group("${spring.lua.filter}", newFilter, destroyFilter)      starter.go:39

gs.Run()
  ├─ config bind: ${spring.lua.filter.instances.<name>} → Config (script path)
  ├─ newFilter: compileFile reads + parses + compiles the script into ONE reusable
  │   *lua.FunctionProto stored atomically (filter.go:49-66). Compile failure or a
  │   missing file fails startup — a typo never reaches production.
  ├─ VM pool created lazily: every pool.New builds a fresh sandboxed *lua.LState and
  │   registers it in f.states so shutdown can close it (filter.go:58-65, 188-207)
  ├─ wiring: gs.TagArg("<name>") injects the filter into your HttpServeMux provider
  └─ SIGTERM/shutdown: destroyFilter closes every VM the pool ever created
      (starter.go:44-47, filter.go:83-90)
```

### 2.2 One request, layer by layer

`GET /admin` without a token, through `guard.Wrap(mux)` (filter.go:95-116):

1. Borrow a VM from the pool (`pool.Get`, creating a sandbox on first use).
2. `install` re-binds the host API as globals bound to *this* request's `reqState`
   (filter.go:127-164) — this is what keeps a pooled VM stateless between requests.
3. The compiled prototype is pushed and `PCall`ed: the script runs `log()`, `resp.set_header()`,
   then its gate. `deny(403, …)` writes status+body and sets `st.denied`.
4. `st.denied` is true → return: the wrapped handler is never reached (short-circuit).
   Otherwise → `next.ServeHTTP(w, r)`.
5. Defer: `SetTop(0)` resets the stack and the VM goes back to the pool.
6. A script *runtime* error (not a deny) surfaces as HTTP 500 `lua filter error: …` and the
   wrapped handler is skipped too (filter.go:107-110).

### 2.3 Host API — everything a script can touch

Re-bound per request (filter.go:127-164); there is nothing else:

| Global | Signature | Effect |
|--------|-----------|--------|
| `req.method`, `req.path` | string fields | request method / URL path |
| `req.header(name)` | fn → string | request header value |
| `req.query(name)` | fn → string | query parameter value |
| `resp.set_header(name, value)` | fn | mutates the response headers |
| `deny(status, message)` | fn | defaults 403 / "denied by lua filter"; writes response, short-circuits |
| `log(message)` | fn | logs at Info under tag `_app_lua_filter`, prefixed with the script path |

### 2.4 Sandbox

Pooled VMs open only `base` / `table` / `string` / `math`; `dofile`, `loadfile`, `load`,
`loadstring`, `collectgarbage` are nil'd (filter.go:188-207). No filesystem, no network, no
coroutines, no `os`/`io`. The only I/O a script can perform is the host API above.

### 2.5 Gateway integration (bean-backed filter)

The gateway's `FilterWrapper` is exactly `Wrap(next http.Handler) http.Handler`
(`starter-gateway/compile.go:38-46`) — `*StarterLuaFilter.Filter` satisfies it with no adapter.
Export the bean and reference it by name in a route's filter list:

```go
gs.Provide(func(guard *StarterLuaFilter.Filter) gateway.FilterWrapper {
    return guard
}).Name("guard")   // resolves in the DSL as lua(guard)
```

```properties
spring.gateway.routes.demo.filters=lua(guard)
```

Resolution happens at route-compile time from the gateway's injected wrapper map
(`compile.go:276-285`); an unknown bean name fails startup with
`no FilterWrapper bean named …`.

---

## 3. Per-key behavior reference

### 3.1 This starter — `spring.lua.filter.instances.<name>.*` (1 key)

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `<name>.script` | string | — | **Required.** Path to the Lua source, resolved relative to the **working directory**. Read + compiled once at bean creation (filter.go:49-53, 168-182). ⚠ env override is `GS_SPRING_LUA_FILTER_<NAME>_SCRIPT`. ⚠ also the path `Reload()` re-reads later. | Missing/unreadable/invalid script → startup fails. Runs from another cwd → path resolves elsewhere (or not at all); chdir in tests to the module dir. |

That is the entire key set. Multiple entries create multiple filters, selected by bean name.

### 3.2 Load-bearing keys owned by consumers

| Key | Owner | Behavior |
|-----|-------|----------|
| `spring.http.server.addr` | gs built-in HTTP server | Listener in front of which the filter wraps (default `:9090`). |
| `spring.gateway.routes.<id>.filters` | starter-gateway | `lua(<beanName>)` token resolves the exported FilterWrapper bean (§2.5). |

---

## 4. Verification & fault drills

### 4.1 Gate + mutate verification

```bash
curl -sD- -o/dev/null :9090/hello | grep -i x-lua-filter   # guard
curl -s -o- -w '%{http_code}\n' :9090/admin                # 403 + body
curl -s -o- -w '%{http_code}\n' -H 'X-Token: sesame' :9090/admin   # 200
```

### 4.2 Per-request log tag

Every script `log()` line lands under `_app_lua_filter` (filter.go:160-163), carrying the request
context. Silence or re-level it:

```properties
logger.lua_filter.type=Logger
logger.lua_filter.level=WARN
logger.lua_filter.tag=_app_lua_filter
```

### 4.3 Hot-reload drill (no restart)

`Reload()` is **API-only** — there is no config-driven refresh. Wire it to whatever trigger you
have (an admin handler, a signal):

```go
if err := guard.Reload(); err != nil { /* bad edit: old script still running */ }
```

Drill (exactly what `example/example.go:118-131` asserts):

1. Edit `guard.lua` to also `deny(403, "hello disabled")` on `/hello`.
2. Call `Reload()` — success: subsequent requests get 403 on `/hello`, no restart.
3. Save a syntactically broken edit and `Reload()` again → returns an error; requests keep being
   served by the last good script (filter.go:72-79). A bad edit can never take the filter down.
4. In-flight requests during a successful swap finish against the previous prototype (atomic
   `proto.Store`).

### 4.4 Sandbox drill

In the script, call `dofile("/etc/passwd")` or `os.getenv("HOME")` → runtime error → HTTP 500
`lua filter error: attempt to index a nil value (global 'os')` (or nil call) — confirming the VM
opened no escape libraries.

### 4.5 Runtime-error observation

A script that throws (e.g. `error("boom")`) → 500 with body `lua filter error: boom`; the wrapped
handler is skipped. grep the access log of the fronting server for 500s under the filter's subtree.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Startup fails "lua filter: read script …" / "compile script …" | script path wrong (cwd-relative) or Lua syntax error | Fix the path/key or the script; errors cite file+line (filter.go:168-182). |
| Filter silently not running | no `spring.lua.filter.instances.*` key (no bean), or you mounted the mux without `guard.Wrap` | Add the key; wrap the handler (`TagArg("<name>")`). |
| 500 `lua filter error: …` on every request | runtime error in the script (nil index, bad arg) | Read the error text; it names the Lua line. |
| Response shows headers but deny didn't fire | script called `resp.set_header` then fell through | `deny()` must be followed by `return`; deny writes but only the flag short-circuits. |
| Double deny / "http: superfluous WriteHeader" | script calls `deny()` twice or writes after deny | Return immediately after `deny()`. |
| Script edits have no effect | `Reload()` never called — the compiled prototype is from startup | Trigger `Reload()` (API-only; see §4.3) or restart. |
| VM/memory growth under load | pool keeps every VM it created (they are reused, closed only at shutdown) | Expected bounded behavior; scripts with unbounded table growth leak per-VM — keep scripts stateless (globals re-bound per request). |
| `no FilterWrapper bean named "guard"` in a gateway app | filter bean not exported as `gateway.FilterWrapper` before warmup | Export via `.Export(gs.As[gateway.FilterWrapper]())` or a named provider (§2.5). |
| Bean name collision | another group starter also registers bean `guard` | Rename the config sub-key (bean name = sub-key, no namespace). |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 1 per instance |
| Required | 1 (`script`) |
| Quickstart external deps | 0 |
| "Watch out" entries | 4 (cwd-relative script path; Reload API-only; deny-needs-return; un-namespaced bean name) |

Design suspects (entries 1-3 carried over from the previous audit):

1. No config-driven hot reload — `Reload()` is API-only, so an operator cannot refresh a script
   without code that calls it.
2. Bean name doubles as the config sub-key with no namespace (`guard`), risking collisions with
   other group starters' beans of the same name.
3. Host API surface (no body access, no upstream inspection) is fixed in the starter; anything
   more requires editing this module.
4. `log()` is Info-level only — a script cannot choose a level, so chatty scripts need the logger
   re-levelled process-wide for the tag.
5. VM pool is unbounded (`sync.Pool` semantics + a permanent `states` registry): a burst of
   concurrent requests mints that many VMs and they are closed only at shutdown.
6. Per-request re-binding of globals is O(host API) per request — fine at the current surface
   size, a cost to watch if the API grows.
