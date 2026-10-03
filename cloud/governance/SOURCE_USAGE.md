# Governance Rule Sources — Usage Reference (file / http, built into cloud/governance)

> Both sources below are **part of this package**: linking `cloud/governance` registers them
> (see [starter.go](starter.go)), so there is nothing to import — a `spring.governance.source.*` key is
> enough to arm one. This manual lives here because the code does; it was written while the adapters
> lived in the (now deleted) `starter-governance-file` module.

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified against
the source (`cloud/governance`: `starter.go`, `source_file.go`, `source_http.go`,
`starter_test.go`, `source_file_test.go`, `source_http_test.go`), the core package it wires
(`center.go`, `source.go`, `fault/config.go`,
`resilience/policy.go`, `resilience/manager.go`, `loadbalance/manager.go`), and the runnable
[example/](example/) (`example/main.go` prints the
resolved policy + fault config every second, prompts you to edit the rules file live, and
self-terminates after 6s unless `-manual`).

**What this starter is**: the file and HTTP **source adapters** for the governance core
(`cloud/governance`). Blank-importing it is inert until configured.

The wiring that used to live here — the four governance beans and the step that hands the
center its Source — now lives in `cloud/governance` (`starter.go`), so linking the
contract is what puts the authorities in the container. This starter adds one thing:

1. **Source adapters** (conditional, `source_file.go` / `source_http.go`): when a
   `spring.governance.source.*` key is present, a `governance.Source` bean is collected and handed
   to the center's constructor; rules refresh governance only — never an app-wide property re-bind.

Governance configuration is its own document (a rules file, a console, a config center) — it is
NOT written into `app.properties`; only the one `spring.governance.source.*` bootstrap key is.

Governance semantics themselves (policy resolution, breaker/retry/ratelimit behavior, fault
injection model) are documented in `cloud/governance` — everything below is the starter's source-adapter
increment plus the contract you need to use it.

---

## 1. Complete worked project

A minimal service whose governance rules live in their OWN file and hot-reload live. File tree
(this is the checked-in example, verbatim):

```
demo/
├── go.mod
├── main.go
└── conf/
    ├── app.properties
    └── governance.yaml
```

**go.mod** (module deps that matter):

```
require (
    go-spring.org/spring            v1.3.x
    go-spring.org/cloud             latest   // the sources are part of this package
)
```

**main.go**:

```go
package main

import (
    "context"
    "flag"
    "fmt"
    "time"

    "go-spring.org/cloud/fault"
    "go-spring.org/cloud/resilience"
    "go-spring.org/spring/gs"
)   // nothing to blank-import: the file source registers with cloud/governance

// printer prints the resolved policy AND the fault-injection config for one
// service label every second — resilience and fault ride the same source.
// Both authorities are nullable beans: a container that never links cloud/governance
// leaves them nil, and the printer then reports the pass-through state.
type printer struct {
    Res *resilience.Manager `autowire:"?"`
    Inj *fault.Injector     `autowire:"?"`
}

func (p *printer) Run(ctx context.Context) error {
    tk := time.NewTicker(time.Second)
    defer tk.Stop()
    for i := 0; ; i++ {
        var pol resilience.ClientPolicy
        if p.Res != nil {
            pol = p.Res.PolicyFor("demo:service")
        }
        fmt.Printf("policy: enabled=%v timeout=%v retries=%d rate-limit=%v",
            !pol.IsZero(), pol.Timeout, pol.MaxRetries, pol.RateLimit)
        if p.Inj != nil {
            c := p.Inj.Config()
            fmt.Printf(" | fault: enabled=%v rate=%v", c.Enabled, c.Rate)
        }
        fmt.Println()
        if i == 3 {
            fmt.Println(">>> edit conf/governance.yaml now: policy AND fault toggle live")
        }
        select {
        case <-ctx.Done():
            return nil
        case <-tk.C:
        }
    }
}

func init() {
    gs.Provide(&printer{}).Export(gs.As[gs.Runner]())
}

func main() {
    manual := flag.Bool("manual", false, "run in manual verification mode (stay up until killed)")
    flag.Parse()
    go func() {
        time.Sleep(6 * time.Second) // give the operator time to edit, then exit
        _ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
    }()
    gs.Run()
}
```

