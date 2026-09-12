# starter-governance-nacos Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). Every behavior claim below is verified
against the starter source (`governance.go`, `governance_test.go`), the shared parse glue
[starter-governance/rules](../../starter-governance/rules/rules.go), the core contract
[cloud/governance](../../../cloud/governance) (`source.go`), and the self-asserting
[example/](example) (`example/example.go`, `example/check.sh`). Nacos's own semantics (dataId,
group, namespace, `ListenConfig`) are [Nacos docs](https://nacos.io/docs/latest/manual/admin/config/)
— everything below is go-spring's increment.

**What this starter is**: the Nacos adapter of the governance rule-source family. It is a
`governance.Source` implementation; presenting the governance center itself is
[starter-governance](../../starter-governance)'s job. Blank-importing this package is inert until a
`govern.source.nacos.*` key is present.

---

## 1. Complete worked project

A service whose governance rules live in their OWN Nacos dataId and hot-reload on every publish.
This is the checked-in example (abridged); the full flow is in [example/example.go](example/example.go).

```
demo/
├── go.mod
├── main.go            (example.go)
└── conf/
    └── app.properties
```

**go.mod** (module deps that matter):

```
require (
    go-spring.org/spring                  v1.3.x
    go-spring.org/starter-governance-nacos latest
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/cloud/governance"
    "go-spring.org/spring/gs"

    _ "go-spring.org/starter-governance-nacos"
)

// poller observes the resolved policy for one label, so a rule push is visible
// without any client wiring.
type poller struct{}

func (p *poller) Run(ctx context.Context) error {
    // ... publish a new document after ~1s, then poll governance.PolicyFor
    //     until the pushed timeout is observed ...
    pol := governance.PolicyFor("demo:resource")
    fmt.Printf("policy: enabled=%v timeout=%v retries=%d\n", !pol.IsZero(), pol.Timeout, pol.MaxRetries)
}

func init() {
    gs.Provide(&poller{}).Export(gs.As[gs.Runner]())
}

func main() { gs.Run() }
```

**conf/app.properties** — the entire wiring:

```properties
# The governance rules live in their OWN dataId, watched by
# starter-governance-nacos — nothing under govern.* rides app.properties. This
# block is the entire wiring: it arms the conditional module, whose Source bean
# is injected onto the governance center.
govern.source.nacos.server=127.0.0.1:8848
govern.source.nacos.data-id=gs-govern-demo.yaml
govern.source.nacos.group=DEFAULT_GROUP
```

**The rules document** (published to the dataId, not stored in `app.properties`):

```yaml
govern:
  enabled: true
  default:
    enabled: true
    attempt-timeout: 100ms
    max-retries: 2
```

**Verify** (with a local Nacos, e.g. [example/docker-compose.yml](example/docker-compose.yml)):

```bash
# seed the dataId over the HTTP open API, then run:
curl -fsS -X POST 'http://127.0.0.1:8848/nacos/v1/cs/configs' \
     -d 'dataId=gs-govern-demo.yaml&group=DEFAULT_GROUP&content=<the document above>'
go run . -manual
# policy: enabled=true timeout=100ms retries=2
#   ... publish attempt-timeout: 900ms to the same dataId ...
# rule push observed: attempt-timeout is now 900ms
```

The runnable [example/](example) does all of this, self-asserts, and exits 0;
[example/check.sh](example/check.sh) wraps it in docker compose.

---

## 2. Assembly & timing

### 2.1 The Source contract — what the center depends on

`cloud/governance` is container-free: its center depends only on the two-method `Source` interface
(`cloud/governance/source.go`):

```go
type Source interface {
    Snapshot() Config                       // latest committed value; zero Config before any push
    Subscribe(cb func(Config))              // invoked with each new config after it commits
}
```

Deliberate omissions (from the interface's doc comment):

- **No error returns** — the center cannot roll back a bad push. "Everything you push, you
  vouch for": keeping the last good snapshot on a bad document is the source's concern (this
  adapter does exactly that).
- **No Close in the interface** — lifecycle belongs to the implementation; the center
  type-asserts `interface{ Close() error }` at Destroy and closes sources that implement it.
- **A single callback** — the center is the only consumer; a second Subscribe may replace the
  first.

`NacosSource` implements `Snapshot`/`Subscribe` and additionally exports `Init`/`Close` for the gs
bean lifecycle, so the center closes it on Destroy.

### 2.2 Bean lifecycle timeline

```
blank-import starter-governance-nacos
  └─ init() governance.go: gs.Module(gs.OnProperty("govern.source.nacos"), ...)
         (OnProperty is a PREFIX check: any govern.source.nacos.* key arms it)
       ├─ conf.Bind(p, &c, "${govern.source.nacos:=}")   bind + expr-validate the keys
       └─ Provide newNacosSource:
            clients.NewConfigClient (namespace, 5s timeout, auth, NotLoadCacheAtStart)
              → NewNacosSource: initial GetConfig + rules.Parse   ← fail fast here
            .Init((*NacosSource).Init).Destroy((*NacosSource).Close)
            .Export(gs.As[governance.Source]())

gs.Run()
  ├─ bean wiring: the exported Source bean is injected into starter-governance's wiring
  ├─ source bean Init: ListenConfig installs the OnChange listener
  ├─ your Runners run (governance already armed — Rooter precedes Runner)
  └─ on SIGTERM: source bean Destroy → Close: CancelListenConfig + CloseClient
```

**Two design points worth knowing:**

- **The Export is load-bearing.** Without `Export(gs.As[governance.Source]())` the bean would be
  invisible to the center's interface injection and governance would stay disabled. The starter
  wires it correctly; a hand-rolled Source bean must export itself the same way.
- **Fail-fast seed.** `NewNacosSource` calls `GetConfig` and `rules.Parse` before the bean exists.
  A missing dataId, a client error, or an unparseable document fails construction — and therefore
  startup — instead of arming a disabled center
  ([`TestNacosSource_BadSeedFailsFast`](governance_test.go) pins this).

### 2.3 One rule publish, end to end

1. A publisher writes the dataId (console, HTTP open API, SDK — any of them).
2. Nacos delivers the new content to the listener installed by `Init` (`ListenConfig` →
   `OnChange` → `NacosSource.apply`).
3. `apply` first compares bytes: a byte-equal re-delivery (Nacos may re-push on reconnect) is a
   no-op. Otherwise it re-parses through `rules.Parse`; a bad document logs with the tag
   `_app_governance_nacos` and keeps the last good snapshot, pushing nothing.
4. A good parse is deduped against the current snapshot with `reflect.DeepEqual`; the snapshot is
   swapped and `cb` fires only when the rules actually changed.
5. The center's subscribed callback adopts the config, re-resolves the policy for every registered
   label, and hot-swaps fault — so a push takes effect on the next call with no restart and no
   app-wide re-bind.

### 2.4 Format resolution

The document format is resolved by `sourceFormat` (mirrored from the etcd sibling): an explicit
`format` key wins, else the dataId's dotted extension, else `properties`. A dataId naming a
supported extension (`app-govern.yaml`, `app-govern.json`, `app-govern.toml`) needs no `format`
key; an extension-less or unknown-extension dataId defaults to `properties`.

`rules.Parse` binds through the same `conf` value-tag machinery with prefix `govern`, and rejects a
document that parses but carries no `govern.*` key (a truncated or emptied document) — turning
governance off is `govern.enabled=false`, a key that IS present.

---

## 3. Per-key behavior reference

All keys live under `govern.source.nacos`, bound with the explicit prefix
`${govern.source.nacos:=}` via `conf.Bind` (exact-match, no relaxed forms). This is the only
starter surface.

| Key | Type | Default | Required | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|----------|-------------------------|------------------------------|
| `server` | string | — | yes (`expr:"$ != ''"`) | Nacos server address in `host:port` form; one server only (no cluster list). Split into host + numeric port. | Not `host:port` → startup error `nacos server address must be host:port`; non-numeric port → `invalid nacos server port ...`. |
| `data-id` | string | — | yes (`expr:"$ != ''"`) | Dedicated rules dataId — do NOT reuse the app-config dataId. Also drives format inference. | Empty → binding error at startup; missing dataId on the server → `get <group>/<dataId> failed` at startup (there is no `optional:` behavior here). |
| `group` | string | `DEFAULT_GROUP` | no | Nacos group of the dataId. | Wrong group → the dataId is not found → startup `get ... failed`. |
| `namespace` | string | `""` (public) | no | Namespace **id**, not name. | Name-instead-of-id → the dataId is not found → startup failure. |
| `username` / `password` | string | `""` | no | Server auth; empty means no auth. | Missing on an auth-enabled server → `get ... failed` at startup. |
| `format` | string | dataId extension, else `properties` | no | `properties` / `yaml` / `toml` / `json`; overrides inference. | Mismatch → parse error: the seed fails startup, a later bad publish keeps last good. |

⚠ There is **no** `timeout` key: the SDK client timeout is a fixed 5000 ms
(`dialTimeoutMs` in `governance.go`), unlike `starter-config-nacos`'s import-string `timeout-ms`.

⚠ `govern.source.*` is the bootstrap surface only. The rules document itself never rides
`app.properties` — it lives in its own dataId, and its keys are the `govern.*` vocabulary
documented in [starter-governance's USAGE](../../starter-governance/USAGE.md).

### 3.1 Byte-portability of the document

The document is parsed by the same `rules.Parse` used by the `starter-governance` file and http
sources: it is flattened, required to carry at least one `govern.*` key, then bound into
`governance.Config`. Consequently a document that works as a local rules file works unchanged as a
Nacos dataId (and vice versa, and as an etcd value with
[starter-governance-etcd](../../starter-governance-etcd)).

---

## 4. Verification & fault drills

### 4.1 Hot push (example app)

```bash
cd example && ./check.sh        # docker-gated: compose up Nacos, run self-asserting example
```

The example seeds the dataId with a 100 ms `attempt-timeout` before startup (so the source's
initial `GetConfig` succeeds), then publishes a 900 ms document; the poller observes
`governance.PolicyFor("demo:resource").Timeout == 900ms` and exits 0.

### 4.2 Bad publish keeps last good (manual)

With the app running, publish a truncated document to the dataId:

```bash
curl -fsS -X POST 'http://127.0.0.1:8848/nacos/v1/cs/configs' \
     -d 'dataId=gs-govern-demo.yaml&group=DEFAULT_GROUP&content=govern: { broken'
# log: governance nacos source: DEFAULT_GROUP/gs-govern-demo.yaml published an
#      invalid document (keeping last good config): ...
# the resolved policy does NOT change
```

[`TestNacosSource_PushChain`](governance_test.go) pins the whole chain: seed → publish parses and
pushes once → a bad publish keeps the last good snapshot and pushes nothing → a byte-equal re-push
is a no-op.

### 4.3 DataId does not exist at startup

Point `data-id` at a name that is not on the server and start: construction's `GetConfig` fails and
startup aborts. There is no `optional:` escape hatch on this surface — create the dataId first (the
example does exactly that).

### 4.4 Smoke test

```bash
cd example && ./check.sh
```

`check.sh` is skipped gracefully when docker or a compose command is unavailable. Otherwise it
brings up `nacos/nacos-server:v2.4.3` in standalone mode, polls
`/nacos/v1/console/health/readiness` for up to 3 minutes (Nacos boot is slow; a raw TCP probe races
the example's first publish), runs the example under a 60 s watchdog, and tears the container down
on exit.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Startup fails `nacos server address must be host:port` | `server` has no `:` or an empty host/port | Use `host:port`, one server. |
| Startup fails `governance nacos source: get <group>/<dataId> failed` | dataId missing, wrong group/namespace, wrong credentials, or server down | Create the dataId under the right group/namespace; check auth. |
| Startup fails inside `rules.Parse` | dataId holds an unparseable or `govern.*`-less document | Publish a valid document first — the seed fails fast by design. |
| Governance configured, `PolicyFor` stays zero | `govern.enabled` false (the default) in the document | Set `govern.enabled=true` — it is the master switch. |
| A publish does not change the policy | logs `published an invalid document (keeping last good config)` | Fix the document; "off" is `govern.enabled=false`, not an empty document. |
| A publish of identical content does nothing | by design — byte-equal re-deliveries and DeepEqual-equal documents push nothing | Expected. |
| Custom Source bean silently ignored | missing `Export(gs.As[governance.Source]())` | Add the Export — without it the bean is invisible to interface injection. |
| Rules change never arrives | Nacos connectivity, or the push target is not the watched `(group, dataId)` | Verify the publish lands on the configured group/dataId. |

---

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 7 (`server`, `data-id`, `group`, `namespace`, `username`, `password`, `format`) |
| Required | 2 (`server`, `data-id`) |
| Quickstart external deps | 1 (Nacos) |
| "Watch out" entries (⚠ above) | 2 |

Design notes (for the audit ledger):

- The initial-seed fail-fast is deliberate: unlike the config-import role, this surface has no
  `optional:` mode, so a misconfigured dataId cannot arm a disabled center silently. The trade-off
  is that the dataId must exist before the app starts — acceptable for a rule document whose
  absence would otherwise be invisible.
- The parse glue (`rules.Parse`) lives as a subpackage of `starter-governance`, not inside
  `cloud/governance`, so the container-free core stays free of the `spring` dependency while every
  backend shares one parser.
- The sibling etcd adapter ([starter-governance-etcd](../../starter-governance-etcd)) has the same
  shape; the two are kept structurally parallel so a reader can move between them.
