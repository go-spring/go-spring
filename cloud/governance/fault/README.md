# fault

`fault` is the in-process **fault-injection** companion to
[cloud/governance/resilience](../resilience). It wraps a `resilience.ClientExecutor` so a
configurable fraction of operations are made to fail or slow down on demand —
"setting fire" to a running client — to verify that retry, circuit-breaker,
per-attempt timeout and Fallback actually engage, and that the observe kit
records the resulting outcomes.

## Features

- Two seams: `WrapClientExecutor` (client side — wraps an executor so injected faults
  land *inside* the retry/breaker loop) and `Server.Fire` (server side — decides,
  before the handler runs, whether an inbound call should be set on fire). Both
  validate resilience, not bypass it.
- Centralized, hot-reloadable config. fault rides the same `governance.Config`
  (and so the same source document) as resilience (see
  [README 设计说明 §8](../README.md)); `cloud/governance/fault` owns TWO
  independent authorities — `*ClientManager` and `*ServerManager`, each registered
  as its own bean — and each swaps its own config in place via `Refresh`, so
  toggling fires at runtime with no restart.
- One config per DIRECTION, so a fire on one side cannot touch the other:
  spring.governance.client.fault.* drives outbound calls, spring.governance.server.fault.* drives
  inbound requests, and each EXPERIMENT counts its own MaxDuration/MaxAffected
  guardrails. An outbound fire that self-heals leaves the inbound one armed.
- Shares the one governance document with resilience: the fault config is nested
  inside the direction blocks (`spring.governance.client.fault` /
  `spring.governance.server.fault`). Each direction is a list of EXPERIMENTS and
  there is no top-level effect; every guardrail counter is per experiment, keyed
  by the rule's identity, so one fire self-healing cannot stop another.
- Four injection kinds: `generic` (a retryable injected error), `timeout`
  (`context.DeadlineExceeded`), `reset` (`syscall.ECONNRESET`), `refused`
  (`syscall.ECONNREFUSED`); plus a pure latency mode. Per-service rules via
  `Config.Rules`.
- Injected errors implement `resilience.Retryable`, so they deterministically
  drive retries regardless of the host's retry predicate.
- No deps of its own beyond the cloud vocabulary — `cloud/chain` (the client
  adapter's `Executor` and the record the fire is written into) and `cloud/traffic` (the
  load-test propagator `scope` reads), plus the gs wiring in `starter.go`. No
  third-party dependency is introduced.

## Install

```sh
go get go-spring.org/cloud
```

## Usage

In a starter, the executor and injector arrive as injected beans (a
*resilience.ClientManager plus the side's own fault authority — `*fault.ClientManager`
for outbound, `*fault.ServerManager` for inbound — all nullable):

```go
import (
    "go-spring.org/cloud/governance/fault"
    "go-spring.org/cloud/governance/resilience"
)

// The authorities are beans: cloud/governance/fault registers one *fault.ClientManager and
// one *fault.ServerManager, and a starter takes the one its side needs as a
// nullable constructor parameter (gs.IndexArg(n, gs.TagArg("?"))). nil means no
// governance is in the container, and both entry points below are nil-safe, so
// the wrap is a transparent pass-through — a client is never forced to depend on
// the starter.
//
// client side: mgr.ClientExecutorFor yields the governed executor with the observe
// layer already applied; the client authority wraps it from the outside. Holding
// the pointer is enough to observe config changes: the center swaps its config
// in place with ClientManager.Refresh rather than replacing it.
exec := injClient.WrapClientExecutor(mgr.ClientExecutorFor("redis", service))

// server side: the shell is created ONCE, when the middleware is built, and
// reads the pushed kernel from then on — the manager is not consulted per
// request. The Target names the inbound call: the entrance plus the route being
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
next(ctx) // `d` (Decision) says which rule governed and whether anything fired
```

For a self-contained injector (tests, cloud/experimental/loadtest), build the
side you need with `fault.NewClientManager(prop)` / `fault.NewServerManager(prop)`
and arm it with its own `Refresh`, then pass it the same way. One `Config` type
serves both directions — the kernel is symmetric, so only the values differ — and
the direction is fixed by which authority you hold, never by a parameter a caller
could aim wrong.

Config (centralized in the governance rules document, one block per direction —
see [README 配置指南 §6](../README.md)):

