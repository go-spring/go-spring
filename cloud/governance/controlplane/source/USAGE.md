# Control-Plane Rule Sources — Usage Reference (file / http, generic transport in cloud/governance/controlplane/source)

> The transport is implemented ONCE, in `cloud/governance/controlplane/source`, and knows no document
> type. A config root (policy, fault) consumes a `source.Source[T]`; **the root does not ship
> the wiring that creates it.** Which transport a deployment reads its documents from — a local
> file, a config center, a control-plane stream — belongs to whoever owns that deployment's
> configuration, and is registered by a company-level source starter. `governance/example/source.go`
> is a complete reference implementation. This manual lives next to the transport it documents.
>
> | Root | Document keys | Source | Center |
> |---|---|---|---|
> | governance | `spring.governance.*` | `Source[governance.Config]`, provided by a source starter | `governance.Center` |
> | fault | `spring.fault.*` | `Source[fault.Document]`, provided by a source starter | `fault.Center` |
>
> One root = one document and one source bean. A root with no source bean arms an empty document
> and runs ungoverned; two sources for one document type make gs reject the ambiguous injection
> rather than pick one. Do not share one byte stream between two decoders: "this document has no
> keys for my prefix" would then mean either "the other root just changed" or "my section was
> truncated", and `Parse` deliberately treats the latter as an error.
>
Both roots are now transport-free: neither ships a source module. The example's
`source.go` is the reference wiring; `FileSource` above is the piece it uses.

Detailed usage reference. Overview: [README.md](../governance/README.md). All behavior claims are verified against
the transport (`cloud/governance/controlplane/source`: `source.go`, `file.go`, `http.go`, `parse.go`), the policy roots it feeds (`controlplane/{timeoutretry,circuitbreaker,ratelimit,loadbalance,admission}`), and the
runnable [example/](../governance/example/) (`example/main.go` prints the resolved policy and fire status every
second, prompts you to edit the rules files live, and self-terminates after 6s unless `-manual`).

Governance configuration is its own document (a rules file, a console, a config center) — it is
NOT written into `app.properties`; the transport is a `Source[governance.Config]` bean provided by
the deployment's source starter. Fault has its own document and its own `Source[fault.Document]` bean.

Governance semantics themselves (policy resolution, breaker/retry/ratelimit behavior) are
documented in the policy roots (`cloud/governance/controlplane/{timeoutretry,circuitbreaker,ratelimit,loadbalance,admission}`); fault semantics in
`cloud/governance/controlplane/fault` and `cloud/governance/capability/fault`.

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

    "go-spring.org/cloud/governance/capability/fault"
    "go-spring.org/cloud/governance/dataplane/outbound/resilience"
    "go-spring.org/spring/gs"
)   // nothing to blank-import: the file source registers with the policy root it feeds

// printer prints the resolved policy AND the fault-injection config for one
// service label every second — they come from two independent roots and sources.
// Both authorities are nullable beans: a container that never links the policy roots
// leaves them nil, and the printer then reports the pass-through state.
type printer struct {
    Res *resilience.Manager `autowire:"?"`
    Inj *clientfault.Manager     `autowire:"?"`
}

