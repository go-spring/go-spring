# starter-resilience Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `executor.go`, `breaker_listener.go`,
`executor_test.go`), the abstraction in
[cloud/governance/resilience](../../../cloud/governance/resilience) (`driver.go`,
`provider.go`), and the self-asserting [example/](example/) (`example/check.sh` — no
container, no external deps). **Resilience semantics (breaker windows, retry backoff,
policy vocabulary) are [cloud/governance/resilience](../../../cloud/governance/resilience);
sentinel-golang behavior is [official docs](https://github.com/alibaba/sentinel-golang)** —
only the driver wiring is covered here.

**Activation**: blank import only. `init` (starter.go) calls `sentinel.InitDefault()`
(panicking on failure — "a misconfigured environment fails loudly here rather than on first
use", per the source comment) and `resilience.RegisterDriver("sentinel", sentinelDriver{})`.
No beans, no port, **no configuration keys of its own** — policies are configured on
whichever consumer selects the driver.

---

## 1. Complete worked project

A self-contained smoke exercising the client-side seams on the sentinel driver: the dialer
seam (breaker at connection establishment) and a composed rate-limit + breaker + retry
policy. File tree (this IS the example):

```
demo/
├── go.mod
├── main.go
└── conf/app.properties   // empty: pure API smoke, no spring.* keys needed
```

**go.mod** (module deps that matter):

```
require (
    go-spring.org/cloud              v0.0.0   // governance/resilience
    go-spring.org/starter-resilience latest
)
```

**main.go** (from the example):

```go
package main

import (
    "context"
    "errors"
    "net"
    "net/http"
    "sync/atomic"
    "time"

    "go-spring.org/cloud/governance/resilience"

    _ "go-spring.org/starter-resilience" // registers the "sentinel" driver
)

func main() {
    driver, err := resilience.GetDriver("sentinel")
    if err != nil {
        panic(err)
    }
    demoClientDialer(driver)
    demoComposedRetry(driver)
}

// demoClientDialer: a breaker with threshold 3 trips after three refused dials;
// the fourth is short-circuited with the neutral ErrCircuitOpen BEFORE touching
// the network.
exec, _ := driver.NewExecutor(resilience.Policy{ErrorThreshold: 3, OpenDuration: time.Minute})
ln, _ := net.Listen("tcp", "127.0.0.1:0"); deadAddr := ln.Addr().String(); _ = ln.Close()
base := resilience.DialFunc((&net.Dialer{Timeout: time.Second}).DialContext)
dial := resilience.NewDialer(base, exec, "dead-service")
for i := 1; i <= 3; i++ { _, err := dial(ctx, "tcp", deadAddr) /* real dial error */ }
_, err := dial(ctx, "tcp", deadAddr)
errors.Is(err, resilience.ErrCircuitOpen) // true — breaker open, network untouched

// demoComposedRetry: flaky upstream 503s twice then succeeds — recovered within
// the retry budget while rate limit and breaker stay transparent.
exec2, _ := driver.NewExecutor(resilience.Policy{
    RateLimit: 100, ErrorThreshold: 10, MaxRetries: 3, Timeout: time.Second,
})
client := &http.Client{Transport: resilience.NewRoundTripper(http.DefaultTransport, exec2, nil)}
resp, _ := client.Get("http://127.0.0.1:PORT/flaky") // 200; server hit exactly 3 times
```

**Verify**:

```bash
cd starter/experimental/starter-resilience/example && ./check.sh
# expects: "client Dialer: circuit opened after 3 refused dials",
#          "composed policy: recovered after 3 attempts ...", exit 0
```

The more common production path is declarative: any driver-aware consumer selects sentinel
by name, e.g. `spring.http.client.<name>.resilience.driver=sentinel` (starter-http-client /
oauth2-client). With no `driver` key those starters stay on the zero-dependency `default`
driver. Under the governance center, clients do not pick a driver at all — they call
`resilience.ExecutorFor(label)` (see §2.2).

---

## 2. Assembly & timing

### 2.1 Driver wiring

```
import starter-resilience
  └─ init(): sentinel.InitDefault()          // panics on failure
             resilience.RegisterDriver("sentinel", sentinelDriver{})
```

- `sentinelDriver.NewExecutor(p)` (starter.go) → `newSentinelExecutor(p)`
  (executor.go): rejects a negative `RateLimit` up front; otherwise constructs the
  executor with an empty per-resource loaded set.
- sentinel keys everything by resource name, so **rules load lazily** the first time a
  resource is seen: `ensureRules(resource)` (executor.go) installs, under mutex, at most
  three rule types — flow (RateLimit>0), circuit-breaker (BreakerActive()), isolation
  (MaxConcurrent>0, under the suffixed name, see §2.3) — then marks the resource loaded.

### 2.2 The ExecutorFor seam (governance center integration)

`resilience.ExecutorFor(resource)` (provider.go in the abstraction, NOT this starter) is
the single call clients use instead of injecting a governance center:

- It returns a stable `resolvedExecutor` holding only the label; the backing executor is
  resolved **lazily on each Execute** and memoized per label (sync.Map cache).
- The provider is installed once by starter-govern after building the governance center;
  because resolution is deferred to call time, client-vs-govern wiring order is irrelevant.
- With no provider registered (starter-govern absent or governance disabled), the executor
  is a transparent **no-op**: fn runs once, untouched — uniform client code whether or not
  resilience is configured.
- Hot-reload rides on the backing executor: the provider registers a governance
  subscription and refreshes it in place; this starter's `Refresh` (below) is what actually
  applies the new policy to sentinel.

### 2.3 One Execute call, layer by layer

`exec.Execute(ctx, "my-resource", fn)` (executor.go):

1. `ensureRules("my-resource")` — load/verify flow + breaker + isolation rules.
2. **Bulkhead first**: if `MaxConcurrent>0`, one Entry under `"my-resource$bulkhead"`
   (`isoSuffix`), held via defer for the WHOLE Execute including retries. The suffix exists
   because sentinel evaluates every rule type registered under a resource on each Entry —
   the concurrency slot and the per-attempt entry must live under distinct resource names
   to be acquired independently (source comment).
3. **MaxDuration budget**: if set, a `context.WithTimeout` wraps everything below.
4. Per attempt (up to `MaxRetries+1`):
   - `sentinel.Entry(resource, Outbound)` — drives flow + circuit-breaking. A block error
     exits immediately through `mapBlockError` (§2.5); no retry on blocks.
   - `runOnce` applies the per-attempt `Timeout` (if >0) around `fn`.
   - On error: `sentinel.TraceError(entry, err)` feeds the breaker statistic, then the loop
     checks budget (`budgetCtx.Err()`), the retry predicate `ShouldRetry`, the last-attempt
     index, and sleeps `Backoff(i)` (aborting early if the budget expired mid-sleep).
5. Nil on first success; otherwise the last error.

### 2.4 Refresh (policy hot-reload)

`sentinelExecutor.Refresh(p)` (executor.go): validates `RateLimit>=0`, swaps the policy and
**clears the loaded set** under mutex. sentinel's `LoadRulesOfResource` replaces a
resource's existing rules, so the next Execute reloads everything under the new thresholds
— and resets the breaker's stat window. This mirrors the default driver's
"discard state, rebuild on next call" lazy semantic (source comment). Route listeners are
re-registered when `ensureRules` re-runs.

### 2.5 Breaker events and outcome mapping (breaker_listener.go)

- **State events**: sentinel's `StateChangeListener` is a process-wide singleton, so a
  single `routeListener` demultiplexes by `rule.Resource` through the `breakerRoutes`
  sync.Map to the per-executor `resilience.BreakerEventListener`. Attachment:
  `SetBreakerEventListener(l)` (implements `resilience.BreakerEventListenerSetter`;
  observe-resilience's WrapExecutor uses it); the listener is registered with sentinel
  once (`ensureRouteListener`, sync.Once) and per resource inside `ensureRules`. States map
  1:1: sentinel Open/HalfOpen/Closed → `resilience.BreakerOpen/BreakerHalfOpen/BreakerClosed`.
  A resource with no route (breaker rules loaded outside go-spring) is silently ignored.
- **Block outcomes**: `mapBlockError` translates sentinel's block reason into the neutral
  sentinels — `BlockTypeCircuitBreaking` → `ErrCircuitOpen`, `BlockTypeIsolation` →
  `ErrBulkheadFull`, everything else (i.e. flow rejection) → `ErrRateLimited` — so callers
  import only the resilience package.

### 2.6 Policy → sentinel rule translation (verified in `ensureRules`/`loadBreakerRule`)

| Policy field | Sentinel rule | Default when zero |
|---|---|---|
| `RateLimit` | flow.Rule Direct/Reject, `StatIntervalInMs=1000`, `Threshold=RateLimit` | not installed when `<=0` |
| `BreakerStrategy=ErrorRate` | circuitbreaker.ErrorRatio; `Threshold=ErrorRateThreshold`; `MinRequestAmount=MinRequests` | MinRequests → 1 |
| `BreakerStrategy=Consecutive` | circuitbreaker.ErrorCount; `Threshold=float64(ErrorThreshold)`; `MinRequestAmount=1` | — |
| `OpenDuration` | `RetryTimeoutMs` | 5000ms |
| `BreakerWindow` | `StatIntervalMs` | 1000ms |
| (both strategies) | `ProbeNum=1` — exactly-one-trial half-open, aligning with the builtin's single-permit gate | — |
| `MaxConcurrent` | isolation.Rule Concurrency under `resource$bulkhead` | not installed when `<=0` |

Retry and per-attempt timeout are NOT sentinel concepts — they wrap the entry check in this
executor (`Execute`/`runOnce`).

---

## 3. Per-key behavior reference

**No keys under this module's own prefix.** `grep -rhoE 'value:"[^"]+"' starter-resilience`
yields nothing. The `Policy` knobs (`rate-limit`, `error-threshold`, `open-duration`,
`max-concurrent`, `max-retries`, `timeout`, ...) are documented by the consuming starter
(e.g. `spring.http.client.<name>.resilience.*`) and by
[cloud/governance/resilience](../../../cloud/governance/resilience); under the governance
center they live in the centralized source (`govern.resilience.*`) and reach this driver via
`ExecutorFor` + `Refresh`.

---

## 4. Verification & fault drills

### 4.1 Driver registration

```bash
cd starter/experimental/starter-resilience && go test ./...
# executor_test.go pins the translation table in §2.6 and the block-error mapping
```

Or: `resilience.GetDriver("sentinel")` must return non-nil with no error; the import-time
Info log "registered sentinel resilience driver" (tag: app default) confirms init ran.

### 4.2 Breaker drill (from the example)

`Policy{ErrorThreshold: 3, OpenDuration: time.Minute}` against a dead address: dials 1–3
fail with real errors; dial 4 returns `errors.Is(err, resilience.ErrCircuitOpen)` without
touching the network. Flip side: with `RetryTimeoutMs` = the minute elapsed, the next Entry
is a single half-open probe (`ProbeNum=1`).

### 4.3 Composed policy drill (from the example)

`Policy{RateLimit: 100, ErrorThreshold: 10, MaxRetries: 3, Timeout: time.Second}` against
an upstream that 503s twice: the client sees 200; `hits == 3` proves the retry budget —
not the breaker — did the recovery, and the generous limit stayed transparent.

### 4.4 Rate-limit drill

`Policy{RateLimit: 1}` and a fast loop of Executes: calls beyond ~1/s fail with
`ErrRateLimited` (`BlockTypeFlow` → the default arm of `mapBlockError`).

### 4.5 Refresh drill (hot-reload)

Hold an executor built with `RateLimit: 1`, observe throttling as in §4.4, then call
`exec.Refresh(resilience.Policy{RateLimit: 1000})` (or push the change through the
governance source when running under starter-govern): the next Execute reloads rules and
throttling stops. Note the breaker stat window resets on refresh (§2.4).

### 4.6 Breaker event observation

Implement `resilience.BreakerEventListener`, attach via `SetBreakerEventListener` before
first Execute of the resource: transitions fire as Closed→Open→HalfOpen→Closed during the
drills above — the seam observe-resilience's WrapExecutor consumes.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Process panics at import: "sentinel init failed" | broken environment for `sentinel.InitDefault()` (config/log dirs) | Fix the env — the panic is by design, fail-loudly at import (suspect #1). |
| `GetDriver("sentinel")` errors | starter not blank-imported | Add `import _ "go-spring.org/starter-resilience"`. |
| Consumers stay on builtin resilience | consumer's `...resilience.driver` unset | Set `=sentinel` per consumer — there is no global switch (suspect #3). |
| Breaker never opens | `MinRequestAmount` not yet reached, or `ErrorThreshold`/window mis-sized | Check the defaults table §2.6 (`MinRequests` → 1, window → 1000ms). |
| Breaker state looks reset after a config push | `Refresh` clears rules; sentinel replaces them and resets the stat window | Intended lazy-reload semantic (§2.4). |
| Doubled resources in sentinel console/metrics | bulkhead lives under `resource$bulkhead` | Driver-internal naming (suspect #2) — filter by suffix. |
| No retry despite `MaxRetries` set | `ShouldRetry(err)` false, or `MaxDuration` budget exhausted before next attempt | Check the policy's retry predicate and budget. |
| Everything no-op despite sentinel imported | running via `ExecutorFor` with no provider (no starter-govern / governance disabled) | Expected zero-cost fallback — configure governance or use the driver directly. |

---

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 0 (owned by consumers) |
| Required | 0 |
| Quickstart external deps | 0 |
| "Watch out" entries | 3 |

Design suspects (kept from the previous edition; for the audit ledger):

1. `sentinel.InitDefault()` + panic at import time makes the failure mode an import-order
   crash rather than a normal boot error.
2. Bulkhead lives under a `resource$bulkhead` suffixed name — sentinel console/metrics show
   twice the resources; leakage of driver internals into observability.
3. Driver selection is per-consumer config (`...resilience.driver=sentinel`) with no global
   switch; enabling sentinel everywhere means repeating the key per client.
