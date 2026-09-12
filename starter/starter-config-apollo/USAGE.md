# starter-config-apollo Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `starter_test.go`), the gs core
(`spring/conf/provider/provider.go`, `spring/gs/internal/gs_conf/conf.go`,
`spring/gs/internal/gs_app/app.go`) and the runnable [example/](example/) with its
`check.sh` smoke script. **Apollo's own semantics (namespaces, clusters, meta server,
portal workflows, access keys) are the [Apollo documentation](https://www.apolloconfig.com/#/en/design/apollo-introduction)** —
everything below is go-spring's increment.

**Activation**: a blank import registers the `apollo` config provider
(`starter.go:79`). The starter does nothing until an `apollo:` entry appears in
`spring.config.import`; there is no `enabled` key, no property prefix, and no injectable
bean.

---

## 1. Complete worked project

A realistic service that cold-loads a namespace from Apollo, hot-reloads a `gs.Dync`
field on portal publish, and exposes the value over HTTP for verification. It is
isomorphic to the smoke-tested [example/](example/) (mock Apollo service + cold-load
assert), extended with a real Apollo server and an HTTP probe. File tree:

```
demo/
├── go.mod
├── main.go
├── conf/
│   └── app.properties
```

**go.mod** (module deps that matter):

```
require (
    github.com/apolloconfig/agollo/v4 v4.4.0   // pulled in by the starter
    go-spring.org/spring          v1.3.x
    go-spring.org/starter-echo    latest        // any server starter works
    go-spring.org/starter-config-apollo latest
)
```

**main.go**:

```go
package main

import (
    "net/http"

    "github.com/labstack/echo/v4"
    "go-spring.org/spring/gs"
    StarterEcho "go-spring.org/starter-echo"
    _ "go-spring.org/starter-config-apollo" // registers the "apollo" provider
)

// Demo holds a hot-reloadable property sourced from Apollo.
// The value tag is a TOP-LEVEL absolute key: `${demo.message:=none}` binds the
// flattened key `demo.message` wherever it was imported from, with fallback "none".
type Demo struct {
    Message gs.Dync[string] `value:"${demo.message:=none}"`
}

func init() {
    demo := gs.Provide(&Demo{}).Export(gs.As[gs.Rooter]())

    gs.Provide(func() StarterEcho.RouterRegister {
        return func(e *echo.Echo) {
            // Reading .Value() always returns the latest refreshed value.
            e.GET("/message", func(c echo.Context) error {
                return c.JSON(http.StatusOK, map[string]string{
                    "message": demo.Interface().(*Demo).Message.Value(),
                })
            })
        }
    })
}

func main() { gs.Run() }
```

**conf/app.properties** — the complete, commented surface actually used:

```properties
# Import one Apollo namespace. Grammar (see §3): every parameter lives in the
# import string, NOT in a property prefix.
#   optional:                 empty/missing namespace is skipped, not fatal
#   apollo:                   provider name registered by the starter
#   127.0.0.1:8080            Apollo meta/config server address (host:port)
#   /application              exactly ONE namespace per import entry
#   ?appId=demo&cluster=default&secret=...&format=properties
#
# Multiple entries: comma-separated; later entries override earlier ones on key
# collision (same rule as file imports, conf.go:202).
spring.config.import=optional:apollo:127.0.0.1:8080/application?appId=demo&format=properties

# Server for the verification probe (port must be explicit — no defaults).
spring.echo.server.addr=:8002
spring.http.server.enabled=false
```

**Prerequisites** (one external system): an Apollo config service reachable at the
address in the import string, with app `demo`, cluster `default`, namespace
`application` published. Quickest local stack is Apollo's official
[Quick Start docker-compose](https://www.apolloconfig.com/#/en/deployment/quick-start-docker);
for CI or offline work the starter's own example embeds a mock Apollo service
(`example.go:73-92`) that serves exactly the two endpoints agollo needs
(`/services/config` and `/configfiles/json/{appId}/{cluster}/{namespace}`) — no docker.

**Verify (cold load)**:

```bash
go run . &
curl -s :8002/message            # {"message":"hello-from-apollo"} (or your published value)
grep 'loaded apollo namespace' <log stream>   # "loaded apollo namespace application keys=N"
```

**Verify (hot reload)**: in the Apollo portal, change `demo.message` in the
`application` namespace and publish; within agollo's notification long-poll interval
the refresh fires without a restart:

```bash
curl -s :8002/message            # new value, process untouched
```

The example's own smoke gate (same shape as `example/check.sh`):

```bash
cd example && ./check.sh && echo SMOKE-OK   # asserts "Apollo cold-load OK:" in output
```

---

## 2. Assembly & timing

### 2.1 When imports resolve — pre-bean, and why that matters

`spring.config.import` is processed while gs loads application properties, i.e. in
step 2 of `App.Start` (`gs_app/app.go:285-294`), **before** the IoC container is wired:

```
blank-import starter-config-apollo
  └─ init(): conf.RegisterProvider("apollo", apolloCtrl.Load)   starter.go:77-80
gs.Run() → App.Start()
  ├─ 1. mount the gs.RefreshProperties / gs.AppStarted facade targets
  ├─ 2. app.p.Refresh() — load app.properties, expand spring.config.import      gs_conf/conf.go:216-240
  │       └─ conf.Load(source) → prefix-split [optional:]<provider>:<path>      provider.go:84-92
  │             └─ apolloCtrl.Load(optional, path)                               starter.go:191
  │                   ├─ parseSource → url.Parse("apollo://"+path)              starter.go:113-148
  │                   ├─ clientFor → agollo.StartWithConfig (one per tuple)     starter.go:158-186
  │                   ├─ registerListener (BEFORE the fetch — "a later change
  │                   │   is never missed", starter.go:204)                     starter.go:226-241
  │                   └─ GetConfigContent → reader.Read(format) → flatten       starter.go:207-221
  ├─ 3. initLog
  ├─ 4. wire IoC container
  ├─ 5. Runners, 6. Servers → readiness
```

This ordering is load-bearing: because Apollo keys land in the property layers
*before* bean binding, **plain `value:` fields bind Apollo keys as if they were local
file keys** — no special casing anywhere downstream. The cost: a required
(`non-optional`) import that fails (empty namespace, bad format, unreachable server)
aborts startup at step 2, before any bean exists — which is exactly the fail-fast
contract. It also means `spring.config.import` is read **once per Refresh**: a
listener firing re-runs the whole pipeline (see 2.2).

Import nesting: only one level is processed — a `spring.config.import` declared
*inside* an imported source is silently ignored (`gs_conf/conf.go:213-215`). Source
strings support `${...}` placeholder resolution before loading (`conf.go:224`), so
`apollo:${APOLLO_ADDR:=127.0.0.1:8080}/application?appId=demo` is valid and is the
supported way to keep addresses out of the file. Deduplication of identical import
entries happens before load (`conf.go:223`).

### 2.2 The watch / hot-reload path

agollo runs its own config-change notification long-poll (`/notifications/v2`, see
[Apollo's config design](https://www.apolloconfig.com/#/en/design/apollo-design)).
The starter bridges those events into gs's refresh chain:

```
Apollo publish → agollo long-poll fires ChangeEvent / FullChangeEvent
  → apolloListener.OnChange / OnNewestChange                    starter.go:248-254
    → apolloCtrl.TriggerRefresh                                 starter.go:95-99
      → gs.RefreshProperties()                                  gs_app/app.go:149-151
        → App.RefreshProperties: guard "app not started yet", then
          reload ALL sources (files, env, cmd args, every import), merge by
          layer priority, propagate to the container             gs_app/app.go:247-256
            → all gs.Dync[T] fields update atomically
```

Two properties of this path worth knowing:

- **No-op before start.** Events arriving before the app has started hit a
  `gs.RefreshProperties()` that returns an error and are dropped harmlessly
  (pinned by `TestListenerChangeFiresRefresh`, `starter_test.go:133-139`) — the
  initial load already captured the state (`starter.go:93-94`).
- **Whole-app refresh, not per-namespace.** One changed key triggers a reload of
  *every* source, so a Dync field backed by a local file also re-reads. Only
  `gs.Dync[T]` re-binds; plain `value:` fields are startup-only (no per-key callback
  exists in gs). Listener registration is deduplicated per source
  (`TestListenerRegisteredOncePerSource`, `starter_test.go:118-128`), so the repeated
  Loads from refreshes never stack listeners.

### 2.3 One cold load, layer by layer

`optional:apollo:127.0.0.1:8080/application?appId=demo`:

1. `conf.Load` strips `optional:` → `optional=true`; splits on the first `:` →
   provider `apollo`, path `127.0.0.1:8080/application?appId=demo` (`provider.go:84-92`).
2. `parseSource` prepends `apollo://` and `url.Parse`s: host, path-namespace, query
   params; fills defaults cluster=`default`, format from namespace extension else
   `properties` (`starter.go:113-148`).
3. `clientFor` builds a cache key `server|appId|cluster|secret|namespace`; on miss,
   `agollo.StartWithConfig` creates a client with `IsBackupConfig: false` (no local
   cache file) pointed at `http://<server>` (`starter.go:150-186`).
4. The change listener is registered, then `GetConfigContent` fetches the namespace
   content; empty content + optional → warn and skip; empty + required → error
   (`starter.go:207-214`).
5. `reader.Read(format, content)` parses, `flatten.Flatten` flattens nested formats
   into dotted keys, and the map joins the layered storage as an app-level source
   (`starter.go:216-221`, `gs_conf/conf.go:233-237`).

---

## 3. Per-key behavior reference

### 3.1 Import-string grammar

Form (`provider.go:60-72` core grammar, `starter.go:111-148` apollo specifics):

```
[optional:]apollo:<host>[:<port>]/<namespace>?appId=<id>[&cluster=<c>][&secret=<s>][&format=<f>]
```

`spring.config.import` accepts a comma-separated list; imports resolve in order and
later entries override earlier ones on key collision.

| Part | Type | Default | Behavior / interactions | Misconfiguration consequence |
|------|------|---------|-------------------------|------------------------------|
| `optional:` | flag | absent | Empty *or never-synced* namespace logs a warn (`optional apollo namespace %s is empty (skipped)`, `starter.go:210`) and contributes no keys. | Without it, an empty/missing namespace is a startup error — intended fail-fast, surprising if the namespace is created later. |
| `apollo` | name | — | Provider key registered by the starter's `init`. | Typo → `unsupported provider type ...` at startup (`provider.go:96`). |
| host[:port] | string | — | Apollo meta/config server; passed to agollo as `http://<host>`. Placeholders (`${APOLLO_ADDR}`) resolve first. | Missing (source starts with `/`) → `missing apollo server address in ...` (`starter.go:118-120`). Wrong address → agollo fetch failure at startup (required) or silent skip (optional). |
| namespace | string | — | Exactly ONE namespace per import entry. Its file extension drives the default format. | Missing → `missing namespace in ...` (`starter.go:122-124`). Asking for a non-public namespace without `secret` yields empty content → depends on `optional:`. |
| `appId` | string | — | Required, no default. | Missing → `missing appId in ...` (`starter.go:134-136`). |
| `cluster` | string | `default` | Part of the client cache key — same appId+namespace in two clusters are two clients/two imports, no dedup. | Typo silently reads a different (possibly empty) cluster; combined with `optional:` this fails *quietly*. |
| `secret` | string | empty | Access key for protected namespaces. ⚠ Lands in the import string and thus in config files/logs of the loading layer. | Missing/wrong for a protected namespace → empty content → required-import startup error or optional-import skip. |
| `format` | string | namespace extension, else `properties` | Explicit parser override (`properties`/`yaml`/`yml`/`json`/`toml`, whatever `reader.Read` supports). Pinned by `TestParseSourceFormatOverride`. | Wrong format for the actual content → `parse apollo namespace %s as %s failed` (`starter.go:218`); startup error even when `optional:` (a parse failure is never optional). |

Supported formats are those of `spring/conf/reader`; unknown values error inside
`reader.Read`.

### 3.2 Property keys

The starter module itself binds **zero** property keys — there is no `apollo.*`
prefix, no `enabled` switch, no client-pool config; the only `value:` tag in the
module tree is the example's demonstration field:

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `spring.config.import` | list | empty | Declared in `app.properties`/profile files; comma-separated; one level deep only; placeholder-resolvable. Drives the entire starter. | Absent → starter fully inactive. Nested import inside an Apollo namespace silently ignored. |
| `demo.message` (example) | string | `none` | Example-only: a top-level absolute key flattened from the namespace content `demo.message`. Illustrates Dync binding. | — |

Keys imported from a namespace are **top-level absolute keys** — a namespace entry
`demo.message=hi` binds `${demo.message}` directly; there is no instance prefixing,
and (per the no-relaxed-binding rule) the match is exact.

---

## 4. Verification & fault drills

All drills assume the §1 project against a real Apollo (or the example's mock for
drills 4.1/4.2).

### 4.1 Cold load

```bash
go run . &
curl -s :8002/message                          # published value
grep -E 'loaded apollo namespace' <log>        # keys=N — N is the flattened key count
cd example && ./check.sh                       # CI gate: "Apollo cold-load OK:" marker
```

The example is self-contained: it starts the mock Apollo on `127.0.0.1:18080`, imports
`optional:apollo:127.0.0.1:18080/application?appId=demo&format=properties`, and exits
non-zero unless `demo.message` cold-loads as `hello-from-apollo` (`example.go:94-103`).
Note the mock's `/notifications/v2` returning 304 (`example.go:85-86`) — agollo keeps
long-polling, which is what drill 4.2 rides on.

### 4.2 Watch push (hot reload, no restart)

1. With the app running, `curl -s :8002/message` → note the value.
2. In the Apollo portal, change `demo.message`, publish.
3. Within the notification long-poll interval, re-run the curl → new value; the
   process was never restarted. The refresh re-loads *all* sources (§2.2), so a Dync
   field backed by a local file picks up file edits made in the same window too.
4. Counter-check: a plain (non-Dync) `value:` field does NOT move — startup-only
   binding.

### 4.3 Malformed namespace / format

```properties
spring.config.import=apollo:127.0.0.1:8080/app.json?appId=demo
```

Publish non-JSON content in `app.json` → startup fails with
`parse apollo namespace app.json as json failed` (`TestLoadParseErrorPropagates`,
`starter_test.go:109-114`). Note `optional:` does NOT soften a parse error — only an
empty/missing namespace is optional (`starter.go:207-214`).

### 4.4 Optional vs required entries

- Required, namespace missing/empty: startup aborts with `apollo namespace %s is empty`.
- Same source with `optional:`: warn line `optional apollo namespace ... is empty
  (skipped)` and startup proceeds with the field's `:=` default (`none` in the demo) —
  pinned by `TestLoadOptionalSkipsOnMissingNamespace` (`starter_test.go:94-105`).
- Grammar errors (missing appId / host / namespace) fail regardless of `optional:`
  (`TestParseSourceMissingAppID`).

### 4.5 Apollo server down

- **At startup, required import**: agollo client creation/fetch fails → startup error
  (`create apollo client for ... failed` / empty-namespace error). This is the
  fail-fast contract; there is no retry loop in the starter.
- **At runtime**: agollo retries on its own schedule (its logs, not gs's); gs-side
  the symptom is simply *stale config* — Dync values stop moving. There is no health
  indicator, no metric, no gs log line for a lost connection (see §6). When the
  server returns, the next successful poll fires the listener and values catch up.
- `IsBackupConfig: false` (`starter.go:178`) means no local cache file bridges the
  outage across restarts: restart during an outage with a required import fails.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| `unsupported provider type apollo` at startup | starter not blank-imported | Add `_ "go-spring.org/starter-config-apollo"`. |
| `missing appId in ...` / `missing namespace in ...` / `missing apollo server address` | import-string grammar error | Fix the source string; see §3.1 grammar. |
| Startup fails `apollo namespace X is empty` | namespace absent, not yet published, or protected without `secret` | Publish the namespace / add `secret`, or mark the entry `optional:` if late creation is expected. |
| Startup fails `parse apollo namespace X as Y failed` | `format` (or namespace extension) doesn't match content | Fix `format=` or the namespace name; `optional:` does not cover parse errors. |
| Field stays at `:=` default, log shows `optional ... empty (skipped)` | wrong cluster/appId pointing at an empty namespace (typo) with `optional:` | Verify appId/cluster; a quiet skip is the failure mode of a typo under `optional:`. |
| Hot reload doesn't arrive | (a) field is not `gs.Dync[T]`; (b) agollo long-poll not reaching the server; (c) event fired pre-wiring | Only Dync refreshes; check agollo's own logs for poll errors; pre-wiring events are intentionally dropped (§2.2). |
| Wrong values win over local file | import-layer precedence: later imports override earlier; app file vs import layering per `gs_conf/conf.go` | Reorder import entries / rename colliding keys. |
| Restart fails while Apollo is down (required import) | no local backup (`IsBackupConfig: false`) | Restore Apollo, or run `optional:` + local defaults for degraded-boot. |
| Values from second namespace missing | one namespace per import entry | Add another comma-separated import entry. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys (source params + properties) | 5 source params + 1 property (`spring.config.import`) |
| Required | 3 source params (host, namespace, appId) |
| Quickstart external deps | 1 (Apollo; the example's mock removes it) |
| "Watch out" entries | 5 (secret in file, quiet optional skip, parse-never-optional, whole-app refresh, no backup config) |

Design suspects (kept from the previous edition, extended):

- Secret in the import string lands in config files — unlike vault (token-file, env),
  there is no out-of-band credential path; only placeholder indirection
  (`${APOLLO_SECRET}`) is available → candidate env-var fallback.
- No governance `Source` integration (nacos/etcd have one) → feature gap if governance
  rules should live in Apollo.
- No health indicator / metric / dedicated log tag for the agollo connection: a dead
  server at runtime is invisible to gs observability (only stale config).
- One agollo client per (server, appId, cluster, secret, namespace) tuple
  (`starter.go:150-155`) — many namespaces means many long-poll connections; agollo
  itself supports multi-namespace clients → candidate consolidation.
- `optional:` semantics conflate "not synced yet" with "legitimately empty" — a
  namespace published *empty* is indistinguishable from a wrong appId/cluster typo.
- Hot-reload refreshes the entire property set on every namespace change (any key,
  any source) — coarse but simple; fine at low change rates, worth watching.
