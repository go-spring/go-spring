# starter-s3 Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

A Client-archetype starter (`starter/DESIGN.md` §2.2) speaking the S3
protocol through minio-go. Structurally it mirrors starter-elasticsearch
(the other HTTP-transport client starter); the differences are called out
below.

## 1. Responsibilities & Boundaries

- **Owns**: bean lifecycle for the `spring.s3.instances.<name>` group (multi-instance,
  container-managed teardown), the fail-fast `ListBuckets` probe, the
  per-instance health indicator, the declaration transport (the resilience layer
  emits the 3 signals), and the resilience round-tripper on the client's HTTP
  transport.
- **Does not own**: bucket/object administration policy, credential rotation
  (static keys only — rotate by config), vendor features outside the S3
  facade (custom-driver territory), and bucket discovery (the endpoint names
  one service; no `service-name`/discovery quartet).

## 2. Key Abstractions & Seams

- **`Client` wrapper bean** — embeds the raw `*minio.Client`, so its whole SDK
  method surface promotes unchanged (`NewClient` is the only constructor). minio fixes the
  transport inside `minio.Options` at construction, so DefaultDriver installs a
  `dynamicTransport` (RWMutex-guarded RoundTripper indirection) and the wrapper
  swaps the declaration+resilience transport into it — the same mechanism
  starter-elasticsearch uses for the same reason.
- **`Driver` (driver.go)** — construction seam: the optional container-bean
  `Driver` interface + the bundled `DefaultDriver` that assembles credentials
  (`NewStaticV4`), region, bucket-lookup style and the dynamic transport. A
  company/umbrella starter provides its own `Driver` bean; when none is present
  the starter falls back to `DefaultDriver`. The bucket-lookup config string
  maps onto minio's `BucketLookupType` (`virtual-host` is an alias of
  `BucketLookupDNS` in minio v7.0.74).
- **declareTransport (command.go)** — per-request declaration seam. It puts the
  request's identity on the context (`observability.WithOperation`), which the
  resilience layer inside it reads to emit the span, the metrics and the access
  log. It sits OUTSIDE the resilience round-tripper on purpose: the emitter reads
  the operation at `Execute` entry, so a declaration nested inside the executor
  would run per attempt and be read by nobody. The vocabulary (observe.go) is
  `db.system=s3`, `db.operation=<HTTP method>` (bounded, as metric labels) and
  `db.statement=<URL path>` (unbounded, detail only — span + log, never a label)
  under the `_app_s3_access` access log.
- **Resilience** — `resilience.NewRoundTripper` wraps `http.DefaultTransport`
  with the executor assembled by
  `fault.WrapClientExecutor(mgr.ClientExecutorFor("s3", service), service, inj)` from the injected
  `*resilience.Manager` / `*fault.Injector` beans, scoped by
  `resilience.ServiceLabel("s3", endpoint)`.

## 3. Constraints

- Credentials are required (`expr` on both keys): every S3 deployment in
  practice authenticates, and anonymous access is better served by a custom
  driver than by a nullable config surface.
- The fail-fast probe doubles as the health check — `ListBuckets` is the
  cheapest call that verifies both reachability and credentials without
  side effects.
- No `Close` on the minio client; Destroy only closes the resilience
  executor.
- minio-go is pinned to **v7.0.74** (the version already mirrored in the
  shared module cache); later 7.x versions add `BucketLookupVirtualHost` as a
  named constant but the alias mapping keeps the config surface stable.

## 4. Trade-offs / Alternatives Rejected

- **S3 protocol as the abstraction vs. per-vendor starters**: the S3 facade
  is the de-facto standard every major cloud exposes; a per-vendor starter
  family would multiply maintenance for near-zero semantic difference. Vendor
  specifics beyond the facade stay in custom drivers.
- **minio-go vs aws-sdk-go-v2**: minio-go is a single dependency with a
  stable, small API surface and native S3-compat awareness (bucket-lookup
  modes); aws-sdk-go-v2 is a large module graph with AWS-specific
  bootstrapping.
- **Static transport wrap at construction vs. dynamicTransport**: the real
  transport (declaration + resilience) is assembled by the wrapper's constructor,
  after minio has already fixed the transport; the indirection keeps the client
  usable (DefaultTransport passthrough) until it is swapped in, instead of
  failing or applying blind.
