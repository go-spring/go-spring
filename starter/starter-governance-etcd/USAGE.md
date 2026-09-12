# starter-governance-etcd Usage — Reference

[English](USAGE.md) | [中文](USAGE_CN.md)

Detailed usage reference. Overview: [README.md](README.md). Every behavior claim below is verified
against the starter source (`governance.go`, `governance_test.go`), the shared parse subpackage
[`starter-governance/rules`](../starter-governance/rules), the two-method
`governance.Source` contract of [`cloud/governance`](../../cloud/governance) (`source.go`,
`global.go`), and the runnable, self-asserting [example](example) (`example/main.go`,
`example/check.sh`). etcd's own client semantics are [etcd docs](https://etcd.io/docs/latest/) —
everything below is go-spring's increment.

**Activation**: any `govern.source.etcd.*` property arms the conditional module (`gs.OnProperty`
is a **prefix** check, so a single sub-key is enough to fire it), and a single
`governance.Source` bean is registered. A blank import with no such property registers nothing.
The bean alone does not arm governance — the wiring bean of
[`starter-governance`](../starter-governance/README.md) is what injects it into the center; import
that too. Exactly one source is active per process (the center holds a single source), so configure
exactly one of file/http/etcd/nacos.

---

## 1. Complete worked project

A service whose governance rules live in their OWN etcd key and hot-reload live. File tree:

```
demo/
├── go.mod
├── main.go
└── conf/
    └── app.properties
```

**go.mod** (module deps that matter):

```
require (
    go.etcd.io/etcd/client/v3        latest
    go-spring.org/spring             v1.3.x
    go-spring.org/starter-governance     latest
    go-spring.org/starter-governance-etcd latest
)
```

**main.go** — the whole wiring; `conf/app.properties` carries only the two bootstrap keys:

```go
package main

import (
    "context"
    "fmt"
    "time"

    "go-spring.org/cloud/governance"
    "go-spring.org/spring/gs"

    _ "go-spring.org/starter-governance"
    _ "go-spring.org/starter-governance-etcd"
)

type printer struct{}

func (p *printer) Run(ctx context.Context) error {
    // Non-blocking on purpose: a Runner that blocks would hold up app startup.
    go func() {
        for {
            select {
            case <-ctx.Done():
                return
            case <-time.After(time.Second):
            }
            pol := governance.PolicyFor("demo:resource")
            fmt.Printf("enabled=%v timeout=%v retries=%d\n", !pol.IsZero(), pol.Timeout, pol.MaxRetries)
        }
    }()
    return nil
}

func init() { gs.Provide(&printer{}).Export(gs.As[gs.Runner]()) }

func main() { gs.Run() }
```

**conf/app.properties** — the entire bootstrap surface (from `example/conf/app.properties`):

```properties
# The governance rules live in their OWN etcd key, watched by
# starter-governance-etcd — nothing under govern.* rides app.properties.
govern.source.etcd.endpoint=127.0.0.1:2379
govern.source.etcd.key=/app/govern.yaml
```

**The rules document** (seeded to the key before startup):

```yaml
govern:
  enabled: true
  default:
    enabled: true
    attempt-timeout: 100ms
```

**Verify** (with a local etcd, e.g. `example/docker-compose.yml`):

```bash
docker compose up -d
ETCDCTL_API=3 etcdctl put /app/govern.yaml "$(cat govern.yaml)"   # seed BEFORE the app starts
go run .
ETCDCTL_API=3 etcdctl put /app/govern.yaml 'govern: {enabled: true, default: {enabled: true, attempt-timeout: 300ms}}'
# no restart: the source pushes the new document and PolicyFor flips to 300ms
```

The runnable [example](example) is exactly this shape: it seeds the key, publishes a 900ms
document after one second, prints the resolved policy every 200ms, and self-terminates with a
success message once the push is observed — `example/check.sh` wraps it in docker compose and is
skipped gracefully when docker is absent.

---

## 2. Assembly & timing

### 2.1 The Source contract — what the center depends on

`governance.Source` (`cloud/governance/source.go`) is two methods:

```go
type Source interface {
    Snapshot() Config                  // latest committed value; zero Config before any push
    Subscribe(cb func(Config))         // invoked with each new config after it commits
}
```

`EtcdSource` implements both, plus an `Init`/`Close` pair wired to the bean lifecycle. Deliberate
omissions, all inherited from the contract:

- **No error returns** — the center cannot roll back a bad push; keeping the last good snapshot is
  the source's job (§2.3).