**conf/app.properties** — one line is the entire wiring:

```properties
# The governance rules live in their OWN file, watched by cloud/governance's built-in
# file source — nothing under spring.governance.* here. This key arms that conditional
# module, whose Source bean is handed to the governance center.
spring.governance.source.file.path=conf/governance.yaml
```

**conf/governance.yaml** — same keys an app.properties entry would use, watched live:

```yaml
spring:
  governance:
    enabled: true
    client:
      default:
        enabled: true
        attempt-timeout: 100ms
        max-retries: 2
    client:
      fault:
        enabled: false        # flip to true mid-run — see §4.2
        rate: 0.5
    client:
      rules:
        - service: demo:service
          attempt-timeout: 50ms
```

**Verify** (from the example directory; the app self-terminates after 6s unless `-manual`):

```bash
go run . -manual
# policy: enabled=true timeout=100ms retries=2 rate-limit=0 | fault: enabled=false rate=0.5
# >>> edit conf/governance.yaml now: policy AND fault toggle live
#   ... change attempt-timeout to 77ms, or fault.enabled to true ...
# policy: enabled=true timeout=77ms retries=2 rate-limit=0 | fault: enabled=true rate=0.5
```

The change lands within ~1s of saving (fsnotify), with no restart and no app-wide re-bind.

**Variant — another source**: swap `spring.governance.source.file.path` for `spring.governance.source.http.*` (a
console), or a nacos/etcd source key from their own modules. Exactly one source is active per
process, and there is no longer an app.properties path: rules always come through a Source.

**Variant — combining with a server starter**: any client starter that injects a module authority
(the `*resilience.Manager` / `*fault.Injector` beans this starter registers) picks the policy up
automatically — no application code changes. See starter-echo's USAGE §4.4 for a worked fault drill
through an HTTP server with `scope: loadtest`.

---

## 2. Assembly & timing

### 2.1 The Source contract — what the center actually depends on

`cloud/governance` is container-free: its `center` depends only on the two-method `Source`
interface (`cloud/governance/source.go:43`):

```go
type Source interface {
    Snapshot() Config                       // latest committed value; zero Config before any push
    Subscribe(cb func(Config))              // invoked with each new config after it commits
}
```

Deliberate omissions (from the interface's doc comment):

- **No error returns** — the center cannot roll back a bad push. "Everything you push, you vouch
  for": keeping the last good snapshot on a bad document is the source's concern (both adapters
  here do exactly that).
- **No Close in the interface** — lifecycle belongs to the implementation; the center type-asserts
  `interface{ Close() error }` at Destroy and closes sources that happen to implement it.
- **A single callback** — the center is the only consumer; a second Subscribe may replace the
  first.

The source is the bean exported as `governance.Source`; the container collects it and hands it to
the center's constructor (REQUIRED — a container without one fails startup). The
adapter family: `FileSource` (fsnotify), `HTTPSource` (interval poll) in this starter;
nacos (ListenConfig push) in `starter-governance-nacos`, etcd (Watch push) in
`starter-governance-etcd`; `governance.PushSource` for hand-rolled push integrations.

### 2.2 Bean lifecycle timeline

