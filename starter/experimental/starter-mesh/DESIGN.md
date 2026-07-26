# starter-mesh Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-mesh` is a global/infrastructure archetype starter (see
[starter/DESIGN.md](../DESIGN.md) §2.4) that flips the app-wide
service-mesh switch used by the client-side discovery and load-balancing
stack. It sits in the *starter* layer so the toggle stays wired at
integration time, while the actual degradation logic — and the single source
of truth — lives in `spring/discovery`.

## 1. Responsibilities & Boundaries

- **In scope:** bind `${spring.mesh}`, call `discovery.SetMeshMode(cfg.Enabled)`
  during `RefreshPrepare` so the switch is set before any bean is wired.
- **Out of scope:** control-plane objects (VirtualService, DestinationRule)
  and traffic-management APIs — those belong to deployment scaffolding, not
  this starter. Trace, metrics, and readiness semantics are also untouched;
  mesh mode only affects client-side dial and pick paths.

## 2. Key Decision — one switch, centralized degradation

The switch is a single process-global `atomic.Bool` in `spring/discovery`,
not per-starter branching. Every client-side seam consults it in exactly
two places:

- `discovery.NewClientDialer` / `NewLiveDialer`: mesh on → build a
  `meshDialer` that returns a single stable endpoint `{Addr:name, Healthy:true}`
  and never resolves, watches, or spins a background goroutine. Kubernetes
  DNS resolves the name to a Service ClusterIP that the sidecar intercepts.
- `loadbalance.Pool.Pick`: mesh on → return `eps[0]` directly with a no-op
  `Done`. The Tracker is bypassed on purpose: the single mesh endpoint must
  never be ejected as unhealthy — that would blackhole all traffic.

The starter itself contains no degradation logic; it is the wire that binds
config to `SetMeshMode`.

## 3. Constraints

- Applied via `gs.Module(nil, setup)` so `RefreshPrepare` runs before bean
  construction — the toggle must be visible to every client starter's
  constructor.
- Applied unconditionally, including when `enabled=false`, so a previous
  "on" state is cleared cleanly if the config is refreshed.
- Client starters must obtain their dialer via `discovery.NewClientDialer`
  (as `starter-go-redis` does) to honor the switch without touching the
  registry. Starters still calling `NewLiveDialer` directly also degrade,
  but continue to require a registered backend even in mesh mode.
- `go.mod` intentionally does not run `go mod tidy` — internal deps
  (spring, log, stdlib) resolve through `go.work`; tidy would 404 on the
  proxy.

## 4. Trade-offs / Alternatives Rejected

- **Per-starter mesh flags — rejected.** N knobs to keep in sync, and the
  cross-starter coordination bug fixed by centralizing is real (redis and
  gorm both consulting the same switch means one flip is enough).
- **Control-plane integration — rejected here.** VirtualService generation
  belongs to `gs k8s` deploy scaffolding; conflating the two would drag
  Kubernetes API deps into every client starter.
- **Delete the LB code when mesh is on — rejected.** Degradation is
  runtime-only; flipping the switch off restores full client-side
  discovery + load-balancing with no rebuild.

## 5. When to enable

- **Enable** when the pod runs an Istio/Envoy or Linkerd sidecar — otherwise
  the app double-balances on top of the sidecar and confuses locality and
  outlier ejection.
- **Disable** (the default) on VMs, bare-metal, and non-mesh Kubernetes;
  keep client-side discovery+LB as the primary path.