- **No `Close` in the interface** — the center type-asserts `interface{ Close() error }` at
  Destroy and closes sources that implement it; `EtcdSource.Close` cancels the watch and closes its
  own client.
- **A single callback** — the center is the only consumer; a second `Subscribe` replaces the first.

### 2.2 Bean lifecycle timeline

```
import starter-governance-etcd
  └─ gs.Module(gs.OnProperty("govern.source.etcd"))       ← prefix match arms the module
        ├─ conf.Bind → governEtcdConfig (${endpoint}/${key} expr-validated non-empty)
        └─ Provide newEtcdSource bean
             .Init((*EtcdSource).Init)                    ← opens the watch stream
             .Destroy((*EtcdSource).Close)                ← cancels watch + closes client
             .Export(gs.As[governance.Source]())
  ├─ newEtcdSource: clientv3.New(Endpoints:[endpoint], Username, Password, DialTimeout:5s)
  │     └─ NewEtcdSource: initial Get (5s ctx) seeds the snapshot
  │           ├─ key missing  → "governance etcd source: key <key> is empty"   (startup error)
  │           └─ bad document → rules.Parse error                              (startup error)
  ├─ import starter-governance: its wiring bean field-injects the Source (autowire "?")
  │     └─ BindDefault(src) subscribes + adopts Snapshot(); GoLive() arms the center
  ├─ source bean Init: the watch goroutine starts
  └─ on SIGTERM: wiring.Destroy() → governance.CloseActiveSource() → EtcdSource.Close()
```

The source builds and owns its etcd client (`clientv3.New` with a 5s dial timeout), separate from
any config-import bootstrap client — see the boundary note in [README.md](README.md).

### 2.3 One rule publish, end to end

1. `etcdctl put <key> <document>` writes a new value.
2. The watch stream delivers the response; the source reacts to `EventTypePut` only
   (`governance.go` `Init`). Responses with no events (compactions, auth refreshes) are ignored.
3. `apply(data)`: a value **byte-identical** to the last delivered document returns immediately —
   a re-put with no change pushes nothing.
4. Otherwise `rules.Parse(key, data, format)` runs — the same parser every governance backend uses,
   so a document that works as a local rules file works unchanged here. A parse failure logs
   `governance etcd source: key <key> got an invalid value (keeping last good config): ...` under
   tag `_app_governance_etcd`, keeps the last good snapshot, and pushes nothing.
5. On a good parse the newly bound `governance.Config` is compared with the current one
   (`reflect.DeepEqual`); if equal it pushes nothing — a touch does not churn executors.
6. If it changed, the snapshot is swapped and the subscribed callback (the center) is invoked; the
   center re-resolves policies and hot-swaps the executor and fault seams.

A **DELETE** on the key is not a PUT, so it is ignored and the last good snapshot stays in effect —
deleting the document is not how governance is turned off. Turning it off is
`govern.enabled=false`, a key that IS present.

### 2.4 Log tag

Runtime logs from this module carry the tag `_app_governance_etcd`
(`log.RegisterAppTag("governance_etcd", "")`). Tune them independently of the main log:

```properties
logger.governance_etcd.type=Logger
logger.governance_etcd.level=WARN
logger.governance_etcd.tag=_app_governance_etcd
```

---

## 3. Per-key behavior reference

All keys live under `govern.source.etcd` (exact match, no relaxed forms). There is no `instances`
dimension — a source is a single object, and the center holds one.

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `endpoint` | string | — (**required**, `expr:"$ != ''"`) | The etcd endpoint watched. One address only — no cluster list. | Empty → bind fails at startup. |
| `key` | string | — (**required**, `expr:"$ != ''"`) | The single KV key holding the rules document. Also the format-inference source when `format` is unset. | Empty → bind fails at startup. |
| `username` | string | `""` | etcd auth username; empty means no auth. | Wrong creds → the initial `Get` fails → startup error. |
| `password` | string | `""` | etcd auth password. | As above. |
| `format` | string | key extension, else `properties` | Overrides document-format detection. `"yaml" \| "json" \| "properties" \| "toml"` (any format registered in the shared reader registry). | Wrong-but-known format → parse error (startup error on seed, keep-last-good after). |

Format inference: an explicit `format` wins; otherwise the key's dotted extension is used
(`/app/govern.yaml` → `yaml`); otherwise it falls back to `properties`. A key with no extension and
a non-properties document therefore needs an explicit `format`.

