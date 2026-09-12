# starter-ants Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `config.go`) and the runnable [example/](example/)
(`example/check.sh` — self-asserting, no external dependencies). **ants' own semantics
(worker pool, purge, overload) are [ants docs](https://github.com/panjf2000/ants)** —
everything below is go-spring's increment: wiring, observers, panic policy, metrics.

**Activation**: any `spring.ants.instances.<name>` subtree. The starter registers via
`gs.Module(gs.OnProperty("spring.ants"))` + `conf.BindEach`, creating one named `Pool` bean
per map key. No keys, no pools; there is no `enabled` switch.

---

## 1. Complete worked project

A service with two isolated pools (small nonblocking I/O pool, larger blocking CPU pool),
panic reporting through the shared chain, and a metrics snapshot endpoint. File tree:

```
demo/
├── go.mod
├── main.go
├── worker.go
└── conf/
    └── app.properties
```

**go.mod** (module deps that matter):

```
require (
    go-spring.org/spring       v1.3.x
    go-spring.org/starter-ants latest
    go-spring.org/starter-actuator latest   // optional: expose stats via HTTP
)
```

**main.go**:

```go
package main

import (
    _ "demo/worker"

    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-ants"
)

func main() { gs.Run() }
```

**worker.go** — the application's entire pool surface:

```go
package worker

import (
    "context"
    "sync/atomic"
    "time"

    "go-spring.org/log"
    "go-spring.org/spring/gs"
    StarterAnts "go-spring.org/starter-ants"
)

// panics counts task panics reported through the shared goutil chain (§2.4).
var panics int64

func init() {
    // Observe the shared chain: pool-task panics are bridged into
    // goutil.ReportPanic by DefaultDriver, so every recover site (goroutines,
    // handlers, pool tasks) lands in one stream.
    prev := goutil.OnPanic
    goutil.OnPanic = func(ctx context.Context, info goutil.PanicInfo) {
        atomic.AddInt64(&panics, 1)
        log.Warnf(ctx, log.TagAppDef, "recovered panic: %v", info.Panic)
        prev(ctx, info)
    }
}

// Service injects pools BY NAME (the map keys under spring.ants).
type Service struct {
    IO      StarterAnts.Pool             `autowire:"io"`
    CPU     StarterAnts.Pool             `autowire:"cpu"`
    Metrics *StarterAnts.MetricsObserver `autowire:""`
}

func init() {
    // gs only wires root-reachable beans: a Provide bean that nothing injects
    // and that exports nothing is never instantiated in prod. Export as
    // gs.Rooter (or inject it somewhere) so the container materializes it.
    gs.Provide(func() *Service {
        s := &Service{}
        // demo workload: 100 counting tasks on the CPU pool
        go func() {
            time.Sleep(500 * time.Millisecond)
            var n int64
            for i := 0; i < 100; i++ {
                _ = s.CPU.Submit(func() { atomic.AddInt64(&n, 1) })
            }
        }()
        return s
    }).Export(gs.As[gs.Rooter]())
}
```

**conf/app.properties** — the complete, commented surface (copied from the example):

```properties
# --- io pool: small, fails fast instead of queueing ------------------------
spring.ants.instances.io.size=2
spring.ants.instances.io.nonblocking=true

# --- cpu pool: bounded blocking pool with idle-worker reclaim --------------
spring.ants.instances.cpu.size=8
spring.ants.instances.cpu.expiry-duration=10s

# --- optional knobs (defaults shown; see §3) -------------------------------
# spring.ants.instances.cpu.max-blocking-tasks=0
# spring.ants.instances.cpu.pre-alloc=false
# spring.ants.instances.cpu.disable-purge=false
```

**Verify** (isomorphic to `example/check.sh`):

```bash
cd demo && go run .                       # watch for "ants pool \"cpu\" initialized"
grep -c "recovered task panic" <log>      # after submitting a panicking task
```

The example asserts programmatically: 100/100 tasks ran, `IO.Cap()==2`, `CPU.Cap()==8`,
the third submit to a full nonblocking pool returns an error, and the panic handler fired.

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-ants
  └─ init():
       gs.Provide(newMetricsObserver).Export(gs.As[PoolObserver]())   // built-in observer bean
       add custom observer: gs.Provide(NewX).Export(gs.As[PoolObserver]())
gs.Run()
  ├─ gs.Module(gs.OnProperty("spring.ants")) fires (prefix check: any spring.ants.instances.* key)
  ├─ conf.BindEach("${spring.ants}") → one Config per map key
  ├─ per name: gs.Provide(ctor).Name(name).Destroy(destroyPool)
  │     ctor wires ①optional Driver bean — none → bundled DefaultDriver; several
  │               coexist → the entry selects one by name:
  │               spring.ants.instances.<name>.driver = <bean-name> (empty = the single
  │               Driver bean by type; naming a missing bean fails startup)
  │               ②[]PoolObserver collection — container collects every bean exported
  │                  as PoolObserver (built-in MetricsObserver is thus forced to exist)
  ├─ bean init: createPool(observers) → (Driver bean | DefaultDriver).CreatePool
  │     (ants.NewPool with WithPanicHandler(goutil.ReportPanic bridge))
  │     foldObservers(name, observers) folds the chain into observedPool at build time,
  │     so every Submit flows that fixed observer chain
  ├─ readiness: pools usable from Init/Run hooks
  └─ on shutdown: destroyPool → pool.Release() (stops the purge goroutine)
```

Design note (source comment): the starter uses `gs.Module` instead of `gs.Group` so the
pool's bean name is available to pass to observers — that is why `OnSubmit(name, task)`
receives the name. Observers exist only as container beans (no package-level registry):
to add one, `gs.Provide` a bean and export it as `PoolObserver`; the container
instantiates it and collects it into every pool's chain before any pool is built.

### 2.2 One Submit, layer by layer

`s.CPU.Submit(task)`:

1. `observedPool.Submit` (starter.go) wraps the task: it calls the `chain` snapshotted at
   pool-build time, folding every collected `PoolObserver`'s `OnSubmit` around it,
   innermost = your task.
2. The built-in `MetricsObserver.OnSubmit` (starter.go) wraps it once more with a
   running-counter increment/decrement — `running` is tracked per pool name.
3. `antsPool.Submit` hands the wrapped task to ants, which queues it for a worker.
4. On the worker goroutine: panic → the goutil report bridge (see §2.4); normal return →
   counters decrement via defer.

### 2.3 Observers — what exists and how to add one

| Hook | Signature | Registered by |
|------|-----------|---------------|
| `PoolObserver.OnSubmit` | `OnSubmit(name string, task func()) func()` | `gs.Provide(NewX).Export(gs.As[PoolObserver]())` |

Observers are container beans only (no package-level global registry; `RegisterObserver`
is gone): `gs.Provide` your observer's object/constructor and export it as `PoolObserver`.
The container collects every such bean (a `[]PoolObserver` collection injected into each
pool's ctor) before building any pool, and folds the chain into `observedPool` at build
time — the chain is then fixed for the pool's lifetime. The built-in `*MetricsObserver`
bean is registered by the starter and reaches every chain through the same collection.
Typical uses per the source comment: metrics, tracing context injection, duration
logging, per-pool rate limiting.

### 2.4 Panic chain (unified panic policy)

`DefaultDriver.CreatePool` installs `ants.WithPanicHandler(goutil.ReportPanic bridge)`, which
bridges straight into `goutil.ReportPanic(ctx, p)` — the same goutil chain
goroutine/handler/job panics use (the structured-log bridge `go-spring.org/log`
installs), so pool panics land in one report stream. ants hands over only the panic
value, so ReportPanic is called from the deferred recover in the worker — the
panicking frames are still on the stack. A process needing custom panic handling
contributes its own `Driver` bean with its own `ants.WithPanicHandler`.

### 2.5 Shutdown

`destroyPool` calls `pool.Release()`: workers finish their current task, the background
purge goroutine stops, memory is reclaimed. Tasks still queued at Release are dropped by
ants — drain business work in your own Stop hooks before returning.

---

## 3. Per-key behavior reference

Prefix `spring.ants.instances.<name>.*` — multi-instance, keys bind per map entry (NOT top-level
absolute refs; the Config is bound via `conf.BindEach` with the instance prefix).

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `size` | int | 256 | Max concurrent workers; `<=0` = unbounded (`Cap()` then -1). | Too small + `nonblocking=false` → submitters block; too large → no backpressure. |
| `expiry-duration` | duration | 1s | Idle-worker reclaim interval for ants' periodic purger. ⚠ ignored when `disable-purge=true`. | Very low value → worker churn (alloc/GC pressure); very high → idle goroutines linger. |
| `pre-alloc` | bool | false | Pre-allocates the worker-queue memory. | Only a startup-cost/latency tradeoff; no failure mode. |
| `max-blocking-tasks` | int | 0 | Cap on submitters blocked waiting for a free worker; 0 = unlimited. ⚠ ignored when `nonblocking=true`. | 0 + tiny pool + blocking mode → unbounded submitter pileup. |
| `nonblocking` | bool | false | Submit returns `ErrPoolOverload` immediately instead of blocking when full. ⚠ overrides `max-blocking-tasks`. | True without checking Submit's error → silently dropped tasks. |
| `disable-purge` | bool | false | Keeps workers forever; no purge goroutine. ⚠ makes `expiry-duration` dead. | Busy pools fine; bursty pools retain peak goroutine count. |

Reconciled against `grep -rhoE 'value:"[^"]+"'` — exactly these 6 keys, no more.

---

## 4. Verification & fault drills

### 4.1 Tasks execute via the pool (from the example)

```bash
cd starter/experimental/starter-ants/example && ./check.sh
# expects: "Nonblocking pool correctly rejected submit",
#          "Panic handler fired: 1 times", the pool metrics block, exit 0
```

Programmatic assertions you can reuse: submit N tasks, WaitGroup, compare counter to N;
compare `Cap()` against configured sizes to prove instance isolation.

### 4.2 Overload behavior drill

With `io.size=2` + `io.nonblocking=true`: submit two tasks that block on a channel, then a
third — `Submit` returns ants' `ErrPoolOverload`. With `nonblocking=false` the third submit
blocks instead (respecting `max-blocking-tasks` if set).

### 4.3 Panic drill

Submit `func(){ panic("boom") }` on any DefaultDriver pool: the process stays up; the
panic surfaces through the goutil report chain (structured log). With the example's
`goutil.OnPanic` observer installed, the counter increments and a Warn line is logged —
proving the bridge.

### 4.4 Metrics drill

```go
stats := s.Metrics.Snapshot()               // Name + Running (Cap/Free/Waiting zero)
s.Metrics.Enrich(&stats, map[string]StarterAnts.Pool{"io": s.IO, "cpu": s.CPU})
```

`Enrich` needs the pool handles because the observer knows names, not Pools — see suspect #1.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| No pool beans; autowire `Pool` fails | No `spring.ants.instances.*` keys | Add at least `spring.ants.instances.<name>.size` — the subtree is the activation switch. |
| Custom pool Driver bean never used | The entry did not select it and by-type injection picked another | When several `Driver` beans coexist, select one per pool: `spring.ants.instances.<name>.driver = <bean-name>` (empty = the single Driver bean by type; naming a missing bean fails startup). |
| Tasks silently dropped | `nonblocking=true` + unchecked Submit error | Check Submit's error (it returns `ErrPoolOverload`). |
| Submitters hang | Blocking pool at capacity, `max-blocking-tasks=0` | Raise `size`, set `max-blocking-tasks`, or go nonblocking. |
| Pool panics unobserved by custom logic | Default chain only reports | Install a `goutil.OnPanic` override or contribute a custom `Driver` bean. |
| Custom `PoolObserver` ignored | Bean not exported as `PoolObserver` (or the container didn't collect it during assembly) | `gs.Provide(NewX).Export(gs.As[PoolObserver]())`; observers go through container beans only. |
| High goroutine count on idle | `disable-purge=true` or huge `expiry-duration` | Re-enable purge / lower `expiry-duration`. |
| Panic reported twice | A `goutil.OnPanic` override chains to the previous handler AND the log bridge also reports | Wrap-and-forward (`prev(ctx, info)`) only once, or don't chain. |

---

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 6 |
| Required | 0 |
| Quickstart external deps | 0 |
| "Watch out" entries | 6 |

Design suspects (kept from the previous edition; for the audit ledger):

1. `Snapshot()` returning half-empty stats that the caller must `Enrich` with pools it has
   to collect by hand — the observer knows pool names but not Pool handles; asymmetric API.
2. Driver is a single optional bean (at most one per process) — every pool shares the one
   assembly; per-pool driver selection by name is no longer possible (per-Config differences
   must flow through the Config the driver receives).
3. ~~(fixed)~~ panic handler used to be a package-level global mutated by `SetPanicHandler`
   (ordering-sensitive, data race window) — removed; panics now bridge straight into the
   goutil chain, custom handling goes through a `Driver` bean.
4. ~~(fixed)~~ observers used to go through a package-level registry `RegisterObserver` plus a
   Submit-time RLock snapshot of `Observers()` (a hot-path lock purely for late registration) —
   now they are injected via a container `[]PoolObserver` collection and the chain is folded
   into the pool at build time; the hot path is lock-free. `MetricsObserver` is also guaranteed
   to be instantiated by being collected (fixing a latent bug where prod might never materialize
   it, so its self-registration never ran).
