# fault

`fault` is the shared, direction-neutral MECHANISM of the in-process
**fault-injection** family: the targeting vocabulary, the rule core every
direction embeds, the guardrail state model and the injection sequence. The two
direction packages own their CONFIGURATION and their effect:
[outbound/fault](../../dataplane/outbound/fault/) defines `ClientRule`/`ClientConfig`
(an error kind + class) and gates an outbound attempt inside the resilience
executor's retry loop; [inbound/fault](../../dataplane/inbound/fault/) defines
`ServerRule`/`ServerConfig` (the failure the server returns) and decides an
inbound request before the handler runs. A rule cannot express an effect its
direction cannot produce.

The two authorities are independent: each owns its own document and kernel, so
"setting fire" to a running client or server verifies retry, circuit-breaker,
per-attempt timeout and Fallback without one direction touching the other.

**What it is for and what it is NOT**: see [SCOPE.md](SCOPE.md). Read it before
adding an effect or a new injection seam.

## Features

- Two seams: `WrapClientExecutor` (client side — wraps an executor so injected faults
  land *inside* the retry/breaker loop) and `Server.Fire` (server side — decides,
  before the handler runs, whether an inbound call should be set on fire). Both
  validate resilience, not bypass it.
- Centralized, hot-reloadable config. fault has its own control-plane ROOT
  (`controlplane/fault`, document `spring.fault.*`, source
  `Source[controlplane/fault.Document]` provided by the deployment's source starter) — peer to
  governance, sharing only the transport.
  The two direction packages own independent authorities —
  `*clientfault.Manager` and `*serverfault.Manager`, each registered
  as its own bean — and each swaps its own config in place via `Refresh`, so
  toggling fires at runtime with no restart.
- One config per DIRECTION, so a fire on one side cannot touch the other:
  spring.fault.client.* drives outbound calls, spring.fault.server.* drives
  inbound requests, and each EXPERIMENT counts its own MaxDuration/MaxAffected
  guardrails. An outbound fire that self-heals leaves the inbound one armed.
- Rules only, no top-level effect: each direction is a list of rules, and a
  call is fired at only when a rule that NAMES its service matches it. Every
  guardrail counter is per rule, keyed by the rule's identity, so one fire
  self-healing cannot stop another.
- Four injection kinds: `generic` (a retryable injected error), `timeout`
  (`os.ErrDeadlineExceeded` — an I/O read/write timeout, not a context deadline),
  `reset` (`syscall.ECONNRESET`), `refused`
  (`syscall.ECONNREFUSED`); plus a pure latency mode. Inbound rules declare the
  response `status` (400–599) instead. Per-service rules via each direction's
  `ClientConfig`/`ServerConfig`.
- Dose and time: `rate` (probability) or `every` (deterministic every-Nth), a
  `start-after` delay, and `on`/`off` intermittent windows; `max-affected` and
  `max-duration` remain the guardrails. This is what expresses "bad for 30s,
  then recover" — the shape a breaker's half-open probe needs.
  All dose knobs count ATTEMPTS: a retried outbound call is N downstream
  requests, each judged independently (the same per-request model Envoy and
  Istio use). For independent draws the call-level failure rate composes as
  `1-(1-rate)^N`; retries mask part of the injected rate, as they would with a
  real flaky downstream.
- Outbound rules may also declare `status` (400–599): it rides the injected
  error for an adapter whose protocol has a native status to translate. A
  `net/http` caller is different — a 5xx there is a RESPONSE, not an error — so
  it sees a status only once response replacement lands.
- Outbound injected errors implement `resilience.Retryable`, so they
  deterministically drive retry and breaker decisions exactly as the rule's
  `class` declared, regardless of the host's own predicates. There is no
  business class: the outbound chain never sees a business answer.
- The kernel itself imports nothing of cloud's call vocabulary — only the
  shared `capability/matcher` vocabulary; `chain` appears in the direction
  adapters (the client seam's `Executor` and the record the fire is written
  into), not in this package. Traffic classification is not a dependency
  either: the seam that builds an operation records it as one ordinary
  metadata entry (`traffic`), which rules match like any other dimension.

## Install

```sh
go get go-spring.org/cloud
```

## Usage

In a starter, the executor and injector arrive as injected beans (a
*resilience.Manager plus the side's own fault authority — `*clientfault.Manager`
for outbound, `*serverfault.Manager` for inbound — all nullable):

```go
import (
    clientfault "go-spring.org/cloud/governance/dataplane/outbound/fault"
    serverfault "go-spring.org/cloud/governance/dataplane/inbound/fault"
    "go-spring.org/cloud/governance/dataplane/outbound/resilience"
)

// The authorities are beans: outbound/fault registers one *clientfault.Manager and
// inbound/fault one *serverfault.Manager, and a starter takes the one its side needs as a
// nullable constructor parameter (gs.IndexArg(n, gs.TagArg("?"))). nil means no
// governance is in the container, and both entry points below are nil-safe, so
// the wrap is a transparent pass-through — a client is never forced to depend on
// the starter.
//
// client side: mgr.ExecutorFor yields the governed executor with the observe
// layer already applied; the client authority wraps it from the outside. Holding
// the pointer is enough to observe config changes: the center swaps its config
// in place with Manager.Refresh rather than replacing it.
exec := injClient.WrapClientExecutor(mgr.ExecutorFor("redis", service))

// server side: the entry point is created ONCE, when the middleware is built,
// and reads the published kernel from then on — no lock is taken
// per request. The Target names the inbound call: the entrance plus the route being
// invoked, so a rule can fire one endpoint.
srv := injServer.NewServer()
d, err := srv.Fire(ctx, fault.Target{
    Service: "gin",
    Action:  "GET /users/{id}",
})
if err != nil {
    // injected fault, or the delay was cancelled: do not run the handler
    return next(ctx) // (the adapter maps it to its protocol's failure)
}
next(ctx) // `d` (Result) says which rule governed and whether anything fired
```

For a self-contained injector (tests, cloud/experimental/loadtest), build the
side you need with `clientfault.NewManager()` / `serverfault.NewManager()`
and arm it with its own `Refresh`, then pass it the same way. One `Config` type
serves both directions — the kernel is symmetric, so only the values differ — and
the direction is fixed by which authority you hold, never by a parameter a caller
could aim wrong.

Config (centralized in the governance rules document, one block per direction —
see [README 配置指南 §6](../../../../README.md)):

```properties
# document metadata: labels ONE drill execution. Copied into both directions, so
# traces and counters from every replica group under it. REQUIRED on every
# published document — a disarm names its run too.
spring.fault.run-id=drill-2026-10-27

# outbound: WrapExecutor's per-attempt gate (mgr.ExecutorFor → clientfault.Manager.WrapClientExecutor).
# A rule is the experiment's smallest execution unit; there is no global default
# and no catch-all, so nothing fires unless a rule that names the call's service
# matches — and the FIRST match governs the attempt.
spring.fault.client.rules[0].service=redis:cache
spring.fault.client.rules[0].rate=0.5
spring.fault.client.rules[0].error=generic        # optional: "generic" (default) | "timeout" | "reset" | "refused"
spring.fault.client.rules[0].class=transient      # optional: transient | permanent
spring.fault.client.rules[0].latency=50ms         # optional, applied on every attempt
spring.fault.client.rules[0].latency-jitter=20ms  # optional, sleep latency ± U[0,20ms]
spring.fault.client.rules[0].max-affected=100     # optional blast-radius cap (per process)
spring.fault.client.rules[0].max-duration=5m      # optional self-heal window (per process)
spring.fault.client.rules[0].status=503           # optional protocol status (400–599) for adapters that translate
spring.fault.client.rules[0].start-after=10s      # optional: delay the whole rule from its first match
# spring.fault.client.rules[0].every=10           # alternative to rate: fire every 10th matching call
spring.fault.client.rules[0].on=5s                # optional intermittent window (pairs with off)
spring.fault.client.rules[0].off=5s
spring.fault.client.rules[0].match.traffic.exact=loadtest       # optional: "real" | "loadtest" | any custom class

# inbound: Server.Fire's decision (gin / echo / grpc / hertz / trpc / dubbo).
# Every rule names its target service — there is no catch-all.
spring.fault.server.rules[0].service=grpc
spring.fault.server.rules[0].action.exact=/demo.Service/Echo
spring.fault.server.rules[0].rate=0.2
spring.fault.server.rules[0].status=503           # required once rate > 0: 400–599
spring.fault.server.rules[0].body={"error":"injected"}       # optional response body / RPC message
spring.fault.server.rules[0].content-type=application/json   # optional, HTTP only
```

A direction is armed when at least one rule carries a `rate`/`every` or a
`latency`; with no such rule it injects nothing. There is no separate `enabled` key, and —
deliberately — no top-level effect: an operator aiming at one service cannot
accidentally burn the rest of the process, and every fire is visible as a rule
in the config diff.

Read the live values with `clientfault.Manager.Config()` / `serverfault.Manager.Config()`.

See the [Design](#design) section below for the injection-point rationale and boundaries, and
`starter-redigo/example-load` for a runnable load test that toggles fault.

---

# Design

`fault` is the in-process fault-injection companion to
[cloud/governance/dataplane/outbound/resilience](../../dataplane/outbound/resilience). Where resilience *protects* a client against
downstream failures, fault *manufactures* them on demand so the protection
stack can be proven under load. It ships two seams — `WrapClientExecutor` for outbound
calls, `Server.Fire` for inbound requests — plus the load-test binary
`starter-redigo/example-load` that drives the outbound one end to end.

## 1. Responsibilities & Boundaries

- **Does:** wrap a `chain.Executor` so a configurable fraction of
  operations are made to fail (or slow down) before the real executor sees
  them; hot-swap the live config behind an atomic pointer; expose neutral
  `InjectedError` values that are `resilience.Retryable` and surface as familiar
  Go errors (`os.ErrDeadlineExceeded`, `syscall.ECONNRESET`).
- **Refuses:**
  - No gs / spring dependency in the DOMAIN code: the injector is a plain object,
    and `WrapClientExecutor` / `Server.Fire` are container-free functions. The one
    bean is registered in this package's `starter.go`, and the hot-reload is driven
    by the FAULT control plane (`controlplane/fault`), which owns its own document
    and its own source; the governance center never touches fault, and no client
    starter does either.
  - No metrics, tracing or logging of its own. Injected failures flow through
    the host's observe layer (the executor sits inside it), so they are recorded
    exactly like real failures — that is the whole point.
  - No external chaos. Container/network-level faults (kill the Redis container,
    inject packet loss) belong to infra chaos tools on the docker-compose, not
    to this package.

## 2. Key Abstraction / Seam

**One seam: `WrapClientExecutor`.** A `faultExecutor` wraps the operation `fn`
*inside* its `Execute`, then delegates to the inner executor:

```
faultExecutor.Execute(ctx, call, fn) =>
    inner.Execute(ctx, call, func(attempt) {
        d, err := fire(attempt, ...)      // decide + delay; err != nil => do not proceed
        if d.Fired() { rec.SetFault(d) }  // the adapter writes it onto the call record
        if err != nil { return err }
        return fn(attempt)               // the adapter owns the call
    })
```

`fire` takes no callback and records nothing itself: it returns the `Result`
(the rule that governed, whether an error was injected, the latency applied)
plus the "may I proceed?" error, and the client seam writes the decision onto
the call's record. Keeping the write in the adapter is what lets the engine stay
free of call state (and of `chain` entirely), while the observe kit still reads
the record back and tags the span and the access line. The write must sit **inside** the per-attempt closure: a
decision captured after `Execute` returned would be only the last attempt's, and
a call that fired on one attempt and succeeded on another would lose its tag.

Rules are an **ORDERED list**: the FIRST rule whose match conditions hold governs
the attempt and evaluation stops there. A later matching rule never runs, so one
call is never affected by more than one experiment. Order is precedence among
rules on the same service — put the specific ones first. A matched rule that does not
fire (its Rate draw misses, a guardrail stopped it, its blast radius is full)
leaves the call untouched and STILL terminates selection: matching decides
governance, the draw decides whether the fault applies (the same shape
LaunchDarkly's ordered rules use). One rule already carries both latency and
error, which is the Envoy/Istio "delay + abort" composition inside a single fault
config.

The injection point is deliberate: because `fn` is faulted **inside** the inner
executor's retry loop, an injected failure is retried, counted by the breaker,
bounded by the per-attempt timeout and surfaced to Fallback — exactly the path a
real downstream failure takes. Short-circuiting at the `Execute` boundary
instead would bypass all of that and defeat the purpose.

**Stack order in a host starter** (e.g. redigo): `fault( observe( rawExec ) )`.
`Manager.Wrap` applies the observe layer itself, on the still-private
governed executor (that is what lets observe attach its breaker listener at
construction); fault then wraps that from the outside. The injected fault still
reaches the real executor's retry loop, so it is both *handled* by resilience
and *recorded* by observe.

**Outbound failure class.** `InjectedError` implements `Retryable()` and carries
the rule's `class`: `transient` (default) is retried and counted by the breaker,
`permanent` is not retried but still counted. The classification overrides the
host's predicate, so "this must not be retried" becomes testable. There is no
"business" class — the outbound chain never sees a business answer (a real
client adapter folds one to nil with `resilience.Tolerate` before the executor
runs). The typed kinds wrap a real error so
`errors.Is(err, os.ErrDeadlineExceeded)` works and downstream classifiers
(observe's outcome map) label the call like a genuine I/O timeout/reset.

**Inbound response.** A server rule declares `status` (400–599), required once
`rate > 0`, and optionally a `body` + `content-type`. The handler does not run:
the adapters answer with the declared response — HTTP writes the status and
body (content type defaults to plain text), gRPC/trpc/dubbo map the status
FAMILY (4xx → InvalidArgument / validation code, 5xx → Unavailable / system
error) and use the body as the status message. This is a declared response, not
a corrupted real one: 2xx/3xx replacement and post-handler mutation are not
supported yet.

**Naming the call: `Target`.** A rule is matched against a `Target` — the three
coordinates a rule can name: `Service` (what was called), `Action` (which
interface of it) and `Metadata` (the caller's own targeting dimensions). It is
fault's own type, deliberately not `chain.Operation`: that is a client-shaped
*declaration* of a call, carrying the trace/metric/log blocks and the client-edge
kind a rule never reads, and an inbound request has no meaningful one to offer.
Each seam's adapter builds the Target, because only it knows its protocol's
vocabulary:

- client (`WrapClientExecutor`): `Service` is the call's target, falling back to
  the label the executor was built for; `Action` is `chain.Request.Action`.
- server (`Server.Fire`): `Service` is the entrance ("gin", "grpc"), and `Action`
  is the route or method being invoked, so **an inbound rule can target one
  interface** — `GET /users/{id}`, `/pkg.Svc/Method` — instead of a whole
  entrance. Adapters also expose request metadata if an operator wants to match
  on it.

A rule names what it targets with `MetadataMatch` (a plain `map[string]string`,
value-equality subset-match) and `ActionMatch` (an
`ActionMatch`: exact or prefix). The field names say which axis they match,
because they are conditions on different ones.

Per-replica targeting is a DELIVERY decision, not a rule field: the platform sends
the document only to the selected instances (its console/config-center channel
knows the targets), so the same document fires on every process that receives it.

The matchers stay deliberately shallow. A router needs rich matching — forwarding
to the wrong backend is an incident — while a fire needs matching an operator can
predict, because the first question in a drill is "what exactly did we hit?".
The mainstream chaos tools agree: Chaos Mesh matches method + path (wildcards) +
a header map; ChaosBlade exposes regex only as a separate opt-in flag. The rich
match grammars (Envoy `StringMatcher`, Istio `StringMatch`, Gateway API, Traefik,
APISIX `vars`) belong to traffic management. The axis a fire needs beyond this is
the DOSE — `Rate` / `MaxAffected` / `MaxDuration` — not a richer
predicate. `ActionMatch` lives in the shared
`capability/matcher` vocabulary alongside `StringMatch`, so policy rules and
fault rules answer the "which action" question with one spelling.

EXTENSION: a new operator is a new field under the same one-of convention
(`ActionMatch`), never a reinterpretation of an existing one; richer metadata
matching arrives as a sibling `MetadataMatchers []MetadataMatcher` that ANDs with
the map. Either way `match.tenant=acme` keeps its meaning forever.

**State: stable manager, replaceable core — published, not queried.** The manager
follows the same model as `resilience.Manager` and `loadbalance.Manager`: it is
the stable object, and the compiled kernel is what a refresh replaces. Each
direction's manager owns ONE atomic pointer to its current kernel; `Manager.Fire`
loads it per attempt, so the call path takes no lock and never asks the manager
anything. A refresh is unsynchronized (pushes arrive serialized from the one
source loop) and simply publishes the new kernel. A refresh reconciles rather
than replaces — a rule's identity is
`(run id, name)`: same run and same name keeps its counters (so a controller may
re-push the same document every tick), while a new run id starts every rule from
zero (a spent blast-radius budget must not disable the next drill). Rules that
are gone are dropped with the old core. Presence in the pushed document is
therefore a rule's whole lifecycle: appearing starts it, disappearing ends and
releases it, and `max-duration` / `max-affected` end it
automatically. There is deliberately no separate start/stop call — the document
is the control plane.

**Two directions, two Managers.** `clientfault.Manager` and `serverfault.Manager` are
independent authorities, each holding its own document and published kernel, so
an outbound fire and an inbound fire never see or trip each other: a `Client`
reads the outbound authority's kernel, a `Server` the inbound one. The two types share the neutral kernel in `manager.go` — rule
semantics, injection sequence, guardrail model — but no authority object and no
creation API, which keeps each side free to evolve its own seam. That is why the two configs
must be separate keys (`spring.fault.client.*` /
`spring.fault.server.*`) rather than one block with a per-rule
direction flag — the direction is a property of WHERE the fire is evaluated, and
the same rule can be meaningful on either side, so nothing inside a rule can
declare it.

**Hot-reload.** Each side swaps its whole kernel behind one `atomic.Pointer`;
the fault center calls each authority's `Refresh` on every source push, so
faults toggle at runtime without a restart. The document was already validated
at bind — every field's `expr` tag runs when it enters, so an unnamed rule or
an out-of-range dose can never be armed; a document the source rejects leaves
the previous one serving. Counter aggregation across replicas is the platform's
job, reading each process's `Config()`. There is no "must be armed at boot"
caveat: the wrap and the middleware are structural and always installed (`nil`
injector ⇒ transparent pass-through), so an inert → armed push takes effect on
the next call.

## 3. Constraints

- **The cloud vocabulary, nothing else.** Its only import is `cloud/governance/dataplane/outbound/chain`
  (the client adapter's `Executor`, and the record the fire is written into), plus the gs wiring in
  `starter.go`; no third-party dependency is introduced.
- **Zero-config is transparent — a nil *injector* is not.** With the injector
  present and no rule armed, the unarmed kernel decides nothing and `fn` runs
  untouched, the same zero-config invariant resilience's `NewRoundTripper`
  upholds. The injector itself must be non-nil: it is a bean whose constructor is
  REQUIRED, so an absent one fails startup rather than degrading silently.
- **Forwarded lifecycle.** `faultExecutor.Close` delegates to the inner
  executor; fault has no resources or policy of its own to manage.

## 4. Trade-offs / Alternatives Rejected

- **Short-circuit at the Execute boundary** (inject, return, never call inner):
  rejected — it bypasses retry/breaker/timeout, so it validates the *caller's*
  reaction, not the resilience stack. Kept as a future opt-in only if a use case
  appears.
- **A `FaultDriver` registry mirroring resilience's `Driver`.** Rejected for
  now: there is exactly one in-process injection strategy, so a registry is
  speculative. Add it when a second backend (e.g. chaos-mesh-driven) lands.
- **All matching rules composing (the previous model).** Rejected: an attempt
  returns one error, so later injectors were discarded yet still spent budget;
  latencies summed without a global bound; two rules on one service composed
  implicitly and unpredictably. In-process effects are only
  additive latency and one exclusive error, so composition added no expressive
  power. Rules are now an ordered list with the FIRST match governing.
- **Dialer / RoundTripper seams in the first cut.** Deferred until an HTTP or
  gRPC starter pilots fault; the package is shaped so `WrapDialer` /
  `WrapRoundTripper` drop in alongside `WrapClientExecutor`.
- **A global `InjectorFor()` resolver.** Rejected: each authority is built once
  at construction (`gs.Provide` builds the one `*clientfault.Manager` and the one
  `*serverfault.Manager`), so there is no global resolver. Client side
  `clientfault.Manager.WrapClientExecutor` wraps unconditionally and the bean is
  always present; server side calls `srv.Fire(ctx, target)` per call, with the
  middleware installed unconditionally (an inert config passes through).