The document itself is parsed by `rules.Parse`, which requires **at least one `govern.*` key** — a
document that parses but carries none (truncated, or emptied) is an error, not "no governance".
The rules keys (`govern.enabled`, `govern.default.*`, `govern.rules[n].*`, `govern.fault.*`) are
documented in [`starter-governance`](../starter-governance/USAGE.md); they are identical across
backends.

---

## 4. Verification & fault drills

Prereqs: a local etcd (`example/docker-compose.yml`; no auth, so no credentials). `etcdctl` uses
`ETCDCTL_API=3`.

### 4.1 Cold seed

```bash
ETCDCTL_API=3 etcdctl put /app/govern.yaml "$(cat govern.yaml)"
go run .        # boots; the initial Get seeds the snapshot
```

The key must exist before the app starts — the source's construction does an initial `Get`.

### 4.2 Watch push (hot reload)

```bash
go run . &
etcdctl put /app/govern.yaml 'govern: {enabled: true, default: {enabled: true, attempt-timeout: 300ms}}'
```

No restart; `governance.PolicyFor("...")` flips to the new timeout on the next call.

### 4.3 Bad value keeps the last good snapshot

```bash
etcdctl put /app/govern.yaml 'govern: { broken'      # or a document with no govern.* keys
```

Expected: an Error log `governance etcd source: key ... got an invalid value (keeping last good
config): ...` under `_app_governance_etcd`; the resolved policy does not change; nothing is pushed.
A subsequent identical bad value is re-parsed and re-logged, because the "last document" pointer
only advances on a successful parse.

### 4.4 Missing key / bad seed fails startup

| Drill | Expected |
|-------|----------|
| key absent before boot | startup error `governance etcd source: key <key> is empty` |
| key holds an unparseable document | startup error from `rules.Parse` (the document cannot be armed) |
| etcd unreachable | startup error `governance etcd source: get <key> failed` |
| `endpoint`/`key` only partially configured | startup error from the `expr:"$ != ''"` bind (any sub-key fires the module) |

### 4.5 Deleting the key

`etcdctl del /app/govern.yaml` fires a DELETE event, which the source ignores (only PUT is
applied). Governance keeps the last good rules, with no log. To disable governance, publish a
document with `govern.enabled=false`.

### 4.6 Smoke test

```bash
cd example && ./check.sh    # docker-gated: compose up etcd, run the self-asserting example
```

`example/check.sh` brings up etcd, runs the example, and exits non-zero unless the example observes
the pushed rule and shuts down cleanly; it prints a warning and exits 0 when docker or docker
compose is unavailable.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Governance stays off (`enabled=false`) despite the key changing | `starter-governance` not imported → no wiring bean, the Source is never injected | Import `go-spring.org/starter-governance`. |
| Startup fails `key <key> is empty` | Required key not present before boot | Seed the key before starting. |
| Startup fails `get <key> failed` | etcd unreachable, wrong port, or auth required but no `username`/`password` | Check endpoint and credentials. |
| Startup fails on an empty `endpoint`/`key` | `govern.source.etcd.*` partially configured (`OnProperty` fired on any sub-key) | Provide both required keys, or remove the whole prefix. |
| Rules don't change after a `put` | (a) value byte-identical or DeepEqual to current; (b) value unparseable / carries no `govern.*` key (Error log, last good kept); (c) another source already bound the single center slot | Check the `_app_governance_etcd` lines; configure exactly one source. |
| Governance keeps old rules after deleting the key | DELETE is not a PUT and is ignored by design | Re-PUT a document; to disable use `govern.enabled=false`. |
| Parse error on a key with no extension | format inference defaulted to `properties` | Set `govern.source.etcd.format`. |

---

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 5 |
| Required | 2 (`endpoint`, `key`) |
| Quickstart external deps | 1 (etcd; compose file provided) |
| "Watch out" entries | 3 (single endpoint, no TLS keys, silent watch gaps) |

Design suspects (for the audit ledger):

- **Single endpoint, no cluster list, no TLS keys.** `endpoint` is one string and the client is
  built with a 5s dial timeout and no TLS configuration — production etcd clusters commonly need a
  node list and TLS. Candidate increment, mirroring the same gap noted for `starter-config-etcd`.
- **No watch-gap surfacing.** A dropped/compacted watch or a rejected auth refresh produces no log
  or metric; DELETEs are silently ignored. Observability increment candidate.
- **Shared parse semantics** are the reason documents are portable across backends; any change to
  `rules.Parse` (e.g. the "must carry a `govern.*` key" rule) applies to every source at once.