```
link cloud/governance        (any client/server starter does it for you)
  ├─ the three authority packages (always linked by any client starter):
  │      cloud/resilience → *resilience.Manager + the bundled driver
  │      cloud/loadbalance → *loadbalance.Manager
  │      cloud/fault      → *fault.Injector
  ├─ cloud/governance starter.go: gs.Provide(*governance.Center) over those beans,
  │      .Init((*Center).GoLive).Destroy((*Center).Close)
  │      .Export(gs.As[gs.Rooter]())            ← the wall + the fix, see below
  └─ cloud/governance starter.go: two conditional gs.Module beans (the built-in sources)
         ├─ OnProperty("spring.governance.source.file")  → *FileSource, Export(As[governance.Source]())
         └─ OnProperty("spring.governance.source.http")  → *HTTPSource, Export(As[governance.Source]())
              (OnProperty is a PREFIX check: any spring.governance.source.<x>.* key arms it)

gs.Run()
  ├─ config bind: source config from ${spring.governance.source.file|http.*} (expr-validated)
  ├─ bean wiring: everything is a constructor argument
  │      resilience.Manager  ← every bean exported as resilience.Driver
  │      loadbalance.Manager ← every bean exported as loadbalance.Factory
  │      source bean: construct, then Init: Start() — fsnotify / poll loop begins
  │      Center ← the two managers, fault.Injector, discovery.Manager, and the source
  │        (REQUIRED: no source bean ⇒ startup fails on this parameter)
  │        └─ NewCenter binds the source: subscribe (stale-guarded by handle pointer)
  │             + adopt Snapshot()
  ├─ center bean Init hook:
  │      GoLive(): dispatch the snapshot into the three module authorities —
  │        resilience.Manager.Apply, loadbalance.Manager.Apply, fault.Injector.SetConfig
  │        — then markLive() (→ fires every queued OnReady callback)
  ├─ your Runners run (policy already armed — Rooter precedes Runner)
  └─ on SIGTERM: center bean Destroy hook → center.Close()
        (closes the source iff it implements Close; FileSource stops its watcher,
         HTTPSource cancels its poll loop)
```

Two design points worth knowing (both from source comments, verified):

- **The Rooter export is load-bearing.** The center bean is not injected by anything in a process
  with no governable client, and gs
  does not instantiate beans that are neither exported nor injected — without
  `Export(gs.As[gs.Rooter]())` none of the registrations would fire in production (`cloud/governance/starter.go`).
  Similarly, a custom Source bean of yours is invisible to interface injection unless you
  `Export(gs.As[governance.Source]())` it — a missing Export makes startup fail on the center's required Source parameter.
- **The Center IS a bean.** `cloud/governance` publishes no package-level facade and holds no
  global; its own `starter.go` registers the `*governance.Center` over the same
  `*resilience.Manager` / `*loadbalance.Manager` / `*fault.Injector` beans it exports to clients,
  so the wiring drives exactly the authorities a client injects. There is no process-level
  singleton to reach behind the container.

### 2.3 Rooter vs Runner, and why OnReady exists

