# Governance Rule Sources — Usage Reference (file / http, built into cloud/governance)

> Both sources below are **part of this package**: linking `cloud/governance` registers them
> (see [starter.go](starter.go)), so there is nothing to import — a `spring.governance.source.*` key is
> enough to arm one. This manual lives here because the code does; it was written while the adapters
> lived in the (now deleted) `starter-governance-file` module.

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified against
the source (`cloud/governance`: `starter.go`, `source_file.go`, `source_http.go`,
`starter_test.go`, `source_file_test.go`, `source_http_test.go`), the core package it wires
(`center.go`, `source.go`, `fault/config.go`,
`resilience/client_policy.go`, `resilience/manager.go`, `loadbalance/manager.go`), and the runnable
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
    Res *resilience.ClientManager `autowire:"?"`
    Inj *fault.ClientManager     `autowire:"?"`
}

func (p *printer) Run(ctx context.Context) error {
    tk := time.NewTicker(time.Second)
    defer tk.Stop()
    for i := 0; ; i++ {
        var pol resilience.ClientPolicy
        if p.Res != nil {
            pol = p.Res.ClientPolicyFor("demo:service")
        }
        fmt.Printf("policy: enabled=%v timeout=%v retries=%d rate-limit=%v",
            !pol.IsZero(), pol.Timeout, pol.MaxRetries, pol.RateLimit)
        if p.Inj != nil {
            c := p.Inj.ClientConfig()
            fmt.Printf(" | fault: armed=%v rules=%d", !c.Inert(), len(c.Rules))
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
    driver: default
    client:
      default:
        timeout: 100ms
        max-retries: 2
      fault:
        # rules:                  # a rule = one experiment; no rule = no fire
        #   - service: "demo:service"   # empty = catch-all
        #     rate: 0.5                 # arm mid-run — see §4.2
      rules:
        - service: demo:service
          timeout: 50ms
```

**Verify** (from the example directory; the app self-terminates after 6s unless `-manual`):

```bash
go run . -manual
# policy: enabled=true timeout=100ms retries=2 rate-limit=0 | fault: client(armed=false rules=0) server(armed=false rules=0)
# >>> edit conf/governance.yaml now: policy AND fault toggle live (e.g. add client.fault.rules[0].rate=0.5)
#   ... change timeout to 77ms, or add a rule to the client fire ...
# policy: enabled=true timeout=77ms retries=2 rate-limit=0 | fault: client(armed=true rules=1) server(armed=false rules=0)
```

The change lands within ~1s of saving (fsnotify), with no restart and no app-wide re-bind.

**Variant — another source**: swap `spring.governance.source.file.path` for `spring.governance.source.http.*` (a
console), or a nacos/etcd source key from their own modules. Exactly one source is active per
process, and there is no longer an app.properties path: rules always come through a Source.

**Variant — combining with a server starter**: any client starter that injects a module authority
(the `*resilience.ClientManager` / `*fault.ClientManager` beans this starter registers) picks the policy up
automatically — no application code changes. See starter-echo's USAGE §4.4 for a worked fault drill
through an HTTP server with `scope: loadtest`.

---

## 2. Assembly & timing

### 2.1 The Source contract — what the center actually depends on

`cloud/governance` is container-free: its `center` depends only on the two-method `Source`
interface (`cloud/governance/source.go:43`):

```go
type Source interface {
    Snapshot() Config                              // latest committed value; zero Config before any push
    Subscribe(cb func(ctx context.Context, cfg Config)) // each new config, with the context of the change that produced it
}
```

The context handed to `cb` is the change's own — whatever the source named it
with (its coordinates, the change's `trace_id`) — so the center logs what it did
about the change against the same identity the source used. A source with
nothing to say about a delivery passes `context.Background()`.

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
  │      cloud/resilience → *resilience.ClientManager + the bundled driver
  │      cloud/loadbalance → *loadbalance.Manager
  │      cloud/fault      → *fault.ClientManager
  ├─ cloud/governance starter.go: gs.Provide(*governance.Center) over those beans,
  │      .Destroy((*Center).Close)
  │      .Export(gs.As[gs.Rooter]())            ← the wall + the fix, see below
  └─ cloud/governance starter.go: two conditional gs.Module beans (the built-in sources)
         ├─ OnProperty("spring.governance.source.file")  → *FileSource, Export(As[governance.Source]())
         └─ OnProperty("spring.governance.source.http")  → *HTTPSource, Export(As[governance.Source]())
              (OnProperty is a PREFIX check: any spring.governance.source.<x>.* key arms it)

gs.Run()
  ├─ config bind: source config from ${spring.governance.source.file|http.*} (expr-validated)
  ├─ bean wiring: everything is a constructor argument
  │      resilience.ClientManager  ← every bean exported as resilience.Driver
  │      loadbalance.Manager ← built with no arguments; resolves strategies out of the
  │                             package registry (loadbalance.Register)
  │      source bean: construct, then Init: Start() — fsnotify / poll loop begins
  │      Center ← the two managers, fault.ClientManager, discovery.Manager, and the source
  │        (REQUIRED: no source bean ⇒ startup fails on this parameter)
  │        └─ NewCenter binds the source: subscribe (stale-guarded by handle pointer)
  │             + adopt Snapshot()
  ├─ center bean construction:
  │      NewCenter dispatches the initial document into the three module
  │        authorities — resilience.ClientManager.Apply, loadbalance.Manager.Apply,
  │        fault.ClientManager.Refresh + fault.ServerManager.Refresh — and fails on a document that cannot serve
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
  `*resilience.ClientManager` / `*loadbalance.Manager` / `*fault.ClientManager` beans it exports to clients,
  so the wiring drives exactly the authorities a client injects. There is no process-level
  singleton to reach behind the container.

### 2.3 Rooter vs Runner

gs collects and runs `gs.Rooter` beans before `gs.Runner` beans. The center bean is a Rooter, so
governance is armed before your Runners start — the common case needs nothing. A push-based
caller that is *itself* a Rooter (e.g. starter-dubbo's poller) and injects the center is wired
after it, so it already sees a live center.

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
4. The center's subscribed callback adopts the config (`adopt` → `dispatch`), in TWO phases: first
   `Center.Validate` checks the document (duplicate labels, every balancer name)
   without touching anything, and only if it passes does the commit store the atomic snapshot and
   PUSH each module its own slice — `resilience.ClientManager.Apply` (a default plus the
   client/server rule lists) and `loadbalance.Manager.Apply` (a default plus the selection rules),
   translated from the document inside `dispatch` — then hot-swap fault in place via
   `injClient.Refresh` + `injServer.Refresh` (`center.go`). Nothing in the commit can fail,
   so a rejected document leaves the snapshot and all three authorities on the last good one. Each
   manager does its own label matching and notifies only subscribers whose policy actually changed.
5. Authority effect: client starters never import governance — they inject the module authority
   they need (`*resilience.ClientManager`, `*loadbalance.Manager`, `*fault.ClientManager`) and ask it for an
   executor, a binding or an injector; a nil authority (a container that never linked
   cloud/governance) means unarmed, i.e. a transparent pass-through. The backing executor is resolved
   lazily, at call time (see `ClientExecutorFor`); a center push evicts it and the next call rebuilds
   it under the new policy, while the fault injector's config is swapped in place, so
   arming a fault (setting its `rate`) takes effect on the very next call with no restart.

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
container fails to start (a hand-built center may pass nil and stay disabled). ⚠ `spring.governance.source.*` is bootstrap ONLY — the rules themselves never live in
app.properties.

### 3.2 Rules document — top level

| Key | Type | Default | Behavior | Misconfiguration consequence |
|-----|------|---------|----------|------------------------------|
| `spring.resilience.driver` | string | "default" | Resilience backend for ALL services. It is a STATIC app-config key, **not** a document key: it names the contributed driver bean to inject (e.g. "sentinel"), or "default" for the bundled one. | Unknown driver → the named `Driver` bean cannot be injected, so startup fails (a typo must not silently disable protection). |
| `spring.governance.client.default.*` | resilience.ClientPolicy | all off | Baseline policy for every service no ClientRule matches. | — |
| `spring.governance.client.rules[n].*` | []ClientRule | empty | The ClientRule whose Service equals the label wins (labels are unique — a duplicate is rejected). ⚠ A matched ClientRule **fully replaces** Default — no field-wise merge: a zero policy field means "disabled", so a partial merge could not distinguish "explicitly 0" from "unset" (`cloud/governance/center.go:364`). List specific rules first. | ClientRule setting only `timeout` silently turns OFF the default's retries for that service. |
| `spring.governance.client.fault.*` | fault.Config | all off | OUTBOUND fault injection — the fire that tests this process's own retry/breaker/timeout, see §3.4. | — |
| `spring.governance.server.default.*` | resilience.ServerPolicy | all off | Baseline INBOUND admission for every route no ClientRule matches: rate limit, concurrency cap, inbound breaker, handling budget. | — |
| `spring.governance.server.rules[n].*` | []ServerRule | empty | Per-route admission override, matched by the same exact label the inbound middleware passes (e.g. `gin::8080`, `grpc:/pkg.Svc/Method`). Same full-replace semantics as the client rules. | — |
| `spring.governance.server.fault.*` | fault.Config | all off | INBOUND fault injection — the fire that tests this server's error paths, observe classification and inbound breaker, see §3.4. | — |

Service labels live in a **value**, never a key (`config.go`): `redis:cache`,
`gorm:mysql:primary`, `gin:api`, `dubbo:com.example.Foo:1.0.0` — colons and dots need no escaping.

### 3.3 Policy knobs (16, usable under `spring.governance.client.default.*` and each `spring.governance.client.rules[n].*`, all default 0/off)

The INBOUND model (`resilience.ServerPolicy`, under `spring.governance.server.*`) accepts the same knobs
**minus the retry family** (`retry.max-retries` / `retry.retry-budget` / `retry.initial-interval` /
`retry.multiplier` / `retry.max-interval` / `retry.randomization-factor`): a handler that already
produced side effects cannot be replayed, so inbound has no retry to express. `timeout` means
exactly what it means outbound — the whole call's wall time — and inbound is simply the case
with one stage to cover. The other knobs keep their names and meanings, so a value moves
between directions without relearning. Inbound additionally carries `max-concurrent` /
`bulkhead-queue-duration` — the concurrency cap, which an outbound client does not have.

| Key | Type | Group | Notes |
|-----|------|-------|-------|
| `rate-limit` / `burst` / `rate-limit-max-wait` | float / int / duration | ratelimit | ops/sec cap; burst defaults to a small multiple of rate-limit when unset; max-wait queues over-limit calls up to that long instead of rejecting (0 = reject immediately). |
| `error-threshold` / `open-duration` | int / duration | breaker | consecutive-error trip. |
| `breaker-strategy` | enum | breaker | `consecutive` \| `error-rate`. |
| `error-rate-threshold` / `min-requests` / `breaker-window` | float / int / duration | breaker | error-rate strategy inputs. |
| `slow-call-duration-threshold` / `slow-call-rate-threshold` | duration / float | breaker | slow-call breaker: successes at/above the duration count toward the rate-window numerator. |
| `half-open-requests` | int | breaker | trials admitted in half-open before deciding (0/1 = single trial; close after all succeed). |
| `retry.max-retries` / `retry.initial-interval` / `retry.multiplier` / `retry.max-interval` / `retry.randomization-factor` | … | retry | the retry node — default OFF; retry is declared per call and a center can push a node here. |
| `retry.retry-budget` | int | retry | cap on in-flight retries across one executor; over-budget retries get `resilience: retry budget exceeded`. |
| `timeout` | duration | timeout | wall-time bound on the WHOLE call, applied as one context deadline at the entry; retries and backoff share it rather than each getting their own. |

Driver selection is a static startup choice (`spring.resilience.driver`, an app-config key, not a governance-document key) and
deliberately NOT re-bindable per service. Semantics of each knob (backoff math, breaker states)
are `cloud/resilience` territory — the starter only binds and fans them out.

### 3.4 Fault experiments (`spring.governance.client.fault.rules[]` and `spring.governance.server.fault.rules[]`)

One rule = one experiment: target + effect + dose/self-heal. **There is no direction-level default** —
no rule matches, nothing fires; to burn everything, write a catch-all (`service` empty) explicitly.
The client block drives `WrapClientExecutor`'s per-attempt gate (outbound), the server block drives
`Server.Fire`'s inbound decision. Each experiment counts its own guardrails.

| Key (all under `rules[n].`) | Type | Default | Behavior |
|-----|------|---------|----------|
| `service` | string | "" | Target/entrance label, exact match; empty = catch-all (matches everything). |
| `action.exact` / `action.prefix` | string | "" | Interface match (`insert`, `GET /users`); prefix covers a family of paths. |
| `match.<key>` | string | empty | Metadata subset match (`match.tenant=acme`); several keys AND together. |
| `scope` | string | "" | `""` all traffic \| `real` only unmarked \| `loadtest` only traffic carrying the load-test marker (X-LoadTest via the traffic middleware). Unknown value behaves as "". |
| `rate` | float | 0 | Per-call probability of injecting the configured error (0..1). Setting it (or `latency`) is what arms the rule. |
| `latency` / `latency-jitter` | duration | 0 | Injected before each hit and before the error decision, on EVERY matching call regardless of rate; jitter randomizes within ±. |
| `error` | string | "" | `""`/`generic` (retryable ErrInjected) \| `timeout` (wraps DeadlineExceeded) \| `reset` (wraps ECONNRESET) \| `refused` (wraps ECONNREFUSED). Empty + rate 0 injects nothing. |
| `max-duration` | duration | 0 | This experiment's safety auto-off: after this long since ITS first affected call, injection stops — a forgotten fire self-heals. ⚠ Survives config pushes; lengthening does NOT un-trip an already-expired fire. |
| `max-affected` | int64 | 0 | This experiment's blast-radius cap: stop after this many faulted calls. Counted per rule identity, so rules do not consume each other's budget. |

> A direction is inert unless some rule carries a non-zero `rate` or `latency`. A rule's identity is
> `service[index]`; reordering rules resets its guardrails (equivalent to re-arming).

---

## 4. Verification & fault drills

### 4.1 Watch the hot reload (example app)

```bash
cd example && go run . -manual
# baseline:  policy: enabled=true timeout=100ms retries=2 rate-limit=0 | fault: client(armed=false rules=0) server(armed=false rules=0)
sed -i '' 's/timeout: 100ms/timeout: 77ms/' conf/governance.yaml   # lands within ~1s
```

Negative drill — bad edit keeps the last good config (watch the log tag `governance`):

```bash
echo "spring.governance: {client: {default: {timeout: 1s}}}" > conf/governance.yaml  # truncated: still carries a spring.governance key
printf '' > conf/governance.yaml                              # EMPTY: no spring.governance.* key → reload error
# log: governance file source: reload conf/governance.yaml failed (keeping last good config): ...
```

Turning governance off is publishing a document that configures nothing — empty `client`/`server`, a
zero `Default`, no rules — yet still carries one `spring.governance.*` key (e.g.
`spring.governance.client.default.timeout=1s`); never an emptied file.

### 4.2 Fault drill, no restart

The drill below arms the CLIENT fire (add a rule under `client.fault.rules[]`); the
`server.fault.*` block is the inbound twin and behaves identically against the requests this
process receives.

1. Start with no rule under `client.fault.rules[]` — nothing fires.
2. Add `client.fault.rules[0].service=...` with `.rate: 0.5` and save — `injClient.Refresh`
   swaps in place; the next call is subject to it.
3. `scope` is per rule: `scope: loadtest` burns only traffic marked `X-LoadTest: 1`; `real`
   inverts that (dedicated environments only); empty hits everything.
4. Add the guards to the same rule before walking away: `max-duration: 5m` (auto-off) and
   `max-affected: 1000` (blast-radius cap).
5. Extinguish by clearing the `rate` (or wait out max-duration).

### 4.3 Probing the authorities from your own code

```go
// mgr, lbMgr, inj and ctr are the injected beans (*resilience.ClientManager,
// *loadbalance.Manager, *fault.ClientManager, *governance.Center); each is nullable.
if mgr.Enabled() { ... }                           // armed? (false before live)
p := mgr.PolicyFor("redis:cache")                  // zero Policy = pass-through
sub := mgr.Subscribe("redis:cache", func(p resilience.ClientPolicy) { /* re-arm client */ })
defer sub.Cancel()                                 // MUST cancel if the client is not process-lifetime
lbMgr.Chain(resolver, "redis:cache")  // build a chain registered under the label
c := inj.ClientConfig()                            // read the live outbound fault config
s := inj.ServerConfig()                            // and the inbound one
```

`mgr.ClientExecutorFor(system, label)` is the call a client makes on the request path; the read-only
helpers above serve a caller that only needs to observe or re-arm something it owns.

### 4.4 Building a center by hand (tests)

There is no package singleton to arm: a test builds its own center over its own authorities —

```go
driver := resilience.NewDefaultDriver(nil)
resClient, resServer := resilience.NewClientManager(driver), resilience.NewServerManager(driver)
lb, fc, fs := loadbalance.NewManager(), fault.NewClientManager(nil), fault.NewServerManager(nil)
cfg := governance.Config{}
cfg.Client.Default.Timeout = 100 * time.Millisecond
ctr, err := governance.NewCenter(resClient, resServer, lb, fc, fs, disc, governance.NewPushSource(cfg))
if err != nil { t.Fatal(err) }
defer ctr.Close()
// assert through resClient.ClientPolicyFor(...) / resClient.ClientExecutorFor(...)
```

⚠ `gs.RunTest` does NOT wire this center for you: gs clones global bean definitions for test
isolation, so the test app wires a *copy* of the beans while the center you built lives
outside it — the two never meet. The governance package's own tests drive `NewCenter`
directly instead (`cloud/governance/starter_test.go`). In app tests, either inject the authority bean your code already takes,
or build a center with `NewCenter` and assert on the authorities you passed it.

### 4.5 Push-based custom source

```go
src := governance.NewPushSource(governance.Config{})
ctr := governance.NewCenter(resClient, resServer, lb, fc, fs, disc, src)  // or export src as a bean
// on each upstream event:
src.Push(ctx, cfg)
```

The source is bound for life at construction (the bean route hands it to the same constructor);
exactly one source serves a center; the center never merges — whole-replace; changing the source
is a restart.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Everything configured, nothing governed | No rule matches the call's label, or the resolved `Default` is all-zero | Give the service a matching rule (or a non-zero `Default`) — there is no master switch. |
| Rules file edited, policy never changes | Wrong path; or the file is a symlink target the dir-watch misses (rare) | Check the startup-armed path; check the log tag `governance` for `reload ... failed`. |
| `reload ... failed (keeping last good config)` in logs | Truncated/emptied document (no `spring.governance.*` key) or syntax error | Fix the document; "off" is a document that configures nothing yet still carries a `spring.governance.*` key, not an empty file. |
| Startup error `governance source: ... contains no spring.governance.* keys` | First load of an empty document | Same as above, at construction time — deliberate. |
| Custom Source bean silently ignored | Missing `Export(gs.As[governance.Source]())` | Add the Export — without it the bean is invisible to interface injection. |
| HTTP source stuck on old rules | Console auth/availability: every poll fails, keep-last-good | Check `headers.*`; check the console returns 200 with a valid document. |
| Per-service rule killed the default's retries | Matched rule fully REPLACES default (no merge) | Restate every knob you want kept in the rule. |
| Fault `max-duration` raised but fire stays out | Lengthening does not un-trip an expired fire | The window is measured from the rule's first affected call and survives config pushes; re-arm by changing the rule identity (e.g. its position) or restarting. |
| RunTest asserts on governance, sees disabled center | RunTest clones beans; the center you build or configure lives outside the clone | Inject the authority bean your code takes, or build a center with `governance.NewCenter` in the test. |
| A Rooter bean starts before governance is armed | Rooter-vs-Rooter order is unspecified | Inject the center as a constructor dependency — gs wires the dependents after it, so they always see a live center. |

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
   the center is a constructor dependency of every governed bean, so dependents wire after it.
8. ~~Fault injection required a restart to toggle~~ — FIXED: `injClient.Refresh` swaps in place
   on every source push (`center.go`, `dispatch`).
