# starter-config-vault Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-config-vault` is a config-provider starter (`starter/DESIGN.md` §2.5)
in the integration layer: it makes HashiCorp Vault a remote configuration
source for Go-Spring at startup and at every property refresh. It reads a KV
secret, exposes its fields as application properties, and pairs naturally with
the property-level decryption seam in `spring/conf/decrypt` — but the two are
independent.

## 1. Responsibilities & Boundaries

- Registers a `vault` provider name via `conf.RegisterProvider` in `init()`
  and nothing else at the package top level — no injectable bean, no server.
- Parses the provider source
  `vault:<host>:<port>/<mount>/<path>?kv-version=&namespace=&scheme=&key=&format=&prefix=&poll-ms=`,
  reads the secret through KV v1 or v2, and returns a flattened property map.
- Supports two modes:
  - **Whole-secret mode** (default): every field of the KV data becomes a
    property.
  - **Single-field mode** (`?key=...`): one field is treated as a document and
    parsed by `format`.
- Optionally prefixes emitted keys via `?prefix=<ns>`.
- Installs a polling watcher that reruns the provider when the secret's data
  fingerprint changes.

## 2. Key Abstractions & Seams

- **Provider seam.** `conf.RegisterProvider("vault", loadVaultConfig)` is the
  only extension point; the application consumes it via
  `spring.app.imports=[optional:]vault:...`. The provider runs during
  `AppConfig.Refresh`, before any bean exists.
- **Token resolution is out-of-band.** Order: `?token=` query (discouraged) →
  `VAULT_TOKEN` env → token file from `?token-file=` or `VAULT_TOKEN_FILE`.
  This keeps the token out of any configuration file the app itself binds.
- **Client cache.** Vault API clients are cached per
  `(address, namespace, token)` tuple so refreshes do not rebuild clients.
- **Refresh hook.** Container-scope bridge bean `configRefreshBridge` (named
  `vaultConfigRefreshBridge`, exported as `gs.Rooter`) injects
  `*gs.PropertiesRefresher` and stores its `RefreshProperties` into an
  `atomic.Pointer[func() error]`.
- **Watch seam — shared fingerprint.** Per `(client, mount, path)` there is a
  shared `loadedFP[watchKey]` string. Every successful `loadVaultConfig`
  writes it; the polling loop compares its own poll's fingerprint against it.
  When they differ, `triggerRefresh` reruns the provider, which updates the
  fingerprint again, so a change fires exactly once.

## 3. Constraints

- **Register the watcher before the read.** `registerWatch` runs before the
  secret is fetched, so hot-reload works even when the secret does not exist
  yet under `optional:`.
- **Do not seed the watcher's baseline from its own first poll.** The naive
  design — first poll seeds the baseline — silently drops the "optional
  secret created after startup" case: the first poll would already read the
  new value and treat it as baseline. Basing the watcher on the shared
  `loadedFP` (i.e. what the application actually loaded) fixes this.
- **Startup does not fire a spurious refresh.** `loadVaultConfig` seeds
  `loadedFP` before returning, so the first poll compares to that seed
  rather than to zero.
- **Vault tokens never live in bound properties.** The token is deliberately
  resolved from env / token file / query string, not from
  `spring.config.vault.*`, so a decryption seam that itself reads properties
  cannot enter a chicken-and-egg loop.
- **The bridge bean must be named.** Same rule as the other config-provider
  starters: `gs.Rooter` is an alias for `any`, so a stable name
  (`vaultConfigRefreshBridge`) is required to avoid `__default__`
  collisions.

## 4. Trade-offs / Alternatives Rejected

- **Vault Agent / CSI-mounted files — served by `starter-config-file`
  instead.** That starter already covers the file-mount case; this starter
  talks to the Vault API directly for cluster-side use cases (dynamic
  secrets, non-file mounts, per-request auth).
- **Push notifications — Vault has no native push.** Polling is the only
  option, so the design puts effort into "poll cheaply and detect exactly
  once" rather than into a fake push interface.
