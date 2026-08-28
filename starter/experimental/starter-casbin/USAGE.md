# starter-casbin Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `config.go`, `registry.go`) and the runnable,
self-asserting [example/](example/) (`example/check.sh`, zero external dependencies).
**Casbin's own semantics (model language, policy formats, Enforce/API, adapters, watchers) are
[the official docs](https://casbin.org/docs/overview)** — everything below is go-spring's
increment.

**Activation**: `gs.Group("${spring.casbin}")` in `init()` — every `spring.casbin.<name>`
config entry becomes one `*StarterCasbin.Enforcer` **container bean** named `<name>`.
The *enforcers* are beans; the *adapters/watchers* are **not** — they live in a package-level
side-registry (`registry.go`) and are looked up by name at construction time. There is no
`enabled` key; zero `spring.casbin.*` keys means zero enforcers and a fully inert starter.

---

## 1. Complete worked project

An HTTP service answering authorization questions from an RBAC enforcer, with hot policy
reload through a registered watcher. File tree (isomorphic to `example/`):

```
demo/
├── go.mod
├── main.go
└── conf/
    ├── app.properties
    ├── model.conf
    └── policy.csv
```

**go.mod** (deps that matter):

```
require (
    github.com/casbin/casbin/v2        v2.135.0
    go-spring.org/spring               v1.3.x
    go-spring.org/starter-casbin       latest
)
```

**conf/model.conf** — a minimal RBAC model (see [model syntax](https://casbin.org/docs/syntax-for-models)):

```ini
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && r.obj == p.obj && r.act == p.act
```

**conf/policy.csv**:

```csv
p, admin,  /data, read
p, admin,  /data, write
p, viewer, /data, read
g, alice, admin
g, bob,   viewer
```

**main.go**:

```go
package main

import (
    "net/http"
    "os"

    fileadapter "github.com/casbin/casbin/v2/persist/file-adapter"
    "go-spring.org/spring/gs"

    StarterCasbin "go-spring.org/starter-casbin"
    _ "go-spring.org/starter-casbin"
)

// Service consumes the enforcer purely by injection. The bean is named after its
// config group key (${spring.casbin.rbac.*} -> "rbac"), hence `autowire:"rbac"`.
// *StarterCasbin.Enforcer embeds *casbin.Enforcer, so Enforce/AddPolicy/... are
// used exactly like upstream.
type Service struct {
    Enforcer *StarterCasbin.Enforcer `autowire:"rbac"`
}

func (s *Service) Allowed(sub, obj, act string) bool {
    ok, err := s.Enforcer.Enforce(sub, obj, act)
    return err == nil && ok
}

func main() {
    // Register side-registry resources BEFORE gs.Run: the enforcer constructor
    // resolves `adapter=` / `watcher=` names from these maps (registry.go).
    StarterCasbin.RegisterAdapter("file", fileadapter.NewAdapter("./conf/policy.csv"))

    // Export the service as a root object so the container instantiates it
    // (nothing else injects it; unexported-only beans are never built in prod).
    // The autowired Enforcer is available after gs.Run assembles the bean.
    svr := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())

    http.HandleFunc("/enforce", func(w http.ResponseWriter, r *http.Request) {
        q := r.URL.Query()
        s := svr.Interface().(*Service)
        if s.Allowed(q.Get("sub"), q.Get("obj"), q.Get("act")) {
            _, _ = w.Write([]byte("allow"))
            return
        }
        _, _ = w.Write([]byte("deny"))
    })

    if err := http.ListenAndServe(":9090", nil); err != nil {
        os.Exit(1)
    }
}
```

**conf/app.properties** — the complete, commented surface:

```properties
# One enforcer per group key. "rbac" is the bean name you autowire.
# Model file (required — no default).
spring.casbin.rbac.model=./conf/model.conf
# Adapter name from RegisterAdapter. When set, the enforcer loads/saves
# through the adapter (starter.go newEnforcer). Mutually exclusive with
# `policy` — setting both is a startup error.
spring.casbin.rbac.adapter=file
# Watcher name from RegisterWatcher. When set, peer change signals fire an
# automatic LoadPolicy (hot reload / multi-instance sync).
spring.casbin.rbac.watcher=local
# Dependency-free alternative to adapter=: plain file-backed policy.
# spring.casbin.rbac.policy=./conf/policy.csv
# Persist AddPolicy/RemovePolicy back to storage (default true).
spring.casbin.rbac.autoSave=true
```

**Verify** (from `example/`, which self-asserts all of this in `runTest`):

```bash
cd example && ./check.sh                      # exits 0; prints "hot reload applied"
go run . -manual &                            # keep the server up
curl 'http://127.0.0.1:9090/enforce?sub=alice&obj=/data&act=write'   # allow
curl 'http://127.0.0.1:9090/enforce?sub=bob&obj=/data&act=write'     # deny
curl 'http://127.0.0.1:9090/enforce?sub=carol&obj=/data&act=read'    # deny
```

---

## 2. Assembly & timing

### 2.1 Lifecycle timeline

```
func init() (user code, before gs.Run)
  └─ StarterCasbin.RegisterAdapter("file", a)   [side-registry map, registry.go:43]
  └─ StarterCasbin.RegisterWatcher("local", w)  [side-registry map, registry.go:52]

import starter-casbin
  └─ init(): gs.Group("${spring.casbin}", newEnforcer, destroyEnforcer)  [starter.go:32]

gs.Run()
  ├─ config bind: each spring.casbin.<name>.* → Config (value tags)
  ├─ bean construction: newEnforcer(ctx, name, c) per group key
  │    ├─ adapter set → lookupAdapter, casbin.NewEnforcer(model, adapter)
  │    ├─ else        → casbin.NewEnforcer(model, policy-file)
  │    ├─ e.EnableAutoSave(c.AutoSave)
  │    └─ watcher set → lookupWatcher → e.SetWatcher(w) →
  │       w.SetUpdateCallback(func(string){ _ = e.LoadPolicy() })   [starter.go:85]
  ├─ Run/serve: your code calls Enforce on the injected bean
  └─ shutdown: destroyEnforcer → watcher.Close() only  [starter.go:95-100]
```

Design rationale, cited from source comments:

- **Side-registry instead of adapter beans** (registry.go:25-30): the starter stays free of
  any database/storage driver — a built-in GORM/Redis/etcd adapter would drag those deps
  into every project that only needs the file policy. Applications register what they use.
- **`*Enforcer` wrapper instead of raw `*casbin.Enforcer`** (starter.go:35-39): the starter
  must own resources Casbin does not close itself — the watcher's background work, released
  in `destroyEnforcer`. Embedding promotes the full upstream API so callers see no difference.
- **Watcher callback** (starter.go:83-85): "a classic callback reloads the policy so this
  instance picks up changes made by peers" — that is the entire hot-reload mechanism; there
  is no polling, no gs.Dync involvement.

### 2.2 One enforcement decision, step by step

`GET /enforce?sub=alice&obj=/data&act=write`:

1. Handler resolves the injected `*Enforcer` (bean "rbac").
2. `Enforce("alice", "/data", "write")` → embedded `*casbin.Enforcer` evaluates the model's
   matcher against the loaded policy matrix (upstream semantics — official docs).
3. Role inheritance `g, alice, admin` grants admin's `p, admin, /data, write` → **allow**.
4. If a peer instance had called `AddPolicy` and fired the watcher, the callback already ran
   `LoadPolicy`, so this decision used the fresh matrix — no restart, no code.

---

## 3. Per-key behavior reference

All keys live under `spring.casbin.<name>.*` (one group per enforcer bean).

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `model` | string | — | **Required.** Path to the Casbin model file, passed as the first `NewEnforcer` arg. | Missing/invalid → container fails at construction (`failed to create casbin enforcer`). |
| `policy` | string | `""` | File-backed policy, used only when `adapter` is empty (`newEnforcer`). **Mutually exclusive with `adapter`** — setting both fails startup. | Empty with no adapter → enforcer starts with an **empty policy** — everything denied, no startup error (value tag cannot express conditional-required). |
| `adapter` | string | `""` | Name in the side-registry (`RegisterAdapter`). Selected via `lookupAdapter` at construction. | Unknown name → fail-fast startup error `casbin: adapter %q not registered`. Registered after `gs.Run` → same error (lookup happens at bean construction). |
| `watcher` | string | `""` | Name in the side-registry (`RegisterWatcher`); wires `SetWatcher` + reload callback; closed at shutdown by `destroyEnforcer`. | Unknown name → fail-fast `casbin: watcher %q not registered`. |
| `autoSave` | bool | `true` | Passed to `e.EnableAutoSave` — whether `AddPolicy`/`RemovePolicy` persist back to the adapter/policy file. | `false` + expecting persistence → mutations vanish on reload. |

⚠ Coupling: `adapter` and `policy` are mutually exclusive storage selections — configuring both
is a fail-fast startup error (`casbin: `policy` and `adapter` are mutually exclusive`). ⚠
`adapter`/`watcher` values are **registry names, not bean names**:
they cannot be resolved via autowire and must exist before `gs.Run`.

---

## 4. Verification & fault drills

### 4.1 Enforcement decision drill (matches example `runTest`)

```bash
curl '...?sub=alice&obj=/data&act=read'    # allow (admin role)
curl '...?sub=alice&obj=/data&act=write'   # allow
curl '...?sub=bob&obj=/data&act=read'      # allow (viewer)
curl '...?sub=bob&obj=/data&act=write'     # deny
curl '...?sub=carol&obj=/data&act=read'    # deny (unknown subject)
```

### 4.2 Hot-reload drill (watcher path, no restart)

1. Start the example with `-manual` (its `localWatcher` stands in for a distributed watcher).
2. Append a grant to the backing store: `echo 'g, carol, admin' >> <policyPath>`.
3. Fire the watcher — in the example, `watcher.Update()`; with Redis/etcd watchers, the
   peer's `Update()` call does it.
4. Re-ask: `curl '...?sub=carol&obj=/data&act=read'` → **allow**. The callback ran
   `LoadPolicy` (starter.go:85) before you asked.

### 4.3 Misconfiguration drills

- Point `adapter` at an unregistered name → boot fails with
  `casbin: adapter "db" not registered` (errutil-wrapped, `newEnforcer`).
- Set neither `policy` nor `adapter` → boots fine; every Enforce returns deny. This is the
  silent-failure mode flagged in §6.
- Shutdown drill: with a watcher configured, SIGTERM runs `destroyEnforcer`, which closes
  the watcher — verify with a watcher whose `Close()` logs; with no watcher it is a no-op.

### 4.4 Startup diagnostics

Construction logs one Debug line (`creating casbin enforcer model=... adapter=... watcher=...`,
tag `app-def`, starter.go:57) and one Error line on failure. No runtime log tag, no health
indicator, no metrics — see §6.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Boot fails: `adapter "x" not registered` | `RegisterAdapter` never called, or called after `gs.Run` | Register in `func init()` / early `main`, before `gs.Run`. |
| Boot fails: `watcher "x" not registered` | Same, for `RegisterWatcher` | Register before `gs.Run`. |
| Boot fails: `failed to create casbin enforcer` | Bad `model` path or invalid model/policy syntax | Check path (relative to process cwd) and model file against official syntax docs. |
| Everything denied, no error | Both `policy` and `adapter` empty → empty policy loaded | Set `policy` (or an adapter) — there is no fail-fast for this case. |
| Changed `policy` file but decisions unchanged | Policy is loaded once at construction; no watcher configured | Call `LoadPolicy()` on the bean, or configure a watcher for hot reload. |
| Boot fails: `policy` and `adapter` are mutually exclusive | Both `policy` and `adapter` configured | Pick one storage source; drop the other key. |
| Multi-instance: one instance's policy changes don't propagate | Watcher callback reloads, but only on an explicit `Update()` signal from the mutating peer | Ensure the mutating instance calls `watcher.Update()` after `SavePolicy`/`AddPolicy` (upstream watcher semantics). |
| `AddPolicy` returns ok but data lost after restart | `autoSave=false` | Set `spring.casbin.<name>.autoSave=true` or call `SavePolicy()` explicitly. |
| Bean not built in prod, works in tests | Service not root-reachable (no Export/injector) | `gs.Provide(&S{}).Export(gs.As[gs.Rooter]())` — see memory note on root-reachable beans. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 5 per instance |
| Required | 1 (`model`; `policy` conditionally, see ⚠) |
| Quickstart external deps | 0 |
| "Watch out" entries | 3 |

Design suspects (kept from the previous audit, re-verified):

1. Adapter/watcher use a side registry parallel to the IoC container — two name spaces for
   the same concept; beans cannot be injected into an adapter built by the group factory
   (registry.go:25-38).
2. `policy` is conditionally required (only without `adapter`), which the `value` tag cannot
   express — an empty-policy enforcer starts silently rather than failing fast.
3. No observe wiring (health/metric) despite being a stateful, hot-reloaded component.

Newly recorded while writing this reference:

4. ~~`adapter` silently overrides `policy` when both are set~~ — fixed: configuring both is now
   a fail-fast startup error (`newEnforcer`).