```properties
# outbound: WrapExecutor's per-attempt gate (mgr.ClientExecutorFor → fault.ClientManager.WrapClientExecutor).
# One rule = one experiment; there is no global default, so nothing fires unless
# a rule matches. `service` is the client's own label (see Target).
spring.governance.client.fault.rules[0].service=redis:cache
spring.governance.client.fault.rules[0].rate=0.5
spring.governance.client.fault.rules[0].error=generic        # "" | "generic" | "timeout" | "reset" | "refused"
spring.governance.client.fault.rules[0].latency=50ms         # optional, applied on every hit
spring.governance.client.fault.rules[0].latency-jitter=20ms  # optional, sleep latency ± U[0,20ms]
spring.governance.client.fault.rules[0].max-affected=100     # optional blast-radius cap
spring.governance.client.fault.rules[0].max-duration=5m      # optional self-heal window
spring.governance.client.fault.rules[0].scope=loadtest       # optional: "" | "real" | "loadtest"

# inbound: Server.Fire's decision (gin / echo / grpc / hertz / trpc / dubbo).
# An empty `service` is the catch-all — the explicit way to fire everything.
spring.governance.server.fault.rules[0].service=grpc
spring.governance.server.fault.rules[0].action.exact=/demo.Service/Echo
spring.governance.server.fault.rules[0].rate=0.2
spring.governance.server.fault.rules[0].error=timeout
```

A direction is armed when at least one rule carries a `rate` or a `latency`;
with no such rule it injects nothing. There is no separate `enabled` key, and —
deliberately — no top-level effect: an operator aiming at one service cannot
accidentally burn the rest of the process, and every fire is visible as a rule
in the config diff.

Read the live values with `ClientManager.Config()` / `ServerManager.Config()`.

