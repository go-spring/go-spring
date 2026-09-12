# starter-config-etcd Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified against
the starter source (`starter.go`), the config core
(`spring/conf/provider/provider.go`, `spring/gs/internal/gs_conf/conf.go`, `spring/gs/internal/gs_app/app.go`)
and the smoke-tested [example/](example/). **etcd's own semantics (KV model, watches, auth, leases,
the etcdctl tool) are [etcd's documentation](https://etcd.io/docs/v3.5/)** — everything below is
go-spring's increment.

**Activation — a single path, no `enabled` key anywhere:** any `etcd:` entry in
`spring.config.import` (e.g. `spring.config.import=etcd:127.0.0.1:2379/key`). The blank import
alone does nothing. The imported keys feed application properties and hot-reload `gs.Dync[T]`
fields.

Governance rule sourcing from etcd now lives in its own module,
`go-spring.org/starter-governance-etcd` (`govern.source.etcd.*`).

---

## 1. Complete worked project

A service that keeps its `demo.message` in etcd and hot-reloads it without restart. File tree
(isomorphic to the smoke-tested `example/`):

```
demo/
├── go.mod
├── main.go
├── conf/
│   └── app.properties
└── docker-compose.yml        # one standalone etcd v3.5, port bound to 127.0.0.1:2379
```

**go.mod** (deps that matter):

```
require (
    go-spring.org/spring              v1.3.x
    go-spring.org/starter-config-etcd latest
    go.etcd.io/etcd/client/v3         v3.6.x   // only if you publish from the app itself
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-config-etcd"
)

// Demo binds a dynamic field sourced from the imported etcd key. ONLY gs.Dync[T]
// fields hot-reload — a plain string field would be startup-only.
type Demo struct {
    Message gs.Dync[string] `value:"${demo.message:=none}"`
}

func main() {
    // Export as gs.Rooter so the container creates the bean eagerly (unexported,
    // non-injected beans are not wired in prod runs).
    gs.Provide(&Demo{}).Export(gs.As[gs.Rooter]())
    gs.Run()
}
```

**conf/app.properties** — the complete surface:

```properties
# --- app config from etcd ----------------------------------------------------
# optional: the app starts even when the key does not exist yet; the value is
# picked up on first refresh after it is published. format= is redundant here
# only because the key has no extension (default inference would be properties).
spring.config.import=optional:etcd:127.0.0.1:2379/gs-config-demo?format=properties
```

Seed the key before start (or rely on `optional:`):

```bash
docker compose up -d
ETCDCTL_API=3 etcdctl put gs-config-demo "demo.message=hello"
go run .
```

**Verify** (mirrors example/check.sh, which self-asserts the same hot-reload):

```bash
# cold load worked — check the load line in logs (tag _app_config_etcd):
#   loaded etcd config from key=gs-config-demo keys=1
# hot reload, no restart:
etcdctl put gs-config-demo "demo.message=hello-2"     # bound gs.Dync field flips
```

---

## 2. Assembly & timing

### 2.1 Where imports resolve in the lifecycle — and why it is early

```
blank-import starter-config-etcd
  └─ init(): conf.RegisterProvider("etcd", etcdController.Load)

gs.Run() → App.Start()
  1. mount the gs.RefreshProperties / gs.AppStarted facade targets
  2. refresh properties: load ./conf files → loadFileImports reads
     spring.config.import → conf.Resolve(${...} in the source string)
     → provider.Load parses [optional:]etcd:<path> → etcdCtrl.Load:
        parseSource → clientFor (cached per endpoint|user|pass)
        → registerWatcher (dedup per client+key)  ← watch armed BEFORE the get
        → Get (5s ctx timeout) → reader.Read(format) → flatten
  3. init logging
  4. IoC container wiring (App as root); bean value tags now bind against
     merged props
  5. Runners → Servers → ready
```

Properties must exist **before step 4**: every `value:"${...}"` tag resolves during wiring, so an
imported etcd key can feed any bean's configuration. The etcd Get itself happens inside step 2 —
a required import that cannot be read fails startup right there (or is skipped with `optional:`).

Notes verified in source:

- **One level of imports**: `spring.config.import` is read only from the top-level config files; an
  `spring.config.import` key inside an imported document is silently ignored (gs_conf/conf.go
  `loadFileImports` doc).
- **Ordering**: imported sources are layered into the same storage as files; later-loaded sources
  override earlier ones (same doc).
- **Placeholder resolution**: the import source string goes through `conf.Resolve` first, so
  `etcd:${etcd.addr:=127.0.0.1:2379}/key` works.

### 2.2 Watch → refresh path, step by step

```
etcd PUT on a watched key
  → clientv3.Watch channel delivers a WatchResponse with events
  → watcher goroutine: len(wr.Events) > 0 → etcdCtrl.TriggerRefresh()
  → gs.RefreshProperties() → App.RefreshProperties():
       re-run the WHOLE property load (all files + all imports) → merge
       → propagate to the container → gs.Dync[T] fields update atomically
```

- Before the app has started, `gs.RefreshProperties()` returns an error, so `TriggerRefresh` is a
  **harmless no-op** — the initial load already captured the state (starter.go comments).
- The watch is a **single-key** watch (no `WithPrefix`): only the exact key named in the import.
  Deletes also count as events: a delete on a *required* imported key logs a WARN
  (`etcd key ... deleted; stale snapshot retained until the key is restored`) plus a WARN on the
  failed refresh itself; the **old merged properties stay in effect** — see §5. Deleting an
  *optional* imported key stays quiet by design (its properties simply disappear).
- Refresh is full-application: a change in one imported key re-reads *every* source. Only
  `gs.Dync[T]` fields change; plain fields and already-bound bean config are startup-only.

### 2.3 Governance rules-push path

Governance rule sourcing from etcd now lives in its own module,
`go-spring.org/starter-governance-etcd` (`govern.source.etcd.*`).

---

## 3. Per-key behavior reference

### 3.1 Import-string surface — `[optional:]etcd:<host>:<port>/<key>?<query>`

The `[optional:]<provider>:<path>` grammar is core (spring/conf/provider/provider.go:74-104);
everything after the leading `etcd:` is this starter's `<path>`, parsed as a URL
(`etcd://` + path, starter.go `parseSource`).

| Part | Type | Default | Behavior / interactions | Misconfiguration consequence |
|------|------|---------|-------------------------|------------------------------|
| `optional:` | flag | absent | Skips the source when the Get fails **or** the key is empty (Warn log). Does NOT suppress: malformed source strings, parse errors of fetched content, or bad `dial-timeout` values. | Omitting it + missing key → startup fails (intended); adding it hides real outages as "empty config". |
| host:port | string | — (**required**) | Single endpoint per source; also the client-cache key component. | Missing (`etcd:/key`) → `missing etcd server address` error. Only one endpoint — no cluster list ⚠ (see §6). |
| key (path) | string | — (**required**) | Leading `/` trimmed; exact single key, no prefix/range. Drives format inference from its extension. | Missing → `missing etcd key`. Pointing at a directory-like prefix silently watches/reads nothing. |
| `format` | string | key extension, else `properties` | Any format registered in the reader registry (`properties`/`yaml`/`toml`/`json`...), name or dotted form. | Unknown format → `unsupported config format` at load. Wrong-but-known format → parse error at load. |
| `username` / `password` | string | empty | etcd [auth](https://etcd.io/docs/v3.5/op-guide/authentication/) credentials on the cached client. Part of the client cache key, so different credentials get different clients. | Missing creds on an auth-enabled cluster → Get fails → startup fails (or is skipped when optional). ⚠ plaintext in the import string, visible in logs/sources. |
| `dial-timeout` | duration | `5s` | `time.ParseDuration` syntax (`2s`, `500ms`). Client dial timeout only — the Get itself always uses a hard-coded 5s context. | Non-duration value → `invalid dial-timeout` error at load. Too low → client creation failures on slow networks. |

Multiple import entries compose: `spring.config.import` accepts a list; later entries override
earlier ones for overlapping keys. Each entry gets its own client (per endpoint/creds) and its own
watcher (deduplicated per client+key).

### 3.2 Governance property surface — moved out

Governance rule sourcing from etcd now lives in its own module,
`go-spring.org/starter-governance-etcd` (`govern.source.etcd.*`).

### 3.3 Keys this starter does NOT have

No `enabled` switch (activation is the presence of an import entry), no TLS keys, no
endpoints list, no watch-prefix, no refresh interval (refresh is event-driven). See §6.

---

## 4. Verification & fault drills

All drills against the §1 project. Prereqs: `docker compose up -d`, `etcdctl` available
(`ETCDCTL_API=3`; the compose etcd has no auth, so no credentials needed).

### 4.1 Cold load

```bash
etcdctl put gs-config-demo "demo.message=v1"
go run . &        # log line: loaded etcd config from key=gs-config-demo keys=1 (tag _app_config_etcd)
```

Delete the key first and drop `optional:` from the import → startup must fail with
`etcd key gs-config-demo is empty`. Keep `optional:` → app starts with the `:=none` default and a
Warn `optional config key ... is empty (skipped)`.

### 4.2 Watch push (hot reload)

```bash
go run . &
etcdctl put gs-config-demo "demo.message=v2"    # no restart; gs.Dync field flips to v2
```

This is exactly what example/check.sh asserts (the example self-publishes `hello-<ts>` and fails
non-zero if the field does not flip within 15s). Note the refresh re-reads ALL sources — a value
edited in a local conf file since startup also flips on the next etcd-triggered refresh.

### 4.3 Malformed value handling

- **App-config key**: `etcdctl put gs-config-demo "demo.message={{{"` (with `format=yaml`) → parse
  error at next refresh; refresh aborts, old properties stay, Error log. At startup (non-optional)
  the same content fails startup.

### 4.4 Optional vs required import entries

| Drill | Expected |
|-------|----------|
| required + key missing | startup error `etcd key ... is empty` |
| optional + key missing | starts, Warn log, `:=` defaults in effect |
| required + etcd down | startup error `get etcd key ... failed` |
| optional + etcd down | starts, Warn `optional config get key ... failed (skipped)` |
| optional + malformed source string | still a hard startup error — optional never covers typos |

### 4.5 Connectivity failure after startup

Stop etcd (`docker compose stop`): watchers' channels close silently; no log, no metric, no health
flip. Restart etcd: clientv3 reconnects transparently and watch delivery resumes (etcd clientv3
semantics); a PUT made while down is **not** replayed as a refresh trigger unless the key changed
again after reconnection — verify with a fresh `etcdctl put` and observe the refresh. This drill
demonstrates the observability gap recorded in §6.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Startup fails `missing etcd server address` / `missing etcd key` | Malformed import string (no host / no path) | Fix the `etcd:<host>:<port>/<key>` shape; `optional:` does not cover this. |
| Startup fails `etcd key ... is empty` | Required import names a key that does not exist (or points at a prefix with no exact key) | Create the key, or mark the entry `optional:` if absence is legitimate. |
| Startup fails `get etcd key ... failed` | etcd down, wrong port, auth required but no username/password | Check endpoint / add `?username=..&password=..`; verify with `etcdctl --user ... get <key>`. |
| App runs but imported values never appear | Import key present but the app started before the first PUT **and** the entry is optional → defaults in effect | PUT the key; the watcher triggers a refresh and values arrive without restart. |
| Bound field does not hot-reload | Field is a plain value, not `gs.Dync[T]` | Only `gs.Dync[T]` refreshes; plain fields are startup-only (gs refresh contract). |
| Refresh seems dead after a delete on an imported key | Delete fired a refresh; the re-load of a required key failed; the bridge swallows the error and keeps old properties | Re-PUT the key; consider whether deletes are part of your config workflow. |
| `unsupported config format` | format query/property names an unregistered format | Use a registered one (properties/yaml/toml/json) or register a reader. |

---

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config surfaces | 5 import-string params |
| Required | 2 import parts (host, key) |
| Quickstart external deps | 1 (etcd; compose file provided) |
| "Watch out" entries | 5 (single endpoint, plaintext creds, silent watch gaps, delete semantics, Dync-only refresh) |

Design suspects (for the audit ledger):

- Single endpoint, no endpoints list, no TLS options in the source string — production etcd clusters
  need all three → candidate cluster-source support.
- No watcher-error surfacing: a dropped/silent watch means stale config with no metric or health
  signal (demonstrated in §4.5) → observability increment candidate.
- Delete on a required imported key keeps the last-good properties (correct), and now surfaces it:
  a WARN names the deleted key and each failed refresh (previously fully silent).
- Credentials live in plaintext in the import string / properties and inside the client cache key
  string; no ENC()/KMS guidance specific to this starter.
- Hard-coded 5s Get context is not tunable while dial-timeout is — asymmetric timeout surface.
