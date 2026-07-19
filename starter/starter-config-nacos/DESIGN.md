# starter-config-nacos Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-config-nacos` is a config-provider starter (`starter/DESIGN.md` §2.5)
in the integration layer: it makes Nacos Config a remote configuration source
for Go-Spring at startup and at every property refresh. It sits between the
core container's provider mechanism (`spring/conf`) and the Nacos SDK, and
holds no configuration state of its own.

## 1. Responsibilities & Boundaries

- Registers a `nacos` provider name via `conf.RegisterProvider` in `init()`
  and nothing else at the package top level — no injectable bean, no server.
- Parses the provider source
  `nacos:<host>:<port>/<dataId>?group=&namespace=&format=&username=&password=&timeout-ms=`,
  fetches the data id, parses it as `properties`/`yaml`/`toml`/`json`, and
  flattens it for the framework to merge.
- Installs `ListenConfig` on the `(group, dataId)` so a remote publish reruns
  the provider and live-updates every bound `gs.Dync` field.
- Explicitly does **not** do service discovery. Nacos naming is a separate
  concern per the "split by role" decision recorded in `starter/DESIGN.md`
  §2.5.

## 2. Key Abstractions & Seams

- **Provider seam.** `conf.RegisterProvider("nacos", loadNacosConfig)` is the
  only extension point; the application consumes it via
  `spring.app.imports=[optional:]nacos:...`. The provider runs during
  `AppConfig.Refresh`, before any bean exists, so it builds its own SDK
  client from the source string rather than take one through injection.
- **Client cache.** Nacos SDK clients are cached per connection tuple. Without
  this, every refresh would leak a client and its background gRPC connections.
- **Refresh hook.** Provider-side state is an `atomic.Pointer[func() error]`
  populated by a container-scope bridge bean (`configRefreshBridge`) that
  injects `*gs.PropertiesRefresher`. Exported as `gs.Rooter` and named
  `nacosConfigRefreshBridge` so it is always instantiated and never collides
  with the application's own default `__default__` Rooter.
- **Listener seam.** `ListenConfig` deduped per `(client, group, dataId)`
  triple so repeat `Load` calls do not register duplicate listeners.

## 3. Constraints

- **Register `ListenConfig` before `GetConfig`.** This is the single most
  important invariant. If the listener is only registered after a successful
  `GetConfig`, an `optional:` import against a data id that does not yet
  exist returns early and never installs the listener — a later publish
  never triggers a refresh. The provider therefore installs the listener
  unconditionally before the fetch.
- **The bridge bean must be named.** `gs.Rooter` is an alias for `any`; two
  Rooter-exported beans under `__default__` collide via the `(name, type)`
  dedup on exports. The stable name `nacosConfigRefreshBridge` is
  load-bearing.
- **Content parsers reuse `spring/conf/reader/*`.** The reader packages do
  not expose a "read bytes as format" helper; the provider imports the
  concrete `Read` functions and keys them by format name.
- **No `go mod tidy` against the proxy.** `spring/*` and `stdlib/*` resolve
  through the workspace `go.work`.

## 4. Trade-offs / Alternatives Rejected

- **A mega-starter that also does discovery — rejected.** `config` and
  `discovery` live at different layers; splitting by role mirrors Spring
  Cloud Alibaba's `nacos-config` / `nacos-discovery`.
- **Sharing the app's Nacos SDK client via IoC — rejected.** The provider
  runs before the container exists; a shared bean would be constructed after
  the point where it needs to be used. The connection tuple cache gives an
  equivalent "one client per endpoint" outcome without the ordering hazard.
