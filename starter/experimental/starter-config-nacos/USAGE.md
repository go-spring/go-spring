# starter-config-nacos Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `governance_nacos.go`, `starter_test.go`,
`governance_nacos_test.go`), the core import grammar (`spring/conf/provider/provider.go:74-104`)
and the refresh chain (`spring/gs/internal/gs_app/app.go`), and the docker-gated smoke-verified
[example/](example/). **Nacos's own semantics (dataId / group / namespace, server deployment,
console usage) are the [Nacos docs](https://nacos.io/en/docs/v2/guide/user/config-history/)** —
everything below is go-spring's increment.

> **Experimental marker**: living under `experimental/` means *not yet design-reviewed*, not a
> quality grade. Expect surface churn.

**Activation** (two independent paths, two independent beans):

1. **Config provider** — active whenever a `nacos:` entry appears in `spring.config.import`
   (the blank import registers the provider: `starter.go:58`). No `enabled` key.
2. **Governance rules source** — active when any `govern.source.nacos.*` key exists
   (`gs.OnProperty("govern.source.nacos")` is a prefix check, `governance_nacos.go:64`).

The two beans are deliberately separate: the config-client plumbing (client cache, source
parsing) is shared, but the config provider pushes **application property refresh** while the
governance source pushes **governance rules only** — a rule publish never touches app properties,
and an app-config publish never re-parses rules.

---

## 1. Complete worked project

A service whose configuration lives in Nacos, with one hot-reloadable field, plus governance
rules pushed from a dedicated dataId. File tree (isomorphic to the smoke-tested
[example/](example/)):

```
demo/
├── go.mod
├── main.go
├── conf/
│   └── app.properties
└── docker-compose.yml          # nacos server for local dev
```

**go.mod** (module deps that matter):

```
require (
    github.com/nacos-group/nacos-sdk-go/v2 v2.3.2   // pulled transitively
    go-spring.org/spring                v1.3.x
    go-spring.org/starter-config-nacos  latest
    go-spring.org/starter-governance    latest      // optional: consumes the rules source
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-config-nacos"
    _ "go-spring.org/starter-governance" // optional: governance center
)

// Demo binds a dynamic field sourced from the imported Nacos dataId.
// Only gs.Dync[T] hot-reloads; a plain field keeps its startup value.
type Demo struct {
    Message gs.Dync[string] `value:"${demo.message:=none}"`
}

func init() {
    // Export as gs.Rooter so the container creates it eagerly even though
    // nothing autowires it.
    gs.Provide(&Demo{}).Export(gs.As[gs.Rooter]())
}

func main() { gs.Run() }
```

**conf/app.properties** — the complete surface actually used:

```properties
# Import configuration from the Nacos config server. "optional:" lets the app
# start even when the data id does not exist yet; the value is filled in once
# published and refreshed live via the provider's change listener.
spring.config.import=optional:nacos:127.0.0.1:8848/gs-config-demo?group=DEFAULT_GROUP&format=properties

# Optional second activation: governance rules from a DEDICATED dataId.
# Content is a governance rules document (govern.enabled/fault/resilience...),
# byte-compatible with a starter-governance file source's rules file.
govern.source.nacos.server=127.0.0.1:8848
govern.source.nacos.data-id=demo-govern.yaml
govern.source.nacos.group=DEFAULT_GROUP
```

**docker-compose.yml** (copy of the example's — note the gRPC port `9848`; the v2 SDK talks
gRPC at main-port + 1000, so exposing only 8848 breaks it):

```yaml
services:
  nacos:
    image: nacos/nacos-server:v2.4.3
    environment:
      MODE: "standalone"
    ports:
      - "127.0.0.1:8848:8848"
      - "127.0.0.1:9848:9848"
```

**Verify** (cold load, then hot reload without restart):

```bash
docker compose up -d
# wait for readiness (Nacos boots slowly, ~1-3 min)
curl -fsS http://127.0.0.1:8848/nacos/v1/console/health/readiness

# seed the dataId via the HTTP open API
curl -fsS -X POST 'http://127.0.0.1:8848/nacos/v1/cs/configs' \
     -d 'dataId=gs-config-demo&group=DEFAULT_GROUP&content=demo.message=hello-v1'
# must print: true

go run .          # app starts; logs "loaded nacos config from DEFAULT_GROUP/gs-config-demo keys=1"

# hot-reload: publish a new version; the app's gs.Dync field updates WITHOUT restart
curl -fsS -X POST 'http://127.0.0.1:8848/nacos/v1/cs/configs' \
     -d 'dataId=gs-config-demo&group=DEFAULT_GROUP&content=demo.message=hello-v2'
# tail the app logs / observe the field; the example's automated form is ./example/check.sh
```

The [example](example/example.go) automates exactly this loop (publish + poll up to 15 s +
assert) and `example/check.sh` is the docker-gated smoke gate.

---

## 2. Assembly & timing

### 2.1 When imports resolve — pre-bean, and why

```
blank-import starter-config-nacos
  └─ init(): gs.Provide(nacosController).Export(gs.As[gs.Rooter]())   starter.go:53
             conf.RegisterProvider("nacos", nacosController.Load)      starter.go:58

gs.Run()
  ├─ App.Start(): app.p.Refresh()                                     app.go "Start"
  │    └─ loads conf/app.properties → sees spring.config.import
  │         └─ conf.Load("optional:nacos:...")                        provider.go:74
  │              ├─ cut "optional:" prefix → optional=true            provider.go:85
  │              ├─ cut "nacos:" prefix → provider lookup             provider.go:89
  │              └─ nacosCtrl.Load(optional, source)                  starter.go:202
  │                   ├─ parseSource → configSource                   starter.go:103
  │                   ├─ clientFor (cached per server|ns|user|pass)   starter.go:153
  │                   ├─ registerListener (deduped)                   starter.go:249
  │                   ├─ GetConfig → reader.Read(format) → flatten    starter.go:219-244
  │                   └─ keys merged into the layered property storage
  ├─ IoC wiring: nacosController gets PropertiesRefresher autowired   starter.go:73
  ├─ Runners → Servers → ready
```

Imports resolve **before any bean is wired** because every bean's `value:"${...}"` tag binds
against the *merged* property storage — remote keys must already be present for binding to see
them. Consequence: the provider itself cannot be an injected bean; it is a package-level
controller registered in `init()`, and the `PropertiesRefresher` it needs is only autowired
*after* the load that already consumed its output. That is why `TriggerRefresh` tolerates a nil
refresher (no-op before wiring; pinned by `TestTriggerRefreshNilRefresherIsNoop`).

### 2.2 Watch / hot-reload path (config surface)

```
Nacos push on dataId
  └─ SDK OnChange (installed by registerListener, deduped per client+group+dataId)
       └─ nacosCtrl.TriggerRefresh                                     starter.go:83
            └─ Refresher.RefreshProperties()  (nil-safe no-op before wiring)
                 └─ App.RefreshProperties()                            app.go:247
                      ├─ reloads ALL sources: files, env, cmd args, and re-runs every
                      │   spring.config.import entry (→ nacosCtrl.Load again — this is why
                      │   the listener registration must be deduped; pinned by
                      │   TestListenerRegisteredOncePerSource)
                      ├─ merges by priority, validates atomically (no partial update)
                      └─ propagates: ONLY gs.Dync[T] fields hot-reload
```

There is **no per-key change callback**: a push re-fetches the whole dataId (and every other
source), and each `gs.Dync[T]` re-resolves its `${...}` expression. Plain `value` fields keep
their startup value forever. `OnProperty` conditions are startup-only.

### 2.3 Governance rules-push path (separate surface)

```
any govern.source.nacos.* key present
  └─ gs.Module(OnProperty("govern.source.nacos"))                     governance_nacos.go:64
       ├─ conf.Bind("${govern.source.nacos:=}") → governNacosConfig (expr-validated)
       └─ Provide NacosSource ctor
            ├─ NEW: GetConfig + rules.Parse → seed snapshot (bad doc FAILS startup)  :127-137
            ├─ Init: ListenConfig(OnChange → apply)                    :140-148
            ├─ Destroy: CancelListenConfig                             :152-154
            └─ Export(gs.As[governance.Source]()) — consumed by the governance Center
                 via the Source contract (Snapshot + Subscribe) in cloud/governance

publish on the rules dataId
  └─ NacosSource.apply(data)                                           governance_nacos.go:173
       ├─ byte-equal re-delivery (SDK may re-push on reconnect) → no-op
       ├─ rules.Parse fails → log error, KEEP last good snapshot, push nothing
       ├─ parsed-equal rules → snapshot swapped, no push
       └─ changed → swap snapshot + invoke the Center's Subscribe callback
```

This path never calls `RefreshProperties`: rule pushes refresh governance executors
(timeouts, retries, fault injection, ...) only.

---

## 3. Per-key behavior reference

### 3.1 Import-string grammar (config surface)

Core grammar (provider.go:74-104): `[optional:]<provider>:<path>` — `optional:` prefix first,
then the provider name, then everything after the first `:` is the path. For `nacos` the path is
`<host>:<port>/<dataId>?<query>` (parsed as a `nacos://` URL, starter.go:103-145). One dataId
per import entry; list multiple imports space/comma-separated in `spring.config.import`.

| Part / param | Type | Default | Behavior / interactions | Misconfiguration consequence |
|---|---|---|---|---|
| `optional:` | prefix | absent (required) | Prefix of the **whole** source string, before `nacos:`. Skips the source on fetch error **or empty content** (warn log), letting startup proceed (starter.go:219-235). | Without it, a missing dataId or a down server aborts startup — usually what you want in prod, wrong for boostrap configs. |
| `host:port` | string | — | **Required.** Exactly `host:port`; one server only (no cluster list). Non-numeric port rejected at parse (starter.go:187-197). | `noport` or `1.2.3.4:x` → startup error `nacos server address must be host:port`. ⚠ Nacos cluster users must front a VIP or accept single-server. |
| `dataId` | string | — | **Required.** Path segment after the first `/`; also drives format inference. | Missing (`...8848` with no path) → startup error `missing data id`. |
| `group` | string | `DEFAULT_GROUP` | Query param (starter.go:126-128). Nacos group semantics: official docs. | Wrong group → "config not found"; with `optional:` this degrades to a silent skip — verify with the startup `loaded nacos config` log. |
| `namespace` | string | empty (public) | Namespace **id**, not name. Part of the client cache key, so different namespaces get different clients. | Name-instead-of-id → empty config (same failure mode as wrong group). |
| `username` / `password` | string | empty | Server auth; also part of the client cache key. | Missing on an auth-enabled server → GetConfig error at startup (or skip if optional). |
| `format` | string | dataId extension, else `properties` | One of the reader's formats: `properties` / `yaml` / `toml` / `json`. Extension-less dataIds default to properties (starter.go:129-135). | Content/format mismatch → startup error `parse nacos config ... as <fmt> failed` (pinned by `TestLoadParseErrorPropagates`). |
| `timeout-ms` | uint64 | `5000` | SDK request timeout per source (starter.go:136-143). Non-numeric rejected at parse. | Too low → flaky startup against a slow Nacos. |

⚠ There is **no** `endpoint` query param and no cluster/address-list form — server is always a
single `host:port`.

### 3.2 Property keys (governance surface)

Bound with prefix `${govern.source.nacos}` via `conf.Bind` (these are instance keys under the
prefix — the only value tags in this starter):

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|---|---|---|---|---|
| `govern.source.nacos.server` | string | — | **Required** (`expr:"$ != ''"`). Same `host:port` form as the import string. | Empty → module binding error at startup. |
| `govern.source.nacos.data-id` | string | — | **Required** (`expr:"$ != ''"`). Dedicated rules dataId — do NOT reuse the app-config dataId. | Empty → binding error; wrong id with a bad/missing doc → construction fails fast (unlike the optional config import). |
| `govern.source.nacos.group` | string | `DEFAULT_GROUP` | Rules group, independent of the config import's group. | Not-found → `get ... failed` at startup. |
| `govern.source.nacos.namespace` / `.username` / `.password` | string | empty | Auth / isolation, same semantics as the import-string params. | Same failure modes. |
| `govern.source.nacos.format` | string | dataId ext, else `properties` | Overrides document-format detection (governance_nacos.go:80-86). | Mismatch → rules.Parse fails; seed fails startup, later bad publishes keep last good. |

⚠ The governance surface has **no timeout key** — the SDK timeout is hard-coded to 5000 ms
(governance_nacos.go:78), unlike the import string's `timeout-ms`. Asymmetry recorded in §6.

Full value-tag audit (matches the source exactly):
`value:"${server}"`, `${data-id}`, `${group:=DEFAULT_GROUP}`, `${namespace:=}`,
`${username:=}`, `${password:=}`, `${format:=}` (governance surface), plus the example's
`${demo.message:=none}` (application field, not starter surface).

---

## 4. Verification & fault drills

All drills use the open-API publish from §1 and the log tag `_app_config_nacos`
(tune via `logger.config_nacos.*`).

### 4.1 Cold load

```bash
# before starting: dataId exists with demo.message=hello-v1
go run . 2>&1 | grep config_nacos
# → "loaded nacos config from DEFAULT_GROUP/gs-config-demo keys=1" (Info)
```

### 4.2 Watch push (hot reload)

```bash
curl -fsS -X POST 'http://127.0.0.1:8848/nacos/v1/cs/configs' \
     -d 'dataId=gs-config-demo&group=DEFAULT_GROUP&content=demo.message=hello-DRILL'
# within ~seconds the gs.Dync[string] field observes "hello-DRILL", no restart
```

Only `gs.Dync[T]` fields move. Automated form: `cd example && ./check.sh` (docker-gated,
self-asserting, 60 s watchdog).

### 4.3 Governance rules push

```bash
curl -fsS -X POST 'http://127.0.0.1:8848/nacos/v1/cs/configs' \
     -d 'dataId=demo-govern.yaml&group=DEFAULT_GROUP&content=govern:
enabled: true
default:
  enabled: true
  attempt-timeout: 300ms'
```

The governance center receives the new Config via Subscribe; breaker/retry/fault behavior
changes live. Publish a **broken** document (`govern: {`) → one Error log
`published an invalid document (keeping last good config)`, previous rules keep applying
(pinned by `TestNacosSource_PushChain`). A byte-equal re-push (SDK reconnect) is a no-op.

### 4.4 Malformed dataId / source string

| Drill | Outcome |
|---|---|
| `nacos:onlydata` (no `host:port`) | startup error `missing nacos server address` |
| `nacos:127.0.0.1:8848` (no dataId) | startup error `missing data id` |
| `...?timeout-ms=abc` | startup error `invalid timeout-ms` (parse-time rejection) |
| required import, dataId absent | startup error `get nacos config ... failed` / `is empty` |
| `optional:` + any of the above fetch failures | warn log, app starts without those keys |

### 4.5 Nacos server down

- **At startup, required import** → startup aborts (GetConfig error).
- **At startup, optional import** → warn `optional config get ... failed (skipped)`; app runs
  with defaults; when the server returns, the next publish fires OnChange and back-fills the
  keys via a full refresh.
- **After startup** → the SDK keeps reconnecting; no refresh fires; `gs.Dync` values hold their
  last good values (stale-config risk — see §5). There is no health indicator or metric for the
  listener state: staleness is only visible as absence of `config_nacos` activity.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| Startup: `unsupported provider type nacos` | starter not blank-imported (provider registered in `init()`) | Add `_ "go-spring.org/starter-config-nacos"`. |
| Startup: `missing nacos server address` / `missing data id` | source string lacks `host:port` or the `/dataId` path | Fix `spring.config.import`; grammar is `[optional:]nacos:host:port/dataId?...`. |
| Startup: `create nacos config client ... failed`, port reachable but SDK errors | gRPC port (main+1000, i.e. 9848) not exposed | Expose 8848 **and** 9848 (v2 SDK uses gRPC). |
| Startup: `get nacos config ... failed` with auth-enabled server | missing `username`/`password` query params | Add them; they are also part of the client cache key. |
| App starts but keys are missing, warn `optional config ... (skipped)` | wrong `group`/`namespace` under `optional:` | Correct group / use the namespace **id**; cross-check the Nacos console. |
| Config loads once, never refreshes | listener registration failed (logged) or server restarted and only 8848 was exposed (gRPC re-listen fails) | Check 9848 exposure and SDK logs; field must be `gs.Dync[T]` — plain `value` fields never hot-reload. |
| `parse nacos config ... as yaml failed` | `format` (explicit or inferred from extension) does not match content | Publish matching content or set `format=` explicitly; note extension-less dataIds default to properties. |
| Governance: startup fails on `rules.Parse` | initial rules document invalid — the seed fails fast by design | Fix the document; a *later* bad publish only logs and keeps last good. |
| Values stuck after network blip | SDK re-delivers byte-equal config on reconnect → intentional no-op | Publish an actually-changed document; verify with console history. |

---

## 6. Design health + suspects

| Metric | Value |
|---|---|
| Import-string params | 7 (+2 required parts) |
| Property keys (governance surface) | 7 (2 required) |
| Quickstart external deps | 1 (Nacos server) |
| "Watch out" entries | 6 (gRPC port, namespace-id, Dync-only, no endpoint/cluster, optional-swallows-wrong-group, governance timeout asymmetry) |

Suspects (for the audit ledger):

- **Two parallel Nacos surfaces**: the provider source string (query params) and
  `govern.source.nacos.*` (property keys) share `configSource` parsing but spell everything
  differently, and the governance path hard-codes `timeoutMs: 5000` with no key → candidate
  unification.
- **Single-server address form**: no cluster list / `endpoint` param; every client is built from
  one `host:port` (starter.go:170). Cluster deployments need an external VIP.
- **Refresh granularity**: one changed dataId triggers a full re-load of *all* sources and
  imports — correct but chatty; the listener dedup is what keeps this safe.
- **No listener liveness signal**: no health indicator/metric; a silently dead listener
  surfaces only as stale config.
- **Example conf carried dead keys**: `spring.nacos.ip-addr` / `spring.nacos.port` used to appear
  in `example/conf/app.properties` but nothing in this starter binds them (leftovers from a sibling
  starter's convention) → removed.
- **`optional:` swallows wrong-group as skip**: an optional import with a typo'd group looks
  identical to "not published yet" at startup.
