# starter-config-consul Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified against
the starter source (`starter.go`, `starter_test.go`), the core import machinery
(`spring/conf/provider/provider.go`, `spring/gs/internal/gs_conf/conf.go`,
`spring/gs/internal/gs_app/app.go`), and the smoke-tested [example/](example/). **Consul's own
semantics (KV store, blocking queries, ACL tokens, datacenters) are
[Consul's documentation](https://developer.hashicorp.com/consul/docs/dynamic-app-config/kv)** —
everything below is go-spring's increment.

**Activation**: blank-importing the package registers the `consul` config provider plus the
change-to-refresh bridge (`starter.go:44-56`). It activates only when a `consul:` entry appears in
`spring.config.import`; the import string is the *entire* configuration surface — the starter binds
no `value:` tags of its own.

> This starter lives under `experimental/` — an **unreviewed** marker, not a quality grade.

---

## 1. Complete worked project

A minimal service whose `demo.message` lives in Consul KV and hot-reloads on change. File tree
(isomorphic to the smoke-tested [example/](example/)):

```
demo/
├── go.mod
├── main.go
├── conf/
│   └── app.properties
└── docker-compose.yml
```

**go.mod** (deps that matter):

```
module demo

require (
    github.com/hashicorp/consul/api v1.34.1   // only if you publish from Go, as below
    go-spring.org/spring            v1.3.x
    go-spring.org/starter-config-consul latest
)
```

**conf/app.properties** — the complete, commented surface:

```properties
# Import configuration from the Consul KV store.
# Grammar: [optional:]consul:<host>:<port>/<kv-path>?<query>
# "optional:" lets the app start even when the key does not exist yet; the
# value is filled in once published and refreshed live via the blocking-query
# watcher (starter.go:196-220 for the optional skips, §2.2 for the watch).
spring.config.import=optional:consul:127.0.0.1:8500/gs-config-demo?format=properties
```

**docker-compose.yml** (same as example/):

```yaml
services:
  consul:
    image: hashicorp/consul:1.18
    command: agent -dev -client=0.0.0.0
    ports:
      - "127.0.0.1:8500:8500"
```

**main.go**:

```go
package main

import (
    "fmt"

    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-config-consul"
)

// Demo binds a dynamic field sourced from the imported Consul KV path.
// Registered as a root object so the container creates it eagerly.
// ONLY gs.Dync[T] fields hot-reload — a plain string would be frozen at boot.
type Demo struct {
    Message gs.Dync[string] `value:"${demo.message:=none}"`
}

func main() {
    demo := gs.Provide(&Demo{}).Export(gs.As[gs.Rooter]())
    gs.Run()
    fmt.Println("final:", demo.Interface().(*Demo).Message.Value())
}
```

**Verify** (same commands the example's check.sh exercises):

```bash
docker compose up -d
# wait for readiness — the leader endpoint is the real signal:
curl -fsS http://127.0.0.1:8500/v1/status/leader

# publish the config the import points at:
consul kv put gs-config-demo "demo.message=hello"     # or: curl -X PUT \
    --data-binary 'demo.message=hello' \
    http://127.0.0.1:8500/v1/kv/gs-config-demo

go run .        # logs: loaded consul config from kvPath=gs-config-demo keys=1

# hot-reload without restart (see §4.2):
consul kv put gs-config-demo "demo.message=hello-2"
```

Run `bash example/check.sh` to execute the full docker-gated smoke test (dev Consul + example that
self-asserts the hot-reload within 15 s, then exits 0 on SIGTERM).

---

## 2. Assembly & timing

### 2.1 Lifecycle timeline

```
blank import starter-config-consul
  └─ init(): gs.Provide(consulCtrl).Export(gs.As[gs.Rooter]())     starter.go:50
             conf.RegisterProvider("consul", consulCtrl.Load)      starter.go:55

gs.Run()
  ├─ config phase (BEAN-LESS, hence pre-bean):
  │    AppConfig.Refresh()                                  gs_conf/conf.go:84
  │      └─ loadFiles → app.properties
  │            └─ loadFileImports: ${spring.config.import}   gs_conf/conf.go:216
  │                  └─ conf.Load("optional:consul:...")    provider.go:74
  │                        ├─ optional/provider split (first ':')
  │                        ├─ consulCtrl.Load → parseSource → KV Get (cold load)
  │                        └─ registerWatch → one goroutine per (client, kvPath)
  ├─ IoC wiring: consulCtrl is a Rooter, autowired *gs.PropertiesRefresher fills
  │    c.Refresher (nil before this point — TriggerRefresh is a no-op, starter.go:86-90)
  ├─ Runners/Servers, readiness
  └─ steady state: watchLoop's blocking query fires RefreshProperties on index bump
```

**Why pre-bean**: imports resolve inside `AppConfig.Refresh()`, which runs before the IoC container
is wired (`app.go:272-279` startup sequence). The provider must therefore be registered from an
`init()` — package-level state, not a bean — and the refresh bridge tolerates being unwired
(`TriggerRefresh` nil-check, `starter.go:86-90`; covered by
`TestTriggerRefreshNilRefresherIsNoop`).

### 2.2 Watch / refresh path, walked

1. `registerWatch` (`starter.go:234-249`) dedupes on `clientKey + "|" + kvPath`; repeated Loads of
   the same source — which happen on *every* property refresh — spawn exactly one goroutine
   (`TestWatchRegisteredOncePerSource`).
2. `watchLoop` (`starter.go:252-283`) issues a Consul **blocking query**: `Get` with
   `WaitIndex: lastIndex`, `WaitTime: 5m`. It swallows the initial index (first poll only
   establishes the baseline), triggers `TriggerRefresh()` when `LastIndex` advances, resets to 0 on
   index regression (Consul restart / index reset), and retries after 2 s on transport errors.
3. `TriggerRefresh` → `PropertiesRefresher.RefreshProperties()` (`app.go:149-151`) → full
   `AppConfig.Refresh()` rebuilds the layered storage from scratch (files, env, cmd, imports — so
   the KV entry is *re-fetched*, `starter.go:196`) → the container propagates the new snapshot to
   every `gs.Dync[T]` field atomically (`app.go:234-256`). Non-`Dync` bindings never re-run.
4. A failed refresh (e.g. the KV key was deleted while non-optional) leaves the old snapshot in
   place, now logged: the watcher emits a WARN naming the key on deletion and a WARN on each
   failed refresh; the loop keeps watching.

Import mechanics to know (`gs_conf/conf.go`): imports declared in an imported file are **silently
ignored** (one level only, `conf.go:213-215`); import strings go through placeholder resolution
(`conf.go:224`), so `${...}` expressions work inside them; imported sources land in the
`StorageAppFile` layer, i.e. same precedence as `app.properties`, later imports overriding earlier
ones.

---

## 3. Per-key behavior reference

There are **no property keys** in this starter — the only `value:` tags in the module tree belong to
the example's demo bean (`${demo.message:=none}`, a top-level absolute reference to a key *inside
the imported KV entry*, not a starter key). The full surface is the import string:

Grammar (core split at `provider.go:84-92`, consul part at `starter.go:104-137`):

```
[optional:]consul:<host>:<port>/<kv-path>?format=..&scheme=..&token=..&datacenter=..
```

| Part | Type | Default | Behavior / interactions | Misconfiguration consequence |
|------|------|---------|-------------------------|------------------------------|
| `optional:` prefix | flag | absent | Core-layer semantics (`provider.go:85-88`): with it, a failed Get / missing key / empty value at **cold load** is skipped with a Warn log (`starter.go:198-220`). ⚠ Does **not** cover parse errors, client-creation errors, or the watch-time refresh path. | Without it, a missing key or unreachable agent aborts startup (`consul kv <path> not found`). |
| `host:port` | string | — | Consul agent address (HTTP API). **Required** — empty host fails parse (`starter.go:109-111`). | Startup error `missing consul server address in %q`. |
| `<kv-path>` | string | — | Exactly **one** KV entry per import (`cli.Get`, not a prefix list). Leading `/` trimmed. Required (`starter.go:112-115`). | Startup error `missing kv path in %q`. No way to read a key directory. |
| `format` | string | extension of kv-path, else `properties` | Content parser selector (`reader.Read`, `starter.go:222`): `properties`/`yaml`/`toml`/`json`. Inferred from the path's extension — a key named `app` containing YAML parses as properties unless `?format=yaml` is set. | Parse error at load: `parse consul kv %s as %s failed` — fails even when `optional:` (parse errors are not optional-skipped, `starter.go:222-226`). |
| `scheme` | string | `http` | Passed to `api.Config.Scheme` (`starter.go:161-166`). Part of the client cache key. | `https` against a plain-HTTP port → every Get errors; non-optional startup failure, optional → skipped silently. |
| `token` | string | empty (anonymous) | Consul ACL token, sent with every request. ⚠ Inline in the import string it lands in config files/logs — no env-var or token-file fallback. | Wrong/insufficient token → 403 on Get; surfaces as `get consul kv %s failed`. |
| `datacenter` | string | agent default | `QueryOptions.Datacenter` override for cold load and watch (`starter.go:196, 256`). Part of the client cache key. | Unknown dc → error path same as above. |

Clients are cached per `(address, scheme, token, datacenter)` tuple (`clientKey`, `starter.go:142-144`);
watches dedupe per client+kvPath. Two imports sharing a tuple share one client and two watch
goroutines.

Precedence note: imported sources sit in the `StorageAppFile` layer (same as `app.properties`),
below profile files, env and command line (`gs_conf/conf.go:27-41`) — env vars override Consul KV
values for non-`Dync` bindings and for the merged property set.

---

## 4. Verification & fault drills

### 4.1 Cold load

```bash
consul kv put gs-config-demo "demo.message=hello"
go run .    # expect log: loaded consul config from kvPath=gs-config-demo keys=1 (tag "def")
```

Turn on debug to see the parse: the starter logs `loading config from address=... kvPath=...
format=...` at Debug level (`starter.go:186`). Missing/failed loads log at Error/Warn with the same
`def` app tag — `grep 'consul' app.log`.

### 4.2 Watch push (hot-reload, no restart)

```bash
# with the app running from §4.1:
consul kv put gs-config-demo "demo.message=hello-2"
# within ~seconds the blocking query returns; gs.Dync[string] Message flips to hello-2
```

The example automates exactly this: it publishes `hello-<timestamp>`, polls the `Dync` field for up
to 15 s, and exits non-zero on timeout (`example/example.go`, `runTest`). Blocking-query latency is
sub-second on a local agent; worst case one `WaitTime` cycle (5 min) after a missed notification.

### 4.3 Malformed value

```bash
consul kv put app-json '{not-json'    # with import .../app-json?format=json
go run .                              # startup fails: parse consul kv app-json as json failed
```

Note `optional:` does **not** rescue this — only fetch-failure/missing/empty are optional-skipped
(`starter.go:196-220` vs `222-226`; `TestLoadParseErrorPropagates`).

### 4.4 Optional vs required

```bash
consul kv delete gs-config-demo
go run .   # optional: starts, logs "optional config kv gs-config-demo not found (skipped)"
           # non-optional: aborts with "consul kv gs-config-demo not found"
```

Required-vs-optional is also observable for an unreachable agent: stop docker (`compose stop`) and
compare `optional:consul:...` (skips with Warn) against `consul:...` (fails `get consul kv ...
failed`).

### 4.5 Consul down at steady state

With the app running and the KV published, `docker compose stop`: the watcher's blocking query
errors, retries every 2 s (`starter.go:261-263`), and the app keeps serving the last snapshot —
config goes stale but nothing crashes and nothing logs per retry beyond the SDK's transport errors.
Restart Consul and the next KV change resumes hot-reload automatically (index regression resets the
baseline, `starter.go:269-271`).

### 4.6 Key deleted at steady state (non-optional)

Delete the key while running: the watch fires a refresh, but the re-Load inside the refresh fails
(key not found) → `RefreshProperties` errors → the watcher logs a WARN naming the key and another
WARN for the failed refresh, and the **old snapshot is retained** (snapshot only swapped on
success, `conf.go:130`). The app keeps the last-known values; the WARNs are the staleness signal.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Startup fails `consul kv <path> not found` | key absent, import not `optional:` | `consul kv put` the key or prefix `optional:` (decide whether absent-config is really OK). |
| Startup fails `get consul kv <path> failed` | agent unreachable, wrong scheme, bad ACL token, unknown datacenter | `curl http://<host>:8500/v1/status/leader`; check `scheme=`/`token=`/`datacenter=` params. |
| Startup fails `parse consul kv %s as %s failed` | content doesn't match format (often format inferred as `properties` from an extension-less key) | add `?format=yaml|json|toml`, or name the key with the right extension. |
| App runs but no config from Consul, only a Warn | `optional:` skipped a missing/failed load | check the Warn line for which of the three skip reasons; publish the key / fix connectivity. |
| Value changes in Consul but app doesn't update | bound field is not `gs.Dync[T]` (plain field frozen at boot), or watch goroutine never started | use `gs.Dync`; confirm the import actually ran (look for the `loaded consul config` Info log). |
| Hot-reload worked once, then stops | Consul restarted and index regressed, or the KV was deleted (refresh fails silently, stale snapshot) | re-publish the key; check §4.5/§4.6 semantics. |
| 403 / permission denied on Get | ACL token missing or lacks `key:read` on the path | pass `?token=...` with a policy granting read; see [Consul ACL docs](https://developer.hashicorp.com/consul/docs/security/acl). |
| Both files and Consul define a key, wrong one wins | layering: imports sit at `StorageAppFile` level; env/cmd/profile override them | move the override to a higher layer, or drop the key from the lower one (`gs_conf/conf.go:27-41`). |
| Import in the imported KV entry ignored | only one import level is processed by design (`conf.go:213-215`) | declare all imports in the local `app.properties`. |

---

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys (import-string params) | 7 (incl. `optional:` prefix) |
| Property keys bound by the starter | 0 |
| Required | 2 (host:port, kv-path) |
| Quickstart external deps | 1 (Consul agent) |
| "Watch out" entries | 5 (§3 ⚠ ×2, §4.3/§4.6/§5 rows) |

Design suspects (for the audit ledger):

- ACL token inline in the import string lands in config files — no env-var or token-file fallback
  like vault's → candidate out-of-band credential path.
- One KV entry per import only; no prefix/list read (directory of keys) — apps modeled that way need
  one import per key.
- Watch errors retry silently every 2 s with no log from this module and no staleness signal
  (metric/health) — "config is stale" is unobservable.
- Watch goroutines are never stopped on shutdown; a deleted non-optional key still degrades to a
  stale snapshot (§4.6), but since the WARN addition that degradation is visible in the logs.
- Refresh is all-or-nothing and global: one changed KV entry re-reads every import and every file;
  fine at this scale, worth remembering when imports grow.