See the [Design](#design) section below for the injection-point rationale and boundaries, and
`starter-redigo/example-load` for a runnable load test that toggles fault.

---

# Design

`fault` is the in-process fault-injection companion to
[cloud/governance/resilience](../resilience). Where resilience *protects* a client against
downstream failures, fault *manufactures* them on demand so the protection
stack can be proven under load. It ships two seams — `WrapClientExecutor` for outbound
calls, `Server.Fire` for inbound requests — plus the load-test binary
`starter-redigo/example-load` that drives the outbound one end to end.

## 1. Responsibilities & Boundaries

- **Does:** wrap a `resilience.ClientExecutor` so a configurable fraction of
  operations are made to fail (or slow down) before the real executor sees
  them; hot-swap the live config behind an atomic pointer; expose neutral
  `InjectedError` values that are `resilience.Retryable` and surface as familiar
  Go errors (`context.DeadlineExceeded`, `syscall.ECONNRESET`).
- **Refuses:**
  - No gs / spring dependency in the DOMAIN code: the injector is a plain object,
    and `WrapClientExecutor` / `Server.Fire` are container-free functions. The one
    bean is registered in this package's `starter.go`, and the hot-reload is driven
    by the governance center (it shares the single Config — and so the same source —
    with resilience), not by each client starter.
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

`fire` takes no callback and records nothing itself: it returns the `Decision`
(the rule that governed, whether an error was injected, the latency applied)
plus the "may I proceed?" error, and the client seam writes the decision onto
the call's record. Keeping the write in the adapter is what lets the engine stay
free of call state (and of `chain` entirely), while the observe kit still reads
the record back and tags the span and the access line. The write must sit **inside** the per-attempt closure: a
decision captured after `Execute` returned would be only the last attempt's, and
a call that fired on one attempt and succeeded on another would lose its tag.

`Decision.Rule` names the rule that *governed*, not only the one that fired:
a rule that matched but whose rate did not hit is still named, and `Fired()`
tells the two apart — which is what makes "why did my rule not fire?" answerable
from the decision alone.

The injection point is deliberate: because `fn` is faulted **inside** the inner
executor's retry loop, an injected failure is retried, counted by the breaker,
bounded by the per-attempt timeout and surfaced to Fallback — exactly the path a
real downstream failure takes. Short-circuiting at the `Execute` boundary
instead would bypass all of that and defeat the purpose.

**Stack order in a host starter** (e.g. redigo): `fault( observe( rawExec ) )`.
`ClientManager.ClientExecutorFor` applies the observe layer itself, on the still-private
governed executor (that is what lets observe attach its breaker listener at
construction); fault then wraps that from the outside. The injected fault still
reaches the real executor's retry loop, so it is both *handled* by resilience
and *recorded* by observe.

**Retryability.** `InjectedError` implements `Retryable() bool` returning true,
which `resilience.ClientPolicy.ShouldRetry` consults first — so injected faults
deterministically drive retries regardless of the host's configured predicate.
The typed kinds wrap a real error so `errors.Is(err, context.DeadlineExceeded)`
works and downstream classifiers (observe's outcome map) label the call like a
genuine timeout/reset.

**Naming the call: `Target`.** A rule is matched against a `Target` — the three
coordinates a rule can name: `Service` (what was called), `Action` (which
interface of it) and `Metadata` (the caller's own targeting dimensions, matched
by a rule's `Match`). It is fault's own type, deliberately not `chain.Operation`:
that is a client-shaped *declaration* of a call, carrying the trace/metric/log
blocks and the client-edge kind a rule never reads, and an inbound request has no
meaningful one to offer. Each seam's adapter builds the Target, because only it
knows its protocol's vocabulary:

- client (`WrapClientExecutor`): `Service` is the call's target, falling back to
  the label the executor was built for; `Action` is `chain.Request.Action`.
- server (`Server.Fire`): `Service` is the entrance ("gin", "grpc"), `Action` is
  the route or method being invoked, so **an inbound rule can target one
  interface** — `GET /users/{id}`, `/pkg.Svc/Method` — instead of a whole
  entrance. Adapters also expose request metadata if an operator wants to match
  on it.

A rule names what it targets with `MetadataMatch` (a plain `map[string]string`,
value-equality subset-match) and `ActionMatch` (an `ActionMatcher`: exact or
prefix). The field names say which axis they match, because both are conditions
on different ones.

The matchers stay deliberately shallow. A router needs rich matching — forwarding
to the wrong backend is an incident — while a fire needs matching an operator can
predict, because the first question in a drill is "what exactly did we hit?".
The mainstream chaos tools agree: Chaos Mesh matches method + path (wildcards) +
a header map; ChaosBlade exposes regex only as a separate opt-in flag. The rich
match grammars (Envoy `StringMatcher`, Istio `StringMatch`, Gateway API, Traefik,
APISIX `vars`) belong to traffic management. The axis a fire needs beyond this is
the DOSE — `Rate` / `MaxAffected` / `MaxDuration` / `Scope` — not a richer
predicate. `ActionMatcher` is fault's own type for the same reason as `Target`:
structurally identical to `chain.ActionMatch`, derived from neither.

EXTENSION: a new operator is a new field under the same one-of convention
(`ActionMatcher`), never a reinterpretation of an existing one; richer metadata
matching arrives as a sibling `MetadataMatchers []MetadataMatcher` that ANDs with
the map. Either way `match.tenant=acme` keeps its meaning forever.

**State: stable shell, replaceable core — pushed, not queried.** The manager
follows the same model as `resilience.ClientManager` and `loadbalance.Manager`, with the
ownership the model requires: every shell (`Client` / `Server`) owns an atomic
pointer to its kernel and reads it directly, so the call path never asks the
manager anything. The manager REGISTERS each shell it creates and, on a refresh,
builds the new kernel once and PUSHES it into every registered shell under its
lock. A refresh reconciles rather than replaces — a rule whose identity
(`service[index]`) is unchanged keeps its counters, a new rule gets fresh ones,
and rules that are gone are dropped with the old core. Presence in the pushed
document is therefore an experiment's whole lifecycle: appearing starts it,
disappearing ends and releases it, `max-duration` / `max-affected` end it
automatically, and closing a shell unregisters it. There is deliberately no
separate start/stop call — the document is the control plane.

**Two directions, two Managers.** `ClientManager` and `ServerManager` are
independent authorities, each holding its own `direction` (the pushed document
plus its registered shells), so an outbound fire and an inbound fire never see or
trip each other: a `Client` reads the client authority's kernel, a `Server` the
server authority's. The two types share the neutral kernel in `core.go` — rule
semantics, injection sequence, guardrail model — but no authority object and no
creation API, which keeps each side free to evolve its own seam. That is why the two configs
must be separate keys (`spring.governance.client.fault.*` /
`spring.governance.server.fault.*`) rather than one block with a per-rule
direction flag — the direction is a property of WHERE the fire is evaluated, and
a catch-all rule is meaningful on either side, so nothing inside a rule can
declare it.

**Hot-reload.** Each side holds its `Config` behind `atomic.Pointer`; the
governance center calls each authority's `Refresh` on every source push, so faults
toggle at runtime without a restart. There is no "must be armed at boot" caveat: the wrap
and the middleware are structural and always installed (`nil` injector ⇒
transparent pass-through), so an inert → armed push takes effect on the next
call.

## 3. Constraints

- **The cloud vocabulary, nothing else.** Its only imports are `cloud/chain`
  (the client adapter's `Executor`, and the record the fire is written into) and
  `cloud/traffic` (the load-test propagator `scope` reads), plus the gs wiring in
  `starter.go`; no third-party dependency is introduced.
- **Zero-config is transparent — a nil *injector* is not.** With the injector
  present and no rule armed, the gate short-circuits on `Config.Inert()` and runs
  `fn` untouched, the same zero-config invariant resilience's `NewRoundTripper`
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
- **Dialer / RoundTripper seams in the first cut.** Deferred until an HTTP or
  gRPC starter pilots fault; the package is shaped so `WrapDialer` /
  `WrapRoundTripper` drop in alongside `WrapClientExecutor`.
- **A global `InjectorFor()` resolver.** Rejected: each authority is built once
  at construction (`gs.Provide` builds the one `*fault.ClientManager` and the one
  `*fault.ServerManager`), so there is no global resolver. Client side
  `fault.ClientManager.WrapClientExecutor` wraps unconditionally and the bean is
  always present; server side calls `srv.Fire(ctx, target)` per call, with the
  middleware installed unconditionally (an inert config passes through).
