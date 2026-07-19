# starter-config-consul Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-config-consul` is a config-provider starter (`starter/DESIGN.md` §2.5)
in the integration layer: it makes Consul KV a remote configuration source for
Go-Spring at startup and at every property refresh. It sits between the core
container's provider mechanism (`spring/conf`) and the Consul HTTP API, and
holds no configuration state of its own.

## 1. Responsibilities & Boundaries

- Registers a `consul` provider name via `conf.RegisterProvider` in `init()`
  and nothing else at the package top level — no injectable bean, no server.
- Parses the provider source `consul:<host>:<port>/<kv-path>?<query>`, reads
  the KV path, parses the payload as `properties`/`yaml`/`toml`/`json`, and
  flattens it into a `map[string]string` for the framework to merge.
- Installs a blocking-query watcher (see §2) that reruns the provider on every
  KV change so live-bound `gs.Dync` fields update without a restart.
- Explicitly does **not** do service discovery. Consul is dual-purpose (config
  and catalog); the naming role belongs elsewhere per the "split by role"
  decision recorded in `starter/DESIGN.md` §2.5.

## 2. Key Abstractions & Seams

- **Provider seam.** `conf.RegisterProvider("consul", loadConsulConfig)` is the
  only extension point; the application consumes it via
  `spring.app.imports=[optional:]consul:...`. The provider runs during
  `AppConfig.Refresh`, before any bean exists, so it must build its own client
  from the source string rather than take a client through injection.
- **Client cache.** Consul clients are cached per
  `(address, scheme, token, datacenter)` tuple. Without this, every refresh
  would leak a client and its idle connections since `loadConsulConfig` runs
  on both startup and each `RefreshProperties`.
- **Refresh hook.** Provider-side state is an `atomic.Pointer[func() error]`
  populated by a container-scope bridge bean (`configRefreshBridge`) that
  injects `*gs.PropertiesRefresher`. The bridge is exported as `gs.Rooter` and
  named `consulConfigRefreshBridge` so it is always instantiated and never
  collides with the application's own default `__default__` Rooter (an alias
  for `any`).
- **Watch seam.** A single background goroutine per KV path runs a blocking
  query (`WaitIndex`, `WaitTime=5m`). Deduped via a `(client-key, kv-path)` set
  so repeat `Load` calls do not launch parallel watchers.

## 3. Constraints

- **Register the watcher before the read.** `registerWatch` is called before
  `KV().Get`, so hot-reload works even when the key does not yet exist under
  `optional:` — a later write triggers a refresh that reruns the provider and
  picks up the new value. Reversing the order is a silent regression.
- **Seed the wait index from the first response.** The watch loop treats the
  first successful poll as its baseline and only fires `triggerRefresh` on a
  subsequent `LastIndex` bump, so startup itself never causes a spurious
  refresh.
- **Handle backwards index.** Consul may return a `LastIndex` that has moved
  backwards after a state reset; the loop resets to `0` in that case per
  Consul's blocking-query guidance.
- **The bridge bean must be named.** `gs.Rooter` is an alias for `any`; two
  Rooter-exported beans under `__default__` collide via the
  `(name, type)` dedup on exports. The stable name `consulConfigRefreshBridge`
  is load-bearing.
- **No `go mod tidy` against the proxy.** `spring/*` and `stdlib/*` resolve
  through the workspace `go.work`; running tidy sends them to the module proxy
  and 404s.

## 4. Trade-offs / Alternatives Rejected

- **A mega-starter that also does discovery — rejected.** `config` and
  `discovery` live at different layers (provider registration vs. bean
  wiring); splitting by role mirrors Spring Cloud Alibaba's
  `nacos-config` / `nacos-discovery` and keeps the module graph clean.
- **Long-poll library / `github.com/hashicorp/consul/api/watch` — rejected
  for now.** The provider only needs "did the index change since last time",
  which a hand-rolled blocking `KV().Get` expresses in ~30 lines and keeps the
  watcher structurally identical to the etcd variant.