func (p *printer) Run(ctx context.Context) error {
    tk := time.NewTicker(time.Second)
    defer tk.Stop()
    for i := 0; ; i++ {
        var pol resilience.Policy
        if p.Res != nil {
            pol = p.Res.PolicyFor("demo:service")
        }
        fmt.Printf("policy: enabled=%v timeout=%v retries=%d rate-limit=%v",
            !pol.IsZero(), pol.Timeout, pol.MaxRetries, pol.RateLimit)
        if p.Inj != nil {
            c := p.Inj.ClientConfig()
            fmt.Printf(" | fault: armed=%v rules=%d", !c.Inert(), len(c.Rules))
        }
        fmt.Println()
        if i == 3 {
            fmt.Println(">>> edit conf/governance.yaml (or conf/fault.yaml) now: policy and fault toggle live")
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

**example/source.go** — the example's source starter; no keys at all:

```go
func init() {
	gs.Provide(func() (*source.FileSource[governance.Config], error) {
		return source.NewFileSource[governance.Config]("conf/governance.yaml")
	}).
		Name("example.governance.source").
		Destroy((*source.FileSource[governance.Config]).Close).
		Export(gs.As[source.Source[governance.Config]](), gs.As[gs.Runner]()).
		Caller(1)

	gs.Provide(func() (*source.FileSource[faultplane.Document], error) {
		return source.NewFileSource[faultplane.Document]("conf/fault.yaml")
	}).
		Name("example.fault.source").
		Destroy((*source.FileSource[faultplane.Document]).Close).
		Export(gs.As[source.Source[faultplane.Document]](), gs.As[gs.Runner]()).
		Caller(1)
}
```

The registration is written out, not hidden behind a helper: which bean, exported as what, and
with which lifecycle must be readable at the wiring site. The container only asks for `Source[T]`.

The paths are decided by the starter, not by gs: a deployment may read them
from a config center, an env var, or hard-code them. Each root gets one
`Source[T]` bean; a root with none is simply not armed, and one root's absence
never affects the other.

**conf/governance.yaml** — policy only; same keys an app.properties entry would use, watched live:

```yaml
spring:
  governance:
    client:
      resilience:
        timeout: 100ms
        retry:
          max-retries: 2
        services:
          - service: demo:service
            timeout: 50ms
```

**conf/fault.yaml** — the peer root, with its own document and its own `name`-identified experiments:

```yaml
spring:
  fault:
    client:
      rules: ""                 # empty list = inert; do NOT use [] (binds one nameless rule)
      # rules:
      #   - name: demo-errors   # required identity, unique within this direction
      #     service: "demo:service"
      #     rate: 0.5
    server:
      rules: ""
```

**Verify** (from the example directory; the app self-terminates after 6s unless `-manual`):

```bash
go run . -manual
# policy: enabled=true timeout=100ms retries=2 rate-limit=0 | fault: client(armed=false rules=0) server(armed=false rules=0)
# >>> edit conf/governance.yaml or conf/fault.yaml now: policy and fault toggle live
#   ... change timeout to 77ms, or add a named rule to fault.yaml ...
# policy: enabled=true timeout=77ms retries=2 rate-limit=0 | fault: client(armed=true rules=1) server(armed=false rules=0)
```

The change lands within ~1s of saving (fsnotify), with no restart and no app-wide re-bind.

**Variant — another source**: in your source starter, provide a nacos/etcd source from its own
module, a `PushSource` fed by your console's push channel, or any other `Source[T]` instead of the
file one. Exactly one source is active per document type, and there is no app.properties path: rules
always come through a Source.

**Variant — combining with a server starter**: any client starter that injects a module authority
(the `*resilience.Manager` / `*clientfault.Manager` beans this starter registers) picks the policy up
automatically — no application code changes. See starter-echo's USAGE §4.4 for a worked fault drill
through an HTTP server with `match.traffic=loadtest`.

---

## 2. Assembly & timing

### 2.1 The Source contract — what the center actually depends on

`Center` is container-free: it depends only on the two-method `Source` interface
(`cloud/governance/controlplane/source/source.go:43`). The gs wiring lives in
the package's `starter.go`.

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

The source is the bean exported as `source.Source[governance.Config]`; the container injects it into the center
(OPTIONAL — a container without one arms the center with an empty document, i.e. governance off). The
implementations are library pieces, wired by a source starter: `FileSource` (fsnotify) here;
nacos (ListenConfig push) in `starter-governance-nacos`, etcd (Watch push) in
`starter-governance-etcd`; `source.PushSource` for hand-rolled push integrations.
`governance/example/source.go` shows the wiring a company starter writes.

### 2.2 Bean lifecycle timeline

```
link cloud/governance                                  (any client/server starter does it for you)
  ├─ both control-plane roots are ordered before Runtime by runtime/starter.go DependsOn:
  │      cloud/governance/controlplane/{policy roots} → *<root>.Center
  │      cloud/governance/controlplane/fault      → *fault.Center       (faultCenter)
  ├─ the authority packages (always linked by any client starter):
  │      cloud/governance/dataplane/outbound/resilience → *resilience.Manager + the bundled driver
  │      cloud/governance/capability/loadbalance → *loadbalance.Manager
  │      cloud/governance/dataplane/{outbound,inbound}/fault → two independent fault managers
  ├─ the deployment's source starter registers one Source[T] per root it drives
  │      (the example provides the Source beans directly; no key involved)
  └─ both Centers Export(gs.As[gs.Rooter]()): constructed and armed even if nothing injects them

gs.Run()
  ├─ source construction: the source starter's own wiring (the example needs no key)
  ├─ bean wiring: every dependency is an injected field
  │      resilience.Manager  ← every bean exported as resilience.Driver
  │      loadbalance.Manager ← built with no arguments; resolves strategies out of the
  │                             package registry (loadbalance.Register)
  │      source bean: construct (synchronous first read), then Run — fsnotify watch loop begins
  │      each Center ← its own root's authority beans + its source
  │        (OPTIONAL: no source bean ⇒ empty document, that root off)
  ├─ center bean construction (Rooter, before Runtime):
  │      NewCenter applies Snapshot() then subscribes the source, pushing the initial
  │        document into that root's managers —
  │        governance → resilience/admission/loadbalance Apply (a bad document fails construction)
  │        fault      → the two fault managers' Refresh (validated at bind; the source
  │                     adapter, not the center, rejects a bad one)
  ├─ your Runners run (policy already armed — both Centers precede Runtime)
  └─ on SIGTERM: each source adapter bean's Destroy hook → Close()
        (closes the source iff it implements Close; FileSource stops its watcher)
```

Two design points worth knowing (both from source comments, verified):

- **The Rooter export is load-bearing.** Neither center is injected by anything (the runtime
  deliberately holds neither), and gs does not instantiate beans that are neither exported nor
  injected — without `Export(gs.As[gs.Rooter]())` neither root would arm in production. Similarly,
  a custom Source bean of yours is invisible to interface injection unless you export it as
  `gs.As[source.Source[T]]()` for its exact document type — a missing Export leaves that root with
  no source, so it arms an empty document.
- **The Center IS a bean.** each policy root publishes no package-level facade and holds no
  global; its own `starter.go` registers the `*governance.Runtime` over the same
  `*resilience.Manager` / `*loadbalance.Manager` / `*clientfault.Manager` beans it exports to clients,
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
   (`file.go`). The loop reacts to every directory event, not just the file's name.)
2. `FileSource.reload()` re-reads and parses through `source.Parse` — format inferred from the
   extension (`.yaml`), parsed by the shared conf reader registry, flattened, required to carry
   at least one `spring.governance.*` key, then bound into `governance.Config` through the same value-tag
   machinery (`cloud/governance/controlplane/timeoutretry/center.go`). A parse failure or a
   govern-key-less document logs `reload ... failed (keeping last good config)` and stops.
3. Unchanged config (DeepEqual) pushes nothing — a touch does not churn executors.
4. The center's subscribed callback adopts the config (`adopt` → `dispatch`), in TWO phases: first
   `Center.Validate` checks the document (duplicate labels, every balancer name)
   without touching anything, and only if it passes does the commit store the atomic snapshot and
   PUSH each module its own slice — `resilience.Manager.Apply` (a default plus the
   client/server rule lists) and `loadbalance.Manager.Apply` (a default plus the selection rules),
   translated from the document inside `dispatch` — then hot-swap fault in place via
   `injClient.Refresh` + `injServer.Refresh` (`center.go`). Nothing in the commit can fail,
   so a rejected document leaves the snapshot and all three authorities on the last good one. Each
   manager does its own label matching and notifies only subscribers whose policy actually changed.
5. Authority effect: client starters never import governance — they inject the module authority
   they need (`*resilience.Manager`, `*loadbalance.Manager`, `*clientfault.Manager`) and ask it for an
   executor, a binding or an injector; a nil authority (a container that never linked
   the policy roots) means unarmed, i.e. a transparent pass-through. The backing executor is resolved
   lazily, at call time (see `ExecutorFor`); a center push evicts it and the next call rebuilds
   it under the new policy, while the fault injector's config is swapped in place, so
   arming a fault (setting its `rate`) takes effect on the very next call with no restart.

---

## 3. Per-key behavior reference

### 3.1 Source implementation fields

A source starter chooses its own key prefix and binds the fields below relative to it. The old
built-in `spring.governance.source.*` modules are gone: the example's `source.go` constructs a
`FileSource` with a fixed path, and a config-center starter binds its own keys.

| Field | Type | Default | Required | Behavior / interactions | Misconfiguration consequence |
|-------|------|---------|----------|-------------------------|------------------------------|
| `path` (file) | string | — | yes (`expr:"$ != ''"`) | fsnotify on the parent directory; format by extension (json/properties/yaml/toml). Initial load failure fails startup. | Wrong/missing path → startup error (deliberate: a broken source must not arm silently). |

Exactly ONE source bean is active per document type: two beans make gs reject the ambiguous
injection. Remote starters ship as their own modules — nacos (ListenConfig push) and etcd (Watch
push) — and their documents are byte-portable with the file one.

There is no default transport: with no source bean the center arms an empty `PushSource` and the
process runs ungoverned. A source bean that cannot start fails startup in its own adapter. The rules
themselves never live in `app.properties`.

### 3.2 Rules document — top level

| Key | Type | Default | Behavior | Misconfiguration consequence |
|-----|------|---------|----------|------------------------------|
| `spring.resilience.driver` | string | "default" | Resilience backend for ALL services. It is a STATIC app-config key, **not** a document key: it names the contributed driver bean to inject (e.g. "sentinel"), or "default" for the bundled one. | Unknown driver → the named `Driver` bean cannot be injected, so startup fails (a typo must not silently disable protection). |
| `spring.governance.client.resilience.*` | resilience.Policy | all off | Baseline policy for every service no resilience services entry matches. | — |
| `spring.governance.client.resilience.services[n].*` | []ResilienceService | empty | The per-call policy for `service`, optionally narrowed by `match`/`action`; a duplicate of the same (service, action, match) is rejected. ⚠ A matched services entry **fully replaces** the block default — no field-wise merge: a zero policy field means "disabled", so a partial merge could not distinguish "explicitly 0" from "unset". | A services entry setting only `timeout` silently turns OFF the block default's retries for that service. |
| `spring.governance.client.loadbalance.*` | loadbalance.Selection | round_robin | Baseline endpoint selection for every service no services entry matches. Resolved per CONNECTION at wiring time. | — |
| `spring.governance.client.loadbalance.services[n].*` | []LoadBalanceService | empty | Complete endpoint selection for `service` (unique; `balancer` required). No `match`/`action`: a service is balanced one way however its calls are governed. | A services entry without `balancer` is rejected rather than defaulted. |
| `spring.fault.client.*` | fault.Config | all off | OUTBOUND fault injection — the fire that tests this process's own retry/breaker/timeout, see §3.4. | — |
| `spring.governance.server.admission.*` | admission.Policy | all off | Baseline INBOUND admission for every route no AdmissionService matches: rate limit, concurrency cap, handling budget. No circuit breaker. | — |
| `spring.governance.server.services[n].*` | []AdmissionService | empty | Per-route admission services entry, matched by the same exact label the inbound middleware passes (e.g. `gin::8080`, `grpc:/pkg.Svc/Method`). Same full-replace semantics as the client rules. | — |
| `spring.fault.server.*` | fault.Config | all off | INBOUND fault injection — the fire that tests this server's error paths and observe classification, see §3.4. | — |

Service labels live in a **value**, never a key (`config.go`): `redis:cache`,
`gorm:mysql:primary`, `gin:api`, `dubbo:com.example.Foo:1.0.0` — colons and dots need no escaping.

### 3.3 Policy knobs (16, usable under `spring.governance.client.resilience.*` and each `spring.governance.client.resilience.services[n].*`, all default 0/off)

The INBOUND model is `admission.Policy`, under `spring.governance.server.*`. It is deliberately
smaller than the client model: no retry (a handler that already produced side effects cannot
be replayed), no endpoint selection, and **no circuit breaker** — an instance-wide "reject all"
has no safe blast radius (see the outbound [circuit-breaking rule](../../dataplane/outbound/resilience/README.md#circuit-breaking-the-rule)).
Its knobs are:

| Key | Type | Notes |
|-----|------|-------|
| `rate-limit` / `burst` / `rate-limit-max-wait` | float / int / duration | ops/sec cap; burst defaults to a small multiple of rate-limit when unset; max-wait queues over-limit requests up to that long instead of rejecting (0 = reject immediately). |
| `algorithm` / `window` | enum / duration | counting scheme (`token-bucket` when empty, or `sliding-window`) and its rolling interval. |
| `rate-limit-warm-up-duration` / `warm-up-cold-factor` | duration / int | cold-start ramp-in; 0 disables it. |
| `max-concurrent` / `bulkhead-queue-duration` | int / duration | concurrency cap (the bulkhead) and how long a request finding it full may wait for a slot; 0 concurrency disables the stage, 0 queue refuses immediately. |
| `timeout` | duration | the handling budget the adapter applies to the handler's context. |

**Migration (2026-10-24):** the inbound breaker keys (`error-threshold`,
`open-duration`, `breaker-strategy`, `error-rate-threshold`, `min-requests`,
`breaker-window`, `slow-call-*`, `half-open-*`, `degrade-on-open`) no longer
have a field in `admission.Policy` and are no longer bound. Delete them from
pushed documents; the remaining keys keep working. Outbound breaking is
unchanged and still configured under `spring.governance.client.*`.

| Key | Type | Group | Notes |
|-----|------|-------|-------|
| `rate-limit` / `burst` / `rate-limit-max-wait` | float / int / duration | ratelimit | ops/sec cap; burst defaults to a small multiple of rate-limit when unset; max-wait queues over-limit calls up to that long instead of rejecting (0 = reject immediately). |
| `breaker-disabled` | bool | breaker | keep the breaker knobs but switch protection off (an explicit kill switch). Disabled knobs are ignored, not validated. |
| `error-threshold` / `open-duration` | int / duration | breaker | consecutive/error-count trip count and cool-down. `error-threshold` defaults to 5; `open-duration` defaults to 5s. |
| `breaker-strategy` | enum | breaker | `consecutive` (default when unset) \| `error-rate` \| `slow-call` \| `error-count`. Required when error-rate/slow-call knobs are set, or when only general knobs are set. |
| `error-rate-threshold` / `min-requests` / `breaker-window` | float / int / duration | breaker | error-rate trip ratio and window. Defaults: 0.5 (50%), 5 samples, 1s (Sentinel). |
| `slow-call-duration-threshold` / `slow-call-rate-threshold` | duration / float | breaker | a complete pair is required for slow-call; under error-rate it is an additional OR arm alongside the (defaulted) failure rate. |
| `half-open-requests` | int | breaker | trials admitted in half-open before deciding (0/1 = single trial; close after all succeed). |
| `half-open-timeout` | duration | breaker | how long a half-open round may wait for its trial outcomes; 0 (default) waits indefinitely so low traffic keeps probing; positive re-opens the circuit when the round outlives it. |
| `retry.max-retries` / `retry.initial-interval` / `retry.multiplier` / `retry.max-interval` / `retry.randomization-factor` | … | retry | the retry node — default OFF; retry is declared per call and a center can push a node here. |
| `retry.retry-budget` | int | retry | cap on in-flight retries across one executor; over-budget retries get `resilience: retry budget exceeded`. |
| `timeout` | duration | timeout | wall-time bound on the WHOLE call, applied as one context deadline at the entry; retries and backoff share it rather than each getting their own. |

**Breaker enablement:** the breaker is on when at least one breaker knob is
configured, and `breaker-disabled=true` is the explicit way to keep those knobs
while switching protection off. Selecting a strategy applies its mainstream
defaults to a trip condition that was left unset; invalid or ambiguous
configurations are rejected when the document is validated, never silently
turned into a no-op.

Driver selection is a static startup choice (`spring.resilience.driver`, an app-config key, not a governance-document key) and
deliberately NOT re-bindable per service. Semantics of each knob (backoff math, breaker states)
are `cloud/governance/dataplane/outbound/resilience` territory — the starter only binds and fans them out.

### 3.4 Fault experiments (`spring.fault.client.rules[]` and `spring.fault.server.rules[]`)

One rule = one experiment: target + effect + dose/self-heal. **There is no direction-level default** —
no rule matches, nothing fires; to burn everything, write a catch-all (`service` empty) explicitly.
The client block drives `WrapClientExecutor`'s per-attempt gate (outbound), the server block drives
`Server.Fire`'s inbound decision. Each experiment counts its own guardrails.

| Key (all under `rules[n].`) | Type | Default | Behavior |
|-----|------|---------|----------|
| `service` | string | "" | Target/entrance label, exact match; empty = catch-all (matches everything). |
| `action.exact` / `action.prefix` | string | "" | Interface match (`insert`, `GET /users`); prefix covers a family of paths. |
| `match.<key>` | string | empty | Metadata subset match (`match.tenant=acme`); several keys AND together. Traffic class is one dimension: `match.traffic=real` / `loadtest`; omit it for every class. |
| `rate` | float | 0 | Per-call probability of injecting the configured error (0..1). Setting it (or `latency`) is what arms the rule. |
| `latency` / `latency-jitter` | duration | 0 | Injected before each hit and before the error decision, on EVERY matching call regardless of rate; jitter randomizes within ±. |
| `error` | string | "" | `""`/`generic` (retryable ErrInjected) \| `timeout` (wraps DeadlineExceeded) \| `reset` (wraps ECONNRESET) \| `refused` (wraps ECONNREFUSED). Empty + rate 0 injects nothing. |
| `max-duration` | duration | 0 | This experiment's safety auto-off: after this long since ITS first affected call, injection stops — a forgotten fire self-heals. ⚠ Survives config pushes; lengthening does NOT un-trip an already-expired fire. |
| `max-affected` | int64 | 0 | This experiment's blast-radius cap: stop after this many faulted calls. Counted per rule identity, so rules do not consume each other's budget. |

> A direction is inert unless some rule carries a non-zero `rate` or `latency`. A rule's identity is
> the rule's `name`; reordering rules resets its guardrails (equivalent to re-arming).

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
`spring.governance.client.resilience.timeout=1s`); never an emptied file.

### 4.2 Fault drill, no restart

The drill below arms the CLIENT fire (add a rule under `client.fault.rules[]`); the
`server.fault.*` block is the inbound twin and behaves identically against the requests this
process receives.

1. Start with no rule under `client.fault.rules[]` — nothing fires.
2. Add `client.fault.rules[0].service=...` with `.rate: 0.5` and save — `injClient.Refresh`
   swaps in place; the next call is subject to it.
3. The traffic class is per rule: `match.traffic=loadtest` burns only traffic marked `X-Traffic: loadtest`;
   `match.traffic=real` inverts that (dedicated environments only); omitting it hits every class.
4. Add the guards to the same rule before walking away: `max-duration: 5m` (auto-off) and
   `max-affected: 1000` (blast-radius cap).
5. Extinguish by clearing the `rate` (or wait out max-duration).

### 4.3 Probing the authorities from your own code

```go
// mgr, lbMgr, inj and ctr are the injected beans (*resilience.Manager,
// *loadbalance.Manager, *clientfault.Manager, *governance.Runtime); each is nullable.
if mgr.Enabled() { ... }                           // armed? (false before live)
p := mgr.PolicyFor("redis:cache")                  // zero Policy = pass-through
sub := mgr.Subscribe("redis:cache", func(p resilience.Policy) { /* re-arm client */ })
defer sub.Cancel()                                 // MUST cancel if the client is not process-lifetime
lbMgr.Chain(resolver, "redis:cache")  // build a chain registered under the label
c := inj.ClientConfig()                            // read the live outbound fault config
s := inj.ServerConfig()                            // and the inbound one
```

`mgr.ExecutorFor(system, label)` is the call a client makes on the request path; the read-only
helpers above serve a caller that only needs to observe or re-arm something it owns.

### 4.4 Building a center by hand (tests)

There is no package singleton to arm: a test builds its own center over its own authorities —

```go
driver := resilience.NewDefaultDriver(nil)
resClient, resServer := resilience.NewManager(driver), resilience.NewManager(driver)
lb, fc, fs := loadbalance.NewManager(), fault.NewManager(nil), fault.NewManager(nil)
cfg := governance.Config{}
cfg.Client.Default.Timeout = 100 * time.Millisecond
ctr, err := governance.NewCenter(resClient, resServer, lb, fc, fs, disc, source.NewPushSource(cfg))
if err != nil { t.Fatal(err) }
defer ctr.Close()
// assert through resClient.PolicyFor(...) / resClient.ExecutorFor(...)
```

⚠ `gs.RunTest` does NOT wire this center for you: gs clones global bean definitions for test
isolation, so the test app wires a *copy* of the beans while the center you built lives
outside it — the two never meet. The governance package's own tests drive `NewCenter`
directly instead (`cloud/governance/controlplane/timeoutretry/center_test.go`). In app tests, either inject the authority bean your code already takes,
or build a center with `NewCenter` and assert on the authorities you passed it.

### 4.5 Push-based custom source

```go
src := source.NewPushSource(governance.Config{})
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
| Custom Source bean silently ignored | Missing `Export(gs.As[source.Source[governance.Config]]())` | Add the Export — without it the bean is invisible to interface injection. |
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

1. The transport is entirely the source starter's business: a misconfigured starter fails in its
   own wiring, not at a framework-owned key. One place to look, but no framework-side validation.
2. Replace-vs-merge asymmetry between resilience rules (full replace, empty service matches
   nothing) and fault rules (catch-all + global fallback) — must be memorized.
3. A custom Source bean without `Export(gs.As[...])` is invisible to injection, so the center
   quietly arms an empty document and the process runs ungoverned.
5. The example's self-kill smoke script is still not checked in as a CI-runnable `check.sh`.
6. Concepts the doc must define (Center / Source / Snapshot vs Subscribe / authorities) — the
   mental model is real work for a first-time user.
7. ~~Rooter beans may initialize before governance arms, with no ordering guarantee~~ — FIXED:
   the center is a constructor dependency of every governed bean, so dependents wire after it.
8. ~~Fault injection required a restart to toggle~~ — FIXED: `injClient.Refresh` swaps in place
   on every source push (`center.go`, `dispatch`).
