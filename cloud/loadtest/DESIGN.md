# loadtest Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`loadtest` is the shared harness behind the client starters' example-load
binaries. It exists so "run a load, judge whether the protection stack
held" is one reusable program rather than a per-starter copy-paste main.

## 1. Responsibilities & Boundaries

- **Does:** drive an `Op` under a pluggable `Driver`, record latency +
  classified error buckets + GC summary, run registered `Assert`ions, and
  print a diffable report.
- **Refuses:**
  - No production claims. It is example-grade: single-mutex recorder, plain
    `time.Sleep` pacing, no distribution fitting. When a run needs
    statistical rigor, reach for a real load tool.
  - No opinion about the op. `Op` is `func(ctx) error`; the
    starter-specific example main supplies the client call.

## 2. The four seams

- **`Driver` owns WHEN; the runner owns WHAT.** `Runner.Run` wraps the op
  in an `invoke` that already times and records, so a driver is pure
  scheduling — timing, concurrency, and nothing else. The contract is two
  clauses: invoke once per scheduled op, and do not return until every
  started invoke finished (no lost tail latency). That single seam covers
  closed-loop, open-loop, ramp, staircase, and arbitrary `Schedule`
  shapes without the runner knowing any of them.
- **`Schedule` composes into `Driver`s.** The open-loop family
  (`OpenLoop`, `Ramp`, `Staircase`, `Scheduled`) is one
  `scheduledDriver` parameterized by a pacing function
  `func(elapsed) float64`; deadline-driven dispatch (recompute the
  interval each iteration, not a fixed ticker) is what lets a varying
  schedule speed up and slow down. `MaxConcurrent` is a bounded semaphore
  because open-loop under sustained excess arrival otherwise spawns
  goroutines without limit.
- **`Assert` turns a run into a verdict.** Assertions run after driving
  completes, against the finished `Result`, with the *original* ctx (not
  the duration-bounded one) so follow-up lookups aren't cancelled.
  `Result.Passed` is true when nothing failed — a run with no assertions
  has nothing to fail.
- **`Classify` maps errors to buckets.** Default is the resilience +
  fault taxonomy; `""` and unknown labels collapse to `other` so a custom
  classifier can never lose an error.

## 3. Relationship with the traffic load-test marking

Every op context is tagged `traffic.WithLoadTest(runCtx, "loadtest.Run")`
before dispatch, so the whole downstream chain — http clients, gateways,
caches — can recognize synthetic load via `traffic.IsLoadTest` and the
propagation helpers. Shadow-table routing, fault-injection scoping, and
metric labeling in `cloud/governance/traffic` all key off that marker; the
harness is its main producer. Without the tagging, a load run would be
indistinguishable from production traffic everywhere except at the driver.

## 4. Single-track error buckets

One flat `map[string]int64` for built-in and custom labels alike — no
separate "custom" section. `Result.Print` fixes the order: the five
built-in constants (`circuit`, `rate-limited`, `bulkhead`,
`fault-injected`, `other`) in declaration order, then custom labels
sorted. Reports stay diffable regardless of map iteration order, which
matters because the example-load binaries' output is eyeballed (and
occasionally diffed) across runs and starters.

## 5. Trade-offs / Alternatives Rejected

- **Example-grade recorder, not a lock-free one.** One mutex over the
  latency slice and bucket map is the simpler trade at these op counts;
  per-label atomics would buy nothing visible.
- **`time.Sleep` pacing instead of timer wheels.** Cancellation is noticed
  on the next loop iteration; for a harness this is acceptable and keeps
  the driver readable.
- **Assertions over statistics.** Pass/fail thresholds (`AssertMinQPS`
  etc.) answer the operational question the example-load binaries ask;
  percentile confidence intervals are out of scope.
- **Legacy `Run` kept as a thin wrapper** over a closed-loop `Runner`
  rather than deleted: the per-starter example mains already use it, and a
  one-liner is the right size for a smoke load.
