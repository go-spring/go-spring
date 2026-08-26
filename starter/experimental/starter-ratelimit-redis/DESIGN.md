# Design: starter-ratelimit-redis

## 1. Problem

The bundled resilience limiter driver ("default") keeps counters in-process,
so N replicas allow N× the configured budget. What production usually wants
for shared quotas is one budget enforced globally — which requires state
outside any single process, i.e. Redis.

## 2. Why a limiter-driver registration, not a resilience-driver switch

resilience has two independent registries:

- `RegisterDriver`/`GetDriver` — builds an **Executor** (limit + breaker +
  retry + timeout as one bundle). Switching it to swap in Redis limiting would
  also drag breaker/retry off `default`/`sentinel`, which is not what anyone
  wants: distributed limiting is orthogonal to in-process circuit breaking.
- `RegisterLimiter`/`GetLimiter` — the **LimiterDriver** registry, added
  precisely as the "distributed limiting" seam (see ratelimit.go comments) and
  already consumed by-name (gateway's `rateLimit(rate=…,driver=…)`).

So this starter registers a `LimiterDriver` under a configurable name
(default: the instance name, so multi-instance setups stay collision-free) and
leaves the executor driver alone. Result: "sentinel executor + redis limiter"
composes freely.

## 3. Reuse, not reimplementation

The Lua token bucket already exists in `starter-go-redis/experimental`
(atomic refill+consume EVAL, hash state, `ratelimit:` key namespace,
burst/rate TTL, unlimited pass-through). Duplicating it here would fork the
script. This module is therefore a thin Contributor starter: config in,
`resilience.LimiterDriver` bean + registry entry out. The trade-off is a
dependency on starter-go-redis's experimental subpackage; if that package ever
moves, this starter follows it.

## 4. Wiring

Per `spring.ratelimit.redis.<name>` (gs.OnProperty + conf.BindEach):

- `client` (required, fail-fast): the starter-go-redis bean name; injected via
  `gs.TagArg(client)` — the same by-name seam starter-session-redis uses.
- `driver` (optional): registry name, defaults to the instance name.

The bean ctor calls `driverFor(name, client)`, which keeps one `*Driver`
singleton per name in a package-level `sync.Map`. Registration with
`resilience.RegisterLimiter` happens once per name per process (the registry
panics on duplicates); re-wiring the container — a second `gs.RunTest` in the
same test binary — merely rebinds the client. The bean is
`Export(gs.As[resilience.LimiterDriver]())` so it is root-reachable and its
ctor (the registration side effect) always runs in prod.

## 5. Semantics inherited from the Lua bucket

- One EVAL per Allow/AllowN: read hash → refill by elapsed ms → consume if
  enough → write hash → EXPIRE ceil(burst/rate)+1s. No client-side locking,
  so N replicas racing on one key still grant exactly the burst.
- `AllowN` is all-or-nothing inside the script.
- Keys are `ratelimit:<key>`; independent keys are independent budgets.
- Zero `Rate` short-circuits to allow without a Redis round-trip.
- Clock: the caller sends `now` in milliseconds; refill correctness depends on
  rough clock agreement between replicas (skew shifts fairness, never safety
  of the atomic consume).

## 6. Failure posture

Backend errors surface as `(false, err)` from Allow. This starter does not
decide open-vs-closed; consumers do (gateway fails open). Keeping the policy
out of the driver matches the "container assembles, not adjudicates" stance —
the limiter is a mechanism, callers choose the response.