gs collects and runs `gs.Rooter` beans before `gs.Runner` beans. The center bean is a Rooter, so
governance is armed before your Runners start — the common case needs nothing. A push-based
caller that is *itself* a Rooter (e.g. starter-dubbo's poller) and injects the center is wired
after it, so it already sees a live center. `center.OnReady(cb)` — a method on the injected
`*governance.Center` bean — is the general guard for anything that can still run early: cb queues until the authority goes live, then
fires exactly once; if it is already live, cb fires immediately. The double-checked locking
guarantees each callback runs exactly once whichever side wins the race.

### 2.4 One rules edit, end to end

1. You save `conf/governance.yaml` (any editor; atomic rename is fine — the watcher watches the
   parent *directory*, never the file, precisely because rename swaps the inode
   (`source_file.go:87-94`). The loop reacts to every directory event, not just the file's name.)
2. `FileSource.reload()` re-reads and parses through `governance.Parse` — format inferred from the
   extension (`.yaml`), parsed by the shared conf reader registry, flattened, required to carry
   at least one `spring.governance.*` key, then bound into `governance.Config` through the same value-tag
   machinery (`cloud/governance/rules.go:51-69`). A parse failure or a
   govern-key-less document logs `reload ... failed (keeping last good config)` and stops.
3. Unchanged config (DeepEqual) pushes nothing — a touch does not churn executors.
4. The center's subscribed callback adopts the config (`adopt` → `dispatch`): it stores the atomic
   snapshot, hands resilience and endpoint selection their halves through each manager's `ApplyServer`
   (re-resolving every subscribed label and notifying only subscribers whose policy actually
   changed), and hot-swaps fault in place via `injector.SetConfig(cfg.Fault)` (`center.go`).
5. Authority effect: client starters never import governance — they inject the module authority
   they need (`*resilience.Manager`, `*loadbalance.Manager`, `*fault.Injector`) and ask it for an
   executor, a binding or an injector; a nil authority (a container that never linked
   cloud/governance) means unarmed, i.e. a transparent pass-through. Resolution is lazy at call time: the executor's
   Refresh swaps the live policy, and the fault injector's config is swapped in place, so
   `fault.enabled: true` takes effect on the very next call with no restart.

---

## 3. Per-key behavior reference

### 3.1 Source keys (this starter)

Bound with the explicit prefix `${spring.governance.source.file:=}` / `${spring.governance.source.http:=}` via
`conf.Bind` — these are the only keys under `spring.governance.*` that are wiring, not rules. ⚠ One
namespace, two roles: `spring.governance.*` is the rules; `spring.governance.source.*` is where the rules come from.

| Key | Type | Default | Required | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|----------|-------------------------|------------------------------|
| `spring.governance.source.file.path` | string | — | yes (`expr:"$ != ''"`) | fsnotify on the parent directory; format by extension (json/properties/yaml/toml). Initial load failure fails startup. | Wrong/missing path → startup error (deliberate: a disabled center must not arm silently). |
| `spring.governance.source.http.url` | string | — | yes (`expr:"$ != ''"`) | Polled with GET; document byte-compatible with the file source. | Bad URL / non-200 / bad body → startup error on first fetch; later, keep-last-good + log. |
| `spring.governance.source.http.interval` | duration | 5s | no | Poll period AND the HTTP client timeout (`source_http.go:70`). | Too small → fetches time out against themselves; too large → slow convergence. |
| `spring.governance.source.http.format` | string | inferred from URL extension | no | `"yaml" \| "json" \| "properties" \| "toml"`; overrides inference (use it when the URL has no/useful extension). | Wrong format → parse error → keep-last-good (or startup failure on first fetch). |
| `spring.governance.source.http.headers` | map[string]string | empty | no | Sent with every request, e.g. `headers.authorization=Bearer xxx` for a console. | Missing auth → every fetch 401 → stuck on last good config. |

Exactly ONE source is active per process: configuring both file and http arms two beans but the
center holds one — the bean injection race is unspecified; configure exactly one. Remote adapters
in their own modules: `spring.governance.source.nacos.*` (starter-governance-nacos, ListenConfig push) and
`spring.governance.source.etcd.*` (starter-governance-etcd: `endpoint`, `key`, …, Watch push) — the documents are
byte-portable across all of these backends.

There is no default path: without a `spring.governance.source.*` key the center gets no source and the
container fails to start (a hand-built center may pass nil and arm itself with `SetSource` later). ⚠ `spring.governance.source.*` is bootstrap ONLY — the rules themselves never live in
app.properties.

### 3.2 Rules document — top level

| Key | Type | Default | Behavior | Misconfiguration consequence |
|-----|------|---------|----------|------------------------------|
| `spring.governance.enabled` | bool | false | Master switch. false → the resilience authority's `PolicyFor` always returns a zero Policy (pass-through) regardless of Default/Rules. | Everything configured but still off — the #1 "why doesn't it work" cause. |
| `spring.governance.driver` | string | "default" | Resilience backend for ALL services, resolved against the container's driver directory: a contributed driver bean's name (e.g. "sentinel"), or "default" for the bundled one. | Unknown driver → startup panic listing the available names (a typo must not silently disable protection). |
| `spring.governance.client.default.*` | resilience.ClientPolicy | all off | Baseline policy for every service no ClientRule matches. | — |
| `spring.governance.client.rules[n].*` | []ClientRule | empty | The ClientRule whose Service equals the label wins (labels are unique — a duplicate is rejected). ⚠ A matched ClientRule **fully replaces** Default — no field-wise merge: a zero policy field means "disabled", so a partial merge could not distinguish "explicitly 0" from "unset" (`cloud/governance/center.go:364`). List specific rules first. | ClientRule setting only `attempt-timeout` silently turns OFF the default's retries for that service. |
| `spring.governance.client.fault.*` | fault.Config | all off | OUTBOUND fault injection — the fire that tests this process's own retry/breaker/timeout, see §3.4. | — |
| `spring.governance.server.default.*` | resilience.ServerPolicy | all off | Baseline INBOUND admission for every route no ClientRule matches: rate limit, concurrency cap, inbound breaker, handling budget. | — |
| `spring.governance.server.rules[n].*` | []ServerRule | empty | Per-route admission override, matched by the same exact label the inbound middleware passes (e.g. `gin::8080`, `grpc:/pkg.Svc/Method`). Same full-replace semantics as the client rules. | — |
| `spring.governance.server.fault.*` | fault.Config | all off | INBOUND fault injection — the fire that tests this server's error paths, observe classification and inbound breaker, see §3.4. | — |

Service labels live in a **value**, never a key (`config.go`): `redis:cache`,
`gorm:mysql:primary`, `gin:api`, `dubbo:com.example.Foo:1.0.0` — colons and dots need no escaping.

### 3.3 Policy knobs (16, usable under `spring.governance.client.default.*` and each `spring.governance.client.rules[n].*`, all default 0/off)

The INBOUND model (`resilience.ServerPolicy`, under `spring.governance.server.*`) accepts the same knobs
**minus the retry family** (`max-retries` / `retry-budget` / `initial-interval` / `multiplier` /
`max-interval` / `randomization-factor`) and minus `max-duration`: a handler that already
produced side effects cannot be replayed, so inbound has no retry to express, and with one
attempt there is nothing for `max-duration` to bound (`attempt-timeout` is the whole handling
budget). The other 16 knobs keep their names and meanings, so a value moves between directions
without relearning.

| Key | Type | Group | Notes |
|-----|------|-------|-------|
| `rate-limit` / `burst` / `rate-limit-max-wait` | float / int / duration | ratelimit | ops/sec cap; burst defaults to a small multiple of rate-limit when unset; max-wait queues over-limit calls up to that long instead of rejecting (0 = reject immediately). |
| `error-threshold` / `open-duration` | int / duration | breaker | consecutive-error trip. |
| `breaker-strategy` | enum | breaker | `consecutive` \| `error-rate`. |
| `error-rate-threshold` / `min-requests` / `breaker-window` | float / int / duration | breaker | error-rate strategy inputs. |
| `slow-call-duration-threshold` / `slow-call-rate-threshold` | duration / float | breaker | slow-call breaker: successes at/above the duration count toward the rate-window numerator. |
| `half-open-requests` | int | breaker | trials admitted in half-open before deciding (0/1 = single trial; close after all succeed). |
| `max-concurrent` | int | isolate | concurrent-call cap. |
| `max-conns` | int | resource | connection-pool cap — the resource half of isolation. Read when the client is CONSTRUCTED (a pool is built once), so unlike every other knob a change reaches the next client built, not the running one; 0 leaves the backend default. |
| `max-retries` / `initial-interval` / `multiplier` / `max-interval` / `randomization-factor` | … | retry | exponential backoff family. |
| `retry-budget` | int | retry | cap on in-flight retries across one executor; over-budget retries get `resilience: retry budget exceeded`. |
| `attempt-timeout` / `max-duration` | duration / duration | timeout | per-attempt bound; overall bound (attempt budget = min(Timeout, remaining MaxDuration)). |

Driver selection and the on/off switch are process-wide (`spring.governance.enabled`/`spring.governance.driver`) and
deliberately NOT re-bindable per service. Semantics of each knob (backoff math, breaker states)
are `cloud/resilience` territory — the starter only binds and fans them out.

### 3.4 Fault knobs (`spring.governance.client.fault.*` and `spring.governance.server.fault.*`)

One knobs table, TWO independent blocks: the client block drives `WrapClientExecutor`'s per-attempt
gate (outbound), the server block drives `ApplyServer`'s inbound-handler gate. Each side counts its own
`max-duration` / `max-affected`, so a self-healed outbound fire leaves an inbound one armed.

| Key | Type | Default | Behavior |
|-----|------|---------|----------|
| `enabled` | bool | false | When false nothing is injected even if rate/error are set. |
| `rate` | float | 0 | Per-call probability of injecting the configured error (0..1). |
| `latency` | duration | 0 | Injected before each call and before the error decision, on EVERY call regardless of rate — models a uniformly slow downstream without forcing errors. |
| `error` | string | "" | `""`/`generic` (retryable ErrInjected) \| `timeout` (wraps DeadlineExceeded) \| `reset` (wraps ECONNRESET). Empty + rate 0 injects nothing. |
| `scope` | string | "" | `""` all traffic \| `real` only unmarked \| `loadtest` only traffic carrying the load-test marker (X-LoadTest via the traffic middleware). Unknown value behaves as "". |
| `max-duration` | duration | 0 | Safety auto-off: after this long since the first affected call, injection stops — a forgotten fire self-heals. ⚠ Hot-reload: shortening takes effect promptly; lengthening does NOT un-trip an already-expired fire (re-arm by toggling enabled off→on). |
| `max-affected` | int64 | 0 | Blast-radius cap: stop after this many faulted calls. |
| `rules[n].service` / `.rate` / `.latency` / `.error` | … | empty | Per-service overrides; FIRST matching rule wins. ⚠ Asymmetry vs resilience: a fault rule with an EMPTY service is a catch-all; when no rule matches, the top-level values still apply — adding rules only adds specificity. |

---

## 4. Verification & fault drills

### 4.1 Watch the hot reload (example app)

```bash
cd example && go run . -manual
# baseline:  policy: enabled=true timeout=100ms retries=2 rate-limit=0 | fault: client(enabled=false rate=0.5) server(enabled=false rate=0.5)
sed -i '' 's/attempt-timeout: 100ms/attempt-timeout: 77ms/' conf/governance.yaml   # lands within ~1s
```

Negative drill — bad edit keeps the last good config (watch the log tag `governance`):

```bash
echo "spring.governance: {enabled: true}" > conf/governance.yaml   # truncated: still carries a spring.governance key
printf '' > conf/governance.yaml                              # EMPTY: no spring.governance.* key → reload error
# log: governance file source: reload conf/governance.yaml failed (keeping last good config): ...
```

Turning governance off is `spring.governance.enabled=false` — a key that IS present — never an emptied file.

### 4.2 Fault drill, no restart

The drill below arms the CLIENT fire (flip `client.fault.enabled`); `server.fault.enabled` is the
inbound twin and behaves identically against the requests this process receives.

1. Start with `client.fault.enabled: false` in governance.yaml.
2. Flip it to true and save — `injector.SetConfig` swaps in place; the next call is subject to it.
3. With `scope: loadtest`, only traffic marked `X-LoadTest: 1` burns; `real` inverts that
   (dedicated environments only); empty scope hits everything.
4. Add the guards before walking away: `max-duration: 5m` (auto-off) and
   `max-affected: 1000` (blast-radius cap).
5. Extinguish by flipping `enabled` back to false (or wait out max-duration).

### 4.3 Probing the authorities from your own code

```go
// mgr, lbMgr, inj and ctr are the injected beans (*resilience.Manager,
// *loadbalance.Manager, *fault.Injector, *governance.Center); each is nullable.
if mgr.Enabled() { ... }                           // armed? (false before live)
p := mgr.PolicyFor("redis:cache")                  // zero Policy = pass-through
sub := mgr.Subscribe("redis:cache", func(p resilience.ClientPolicy) { /* re-arm client */ })
defer sub.Cancel()                                 // MUST cancel if the client is not process-lifetime
lbMgr.Bind(pool, "redis:cache")                    // bind a pool's endpoint selection to the label
ctr.OnReady(func() { /* runs once when the authority goes live */ })
c := inj.ClientConfig()                            // read the live outbound fault config
s := inj.ServerConfig()                            // and the inbound one
```

`mgr.ClientExecutorFor(system, label)` is the call a client makes on the request path; the read-only
helpers above serve a caller that only needs to observe or re-arm something it owns.

### 4.4 Building a center by hand (tests)

There is no package singleton to arm: a test builds its own center over its own authorities —

```go
res, lb, inj := resilience.NewManager(), loadbalance.NewManager(), fault.NewInjector(fault.Configs{})
cfg := governance.Config{Enabled: true}
cfg.Client.Default.AttemptTimeout = 100 * time.Millisecond
ctr := governance.NewCenter(cfg, res, lb, inj)
if err := ctr.GoLive(); err != nil { t.Fatal(err) }
defer ctr.Close()
// assert through res.PolicyFor(...) / res.ClientExecutorFor(...)
```

⚠ `gs.RunTest` does NOT wire this center for you: gs clones global bean definitions for test
isolation, so the test app wires a *copy* of the beans while the center you built lives
outside it — the two never meet. The governance package's own tests drive `Center.GoLive()`
directly instead (`cloud/governance/starter_test.go`). In app tests, either inject the authority bean your code already takes,
or build a center with `NewCenter` and assert on the authorities you passed it.

### 4.5 Push-based custom source

```go
src := governance.NewPushSource(governance.Config{})
ctr.SetSource(src)   // any time: replaces whatever source is bound (last write wins)
// on each upstream event:
src.Push(cfg)
```

`SetSource` outranks the injected bean and the default; exactly one source is active at a time;
the center never merges — whole-replace; removing a custom source is not supported (restart).

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Everything configured, nothing governed | `spring.governance.enabled` false (default) | Set `spring.governance.enabled=true` — it is the master switch. |
| Rules file edited, policy never changes | Wrong path; or the file is a symlink target the dir-watch misses (rare) | Check the startup-armed path; check the log tag `governance` for `reload ... failed`. |
| `reload ... failed (keeping last good config)` in logs | Truncated/emptied document (no `spring.governance.*` key) or syntax error | Fix the document; "off" is `spring.governance.enabled=false`, not an empty file. |
| Startup error `governance source: ... contains no spring.governance.* keys` | First load of an empty document | Same as above, at construction time — deliberate. |
| Custom Source bean silently ignored | Missing `Export(gs.As[governance.Source]())` | Add the Export — without it the bean is invisible to interface injection. |
| HTTP source stuck on old rules | Console auth/availability: every poll fails, keep-last-good | Check `headers.*`; check the console returns 200 with a valid document. |
| Per-service rule killed the default's retries | Matched rule fully REPLACES default (no merge) | Restate every knob you want kept in the rule. |
| Fault `max-duration` raised but fire stays out | Lengthening does not un-trip an expired fire | Toggle `fault.enabled` off→on to re-arm. |
| RunTest asserts on governance, sees disabled center | RunTest clones beans; the center you build or configure lives outside the clone | Inject the authority bean your code takes, or build a center with `governance.NewCenter` in the test. |
| A Rooter bean starts before governance is armed | Rooter-vs-Rooter order is unspecified | Wrap the dependent work in the injected center's `OnReady`. |

---

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys (source) | 5 (this starter) + nacos/etcd in their own modules |
| Required | 1 (the chosen source's `path`/`url`) |
| Rules-document keys | 4 top + 16 policy knobs ×2 (default/rules[n]) + 7 fault + 4 fault-rule |
| Quickstart external deps | 0 (file source); a console for http |
| "Watch out" entries (⚠ above) | 7 |

Design suspects (audit ledger; carried over from the previous edition, none newly fixed):

1. `spring.governance.*` is both the rules namespace and the source config lives under `spring.governance.source.*` —
   one namespace, two roles.
2. Replace-vs-merge asymmetry between resilience rules (full replace, empty service matches
   nothing) and fault rules (catch-all + global fallback) — must be memorized.
3. A custom Source bean without `Export(gs.As[...])` makes startup fail on the center's required
   Source parameter.
5. The example's self-kill smoke script is still not checked in as a CI-runnable `check.sh`.
6. Concepts the doc must define (Center / Source / Snapshot vs Subscribe / authorities) — the
   mental model is real work for a first-time user.
7. ~~Rooter beans may initialize before governance arms, with no ordering guarantee~~ — FIXED:
   `Center.OnReady` queues-and-fires exactly once at `GoLive`.
8. ~~Fault injection required a restart to toggle~~ — FIXED: `injector.SetConfig` swaps in place
   on every source push (`center.go`, `dispatch`).
