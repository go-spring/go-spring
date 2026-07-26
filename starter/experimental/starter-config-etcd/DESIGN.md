# starter-config-etcd Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-config-etcd` is a config-provider starter (`starter/DESIGN.md` §2.5)
in the integration layer: it makes etcd a remote configuration source for
Go-Spring at startup and at every property refresh. It sits between the core
container's provider mechanism (`spring/conf`) and the etcd v3 client, and
holds no configuration state of its own.

## 1. Responsibilities & Boundaries

- Registers an `etcd` provider name via `conf.RegisterProvider` in `init()`.
  No injectable bean, no server.
- Parses the provider source `etcd:<host>:<port>/<key>?<query>`, reads the
  key, parses the payload as `properties`/`yaml`/`toml`/`json`, and flattens it
  into a `map[string]string` for the framework to merge.
- Installs an etcd `Watch` on the key so a subsequent put reruns the provider
  and live-updates every bound `gs.Dync` field.
- Explicitly does **not** do service discovery. etcd is dual-purpose; the
  naming role belongs elsewhere per the "split by role" decision recorded in
  `starter/DESIGN.md` §2.5.

## 2. Key Abstractions & Seams

- **Provider seam.** `conf.RegisterProvider("etcd", loadEtcdConfig)` is the
  only extension point; the application consumes it via
  `spring.app.imports=[optional:]etcd:...`. The provider runs during
  `AppConfig.Refresh`, before any bean exists, so it must build its own
  client from the source string rather than take one through injection.
- **Client cache.** `clientv3.Client`s are cached per
  `(endpoint, username, password)` tuple. Without this, every refresh would
  leak a client and its background goroutines since `loadEtcdConfig` runs on
  both startup and each `RefreshProperties`.
- **Refresh hook.** Provider-side state is an `atomic.Pointer[func() error]`
  populated by a container-scope bridge bean (`configRefreshBridge`) that
  injects `*gs.PropertiesRefresher`. Exported as `gs.Rooter` and named
  `etcdConfigRefreshBridge` so it is always instantiated and never collides
  with the application's own default `__default__` Rooter.
- **Watch seam.** One `cli.Watch` channel per key, consumed in a background
  goroutine that fires `triggerRefresh` for every non-empty event batch.
  Deduped via a `(client-key, etcd-key)` set so repeat `Load` calls do not
  register parallel watchers.

## 3. Constraints

- **Register the watcher before the read.** `registerWatcher` runs before
  `cli.Get`, so hot-reload works even when the key does not yet exist under
  `optional:` — a later put triggers a refresh that reruns the provider and
  picks up the new value. Reversing the order is a silent regression.
- **Watch is best-effort.** If the watch channel closes with an error the
  goroutine exits; this loses hot-reload for that key but does not block
  startup. The initial fetch has already produced a value.
- **`optional:` swallows Get errors, not parse errors.** A network failure or
  missing key returns `(nil, nil)` when optional; a decode error is always
  fatal so a mistyped format surfaces immediately.
- **The bridge bean must be named.** `gs.Rooter` is an alias for `any`; two
  Rooter-exported beans under `__default__` collide via the `(name, type)`
  dedup on exports. The stable name `etcdConfigRefreshBridge` is
  load-bearing.
- **No `go mod tidy` against the proxy.** `spring/*` and `stdlib/*` resolve
  through the workspace `go.work`.

## 4. Trade-offs / Alternatives Rejected

- **A mega-starter that also does discovery — rejected.** `config` and
  `discovery` live at different layers; splitting by role mirrors Spring
  Cloud Alibaba and keeps the module graph clean.
- **Prefix watch (`clientv3.WithPrefix`) — deliberately not exposed.** A
  provider source targets a single key holding a config document; multi-key
  fan-out is left to the application, keeping the mental model identical to
  the Consul and Nacos variants.
