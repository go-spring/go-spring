# starter-config-apollo Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

A config-provider starter (starter/DESIGN.md §2.5): registers an `apollo`
provider under `spring.config.import`, mirroring starter-config-nacos's shape.

## 1. Responsibilities & Boundaries

- **Owns**: the provider registration, source parsing, agollo client cache,
  change-listener→refresh bridge, and the namespace→properties parsing.
- **Does not own**: governance.Source integration (a future adapter, see
  README), and the Apollo admin/portal stack.

## 2. Key Abstractions & Seams

- **conf.RegisterProvider("apollo", ctrl.Load)** — the controller is not a
  bean, same as nacos: on a change the listener callback calls the
  process-level `gs.RefreshProperties()` facade, and Load serves config
  fetches. No bound Config — connection params live in the source string.
- **clientFor cache** — one agollo Client per `(server, appId, cluster,
  secret, namespace)`; namespace is in the key because agollo fixes
  `NamespaceName` at StartWithConfig time.
- **Listener-before-fetch** — the load-bearing invariant shared with nacos.

## 3. Constraints

- agollo v4's wire uses `/configfiles/json/...` (raw JSON object, not the
  ApolloConfig envelope) for the initial sync; the starter relies on agollo's
  own parsing, not its own.
- `appId` is required in the source (agollo cannot fetch without it).

## 4. Trade-offs / Alternatives Rejected

- **agollo v4 vs Apollo OpenAPI**: agollo is the official Go SDK with the
  change-notification long-polling already built; the OpenAPI would mean
  hand-rolling the notification loop.
- **Cold-load-only example vs dockerized quick-start**: the quick-start needs
  a MySQL + configservice/admin/portal trio; the starter's contract is the
  provider seam, so a mock config service exercises it end-to-end without the
  stack. A real `docker pull apolloconfig/apollo-quick-start` was attempted
  (2026-08-26); the registry is network-unreachable from this environment
  (`registry-1.docker.io ... i/o timeout`), so no compose file is provided and
  `check.sh` runs the self-contained mock smoke instead.

## 5. Client Choice

**agollo v4 (github.com/apolloconfig/agollo/v4, v4.4.0) over a hand-rolled
net/http client.** agollo is the community-recommended Apollo Go client with
the notifications/v2 long-polling loop, in-memory namespace cache, local-file
backup and secret signing already built. Reimplementing
`/configs/{appId}/{cluster}/{ns}` plus `/notifications/v2` on the standard
library was the recorded fallback, to be used only if agollo could not be
fetched from GOPROXY — it could, so the fallback stays unused. The one
concession: agollo fixes `NamespaceName` at client creation, hence the
namespace in the client cache key (§2).
