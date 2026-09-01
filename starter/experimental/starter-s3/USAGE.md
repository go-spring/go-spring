# starter-s3 Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `config.go`, `driver.go`, `client.go`,
`command.go`, `health/health.go`, `s3_test.go`) and the runnable [example/](example/)
(smoke-verified against MinIO via docker-compose). **Object-storage operation semantics
(bucket/object APIs, retention, versioning) are [minio-go's documentation](https://github.com/minio/minio-go)**
— everything below is go-spring's increment: configuration, wiring, fail-fast startup,
per-request observability, resilience, health.

**Activation**: `gs.OnProperty("spring.s3")` gates a gs.Module; every `spring.s3.<name>`
subtree creates one `*Client` bean named `<name>` plus one health indicator named
`s3:<name>`. No keys → no beans.

The starter speaks the S3 protocol through **minio-go**, so one config surface covers MinIO
and AWS S3 natively and the S3-compatible endpoints of other clouds (Aliyun OSS, Tencent
COS, ...) — see README compatibility notes.

---

## 1. Complete worked project

A service storing objects in MinIO with health probes, per-request observability and
governance-guarded access. File tree (mirrors the smoke-verified [example/](example/)):

```
demo/
├── go.mod
├── main.go
├── storage.go
└── conf/
    └── app.properties
```

**go.mod** (module deps that matter):

```
require (
    github.com/minio/minio-go/v7   latest   # transitive, pulled by the starter
    go-spring.org/spring           v1.3.x
    go-spring.org/starter-s3       latest
    go-spring.org/starter-actuator latest   # optional: readiness endpoint
    go-spring.org/starter-otel     latest   # optional: real span/metric export
    go-spring.org/starter-governance latest # optional: retry/limiter/breaker/fault
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-governance"
    _ "go-spring.org/starter-otel"
    _ "go-spring.org/starter-s3"
)

func main() { gs.Run() }
```

**storage.go** — the application's entire storage surface:

```go
package storage

import (
    "context"

    "github.com/minio/minio-go/v7"
    "go-spring.org/spring/gs"

    starter "go-spring.org/starter-s3"
)

// Service injects one named client. Client embeds *minio.Client, so every
// generated method (PutObject, GetObject, StatObject, ...) promotes unchanged.
// A second endpoint is a pure config change plus another field.
type Service struct {
    Client *starter.Client `autowire:"a"`
}

func init() {
    gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())
}

// Put demonstrates one upload; the observability/resilience transport sits
// inside the client, so this call is already spanned, metered, logged and
// governance-guarded (see §2.3).
func (s *Service) Put(ctx context.Context, bucket, key string, b []byte) error {
    _, err := s.Client.PutObject(ctx, bucket, key,
        bytes.NewReader(b), int64(len(b)), minio.PutObjectOptions{ContentType: "text/plain"})
    return err
}
```

**conf/app.properties** — the complete, commented surface (example config extended):

```properties
# --- s3 client "a" (one instance per spring.s3.<name> subtree) ----------------
spring.s3.a.endpoint=127.0.0.1:9000        # host:port, no scheme
spring.s3.a.access-key-id=minioadmin
spring.s3.a.secret-access-key=minioadmin
spring.s3.a.region=us-east-1
spring.s3.a.use-ssl=false
# path style keeps the demo friendly to S3-compatible clouds that reject
# virtual-host addressing.
spring.s3.a.bucket-lookup=path

# a second client against the same cluster shows multi-instance wiring
spring.s3.b.endpoint=127.0.0.1:9000
spring.s3.b.access-key-id=minioadmin
spring.s3.b.secret-access-key=minioadmin

# per-request observability: configure it per instance under the instance
# prefix (preferred); top-level observability.* still works as a fallback
# (instance keys override it — see §3).
spring.s3.a.observability.level=brief
# spring.s3.a.observability.maxArgBytes=512
# spring.s3.a.observability.skipOps=GET /bucket/x

# --- actuator (folds the s3:<name> indicators into readiness) -----------------
spring.actuator.addr=:9370

# --- governance (optional: retry/limiter/breaker/fault under s3:<endpoint>) ---
govern.source.file.path=conf/govern.yaml
```

Local dependency: `cd example && docker compose up -d` (MinIO :9000/:9001, plus an
`initbucket` job that creates `go-spring-example`).

**Verify** (isomorphic to example/check.sh's assertions):

```bash
go run .                                                  # example prints "Object round trip OK:"
curl -i :9370/readyz ; curl -s :9370/readyz | grep -o 's3:a[^,}]*'   # indicator health
curl -s :9370/metrics | grep -E 's3.*client|duration' | head        # per-request metrics (with otel)
mc ls local/go-spring-example                              # via the compose network, if mc present
```

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-s3
  └─ init(): gs.Module(gs.OnProperty("spring.s3"), ...) — a Module (not gs.Group) so each
        instance's *Client can be PAIRED with a health.Indicator under the same name
        (source comment, starter.go:32-36)

gs.Run()
  ├─ conf.BindEach over spring.s3.* → one Config per instance name
  ├─ per instance:
  │    ├─ r.Provide(newClient, IndexArg(1, ValueArg(c))).Name(name)
  │    │      .Init((*Client).Init).Destroy((*Client).Destroy).Caller(1)
  │    ├─ r.Provide(health indicator).Name("s3:"+name).Export(health.Indicator)
  │    │      — .Name is what keeps multi-instance (Name,Type) keys unique
  │    ├─ newClient: driverRegistry lookup (default DefaultDriver)
  │    │      → DefaultDriver.CreateClient: static creds + region + bucket-lookup
  │    │        + a dynamicTransport placeholder inside minio.Options
  │    │      → dynamicTransports.LoadAndDelete hands the placeholder to the wrapper
  │    ├─ fail-fast probe: HealthCheck → ListBuckets — unreachable endpoint or rejected
  │    │      credentials abort startup (starter.go:76-78)
  │    └─ gs field-injects Client.Observability, then Init():
  │          obs := observe.NewDB("s3", ...) → obsTransport (span+metric+access log)
  │          exec := fault.WrapExecutor(resilience.ExecutorFor("s3:<endpoint>"))
  │          exec := resilience.WrapExecutor(exec, "s3", ...)  // outcome spans/counter
  │          dyn.Swap(resilience.NewRoundTripper(obsTransport, exec, → resource))
  ├─ Run / serve: readyz folds in every s3:<name> indicator (needs starter-actuator)
  └─ SIGTERM: Destroy() closes the resilience executor; minio holds no session to close
```

### 2.2 Why the dynamicTransport exists (rationale from source)

minio-go fixes the `http.Transport` inside `minio.Options` at construction and exposes no
setter, while the observability policy is only field-injected **after** the client exists.
`DefaultDriver.CreateClient` therefore installs a thin `dynamicTransport` — an atomic
RoundTripper indirection (RWMutex-guarded, not atomic.Value, because the active tripper is
one of several concrete types; see client.go:104-114) — and records it in a package-level
`dynamicTransports sync.Map` keyed by the returned client. `newClient` picks it up
(`LoadAndDelete`) so `Init` can swap the real observe+resilience transport in. Until Init
runs, requests pass straight through to `http.DefaultTransport`.

### 2.3 One upload, layer by layer

`PutObject(ctx, bucket, key, ...)`:

1. minio-go signs the request (SigV4 static credentials) and issues the HTTP request through
   the configured transport — which is the swapped-in resilience round-tripper.
2. Resilience round-tripper: the request enters the executor resolved for resource
   `s3:<endpoint>` — retry / rate-limit / circuit-breaker / bulkhead when starter-governance
   arms them (hot-reloadable through the governance center), transparent pass-through
   otherwise; the process-wide fault injector (`fault.InjectorFor`, nil-safe) may inject
   failures for drills. `resilience.WrapExecutor` emits an outcome span + call counter +
   duration histogram + access log for breaker trips, limit rejects, bulkhead rejections.
3. obsTransport (command.go:38): starts the per-request observer span with operation
   `"PUT /bucket/key"` (method + URL path), runs the base `http.DefaultTransport`, ends the
   span with the error — the span + duration metric + access log all carry that operation
   name (minio-go ships no OTel hooks of its own, so the starter's transport carries all
   three signals).
4. The response unwinds: span attributes/metrics recorded, log line emitted per the
   observability level; minio-go returns the object info to the caller.

### 2.4 Health

`NewClientHealth` (health/health.go:33) probes with `ListBuckets` — it verifies both endpoint
reachability **and** that the credential pair is accepted. Registered per instance under
`s3:<name>` and exported as `health.Indicator`, so an app importing starter-actuator gets S3
readiness folded into `/readiness` with no extra wiring. The same function is exported as
`StarterS3.HealthCheck(ctx, *Client) error` for ad-hoc probing.

---

## 3. Per-key behavior reference

Prefix `spring.s3.<name>.*` for the ctor-bound `Config` keys (config.go). `observability`
also exists as a top-level `observability.*` fallback field on the Client wrapper
(instance keys override it; binding fills the defaults brief/512/no-skips even when unset,
so only a non-default instance value counts as "set" — see §6).

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `endpoint` | string | — | **Required** (`expr:"$ != ''"`). `host:port` without scheme. | Missing/empty → bind-time validation error at startup. |
| `access-key-id` | string | — | **Required**; static SigV4 credential. | Missing → startup error; wrong → the ListBuckets fail-fast probe rejects boot. |
| `secret-access-key` | string | — | **Required**; static SigV4 credential. | Same as above. |
| `session-token` | string | — | Optional third element (temporary credentials). | Permanent creds + stale token → signature rejected at the boot probe. |
| `region` | string | us-east-1 | Bucket region passed to minio.Options. | Wrong region → signature/redirect errors on region-aware endpoints (may pass the probe against region-agnostic MinIO, then fail per-bucket). |
| `use-ssl` | bool | false | HTTPS towards the endpoint. | false against an TLS-only endpoint (or true against plaintext) → boot probe fails. |
| `bucket-lookup` | string | auto | `auto` \| `virtual-host`/`dns` (aliases, `BucketLookupDNS`) \| `path`. | Some S3-compatible clouds only serve path style → wrong style yields per-request addressing failures. Unknown value → startup error listing valid values. |
| `observability` | struct | brief | Level of per-request span+metric+access log (`spring.s3.<name>.observability.level` / `.maxArgBytes` (default 512) / `.skipOps`); top-level `observability.*` is the fallback surface, overridden per field by instance keys. | Instance value set back to its default (e.g. `level=brief`) cannot override a non-default top-level value — configure one surface only. |
| `driver` | string | DefaultDriver | Selects a driver from the registry (`RegisterDriver`). Unknown name → startup error "s3 driver not found". | Custom drivers skip the dynamicTransport handshake → resilience unavailable for that client (observe still binds via the wrapper field). |

---

## 4. Verification & fault drills

### 4.1 Object round-trip drill (same path as example/check.sh)

```bash
cd example && docker compose up -d && go run .   # asserts put→read→stat→remove, prints marker
curl -s :9370/readyz | grep -o '"s3:a[^"]*":[^,}]*'   # indicator UP (with actuator)
```

The example self-asserts `bytes.Equal(got, content)` after GetObject — any transport-level
corruption fails the run.

### 4.2 Fail-fast drill

Set `spring.s3.a.secret-access-key=wrong`: boot aborts with `failed to reach s3 endpoint ...
(the request signature we calculated does not match ...)`. The probe (ListBuckets) exists so
credential/endpoint mistakes never reach first use.

### 4.3 Health drill

Stop MinIO (`docker compose stop minio`) while the app runs: `curl :9370/readyz` flips the
`s3:a` component to DOWN (probe = ListBuckets). Restart and it recovers — the indicator is
per-request, not latched.

### 4.4 Observability drill

With `spring.s3.a.observability.level=detailed` and starter-otel imported, generate one upload
and read the three signals: a span named `PUT /go-spring-example/hello.txt`, the duration
histogram, and the per-request access log line. Add
`spring.s3.a.observability.skipOps=PUT /go-spring-example/hello.txt` and repeat: that
operation disappears from the log while others remain — proving the instance-prefixed
resolution path (top-level `observability.*` acts as the fallback surface).

### 4.5 Fault/resilience drill (needs starter-governance)

Arms per endpoint resource label `s3:127.0.0.1:9000`: a governance rule with
`fault.rate` against that resource makes a fraction of uploads fail through the executor —
observable as outcome-tagged spans/counters from `resilience.WrapExecutor`. Flip the rule
file to withdraw (hot-reload through the governance source). ⚠ note the retry policy retries
per round-trip, not per stream: uploads with large bodies may re-send the body.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Startup aborts "failed to reach s3 endpoint" | endpoint down, wrong port, `use-ssl` mismatch, or bad credentials | The probe error carries the underlying cause (signature mismatch ⇒ creds; connection refused ⇒ endpoint/ssl). |
| Startup aborts "s3 driver not found: X" | `driver` names nothing registered | Register via `RegisterDriver` in an init() before use, or drop the key. |
| Startup aborts "unknown bucket-lookup" | invalid style string | One of auto / virtual-host / dns / path. |
| Top-level `observability.*` ignored | instance-prefixed `spring.s3.<name>.observability.*` set to a non-default value overrides it per field | Clear one of the two surfaces, or set the instance key to the desired value. |
| Custom driver client has no resilience | dynamicTransport handshake only exists for DefaultDriver | Accept observe-only, or install your own indirection in the driver. |
| Works against MinIO, 404/redirect on cloud X | virtual-host addressing not supported there | `bucket-lookup=path`. |
| readyz DOWN though app works | credential rotation invalidated the pair after boot | The indicator probes live; refresh credentials / restart. |
| Two instances → container duplicate-bean error on health | (historical) indicator registered without `.Name` | Current code registers `s3:<name>` — keep that pattern when copying it for your own starters. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 9 (+3 top-level observability.* shared) |
| Required | 3 (endpoint, access-key-id, secret-access-key) |
| Quickstart external deps | 1 (MinIO / any S3 endpoint) |
| "Watch out" entries | 5 |

Design suspects (for the audit ledger; first two carried over from the previous edition):
- `driver` key exists but only one driver (DefaultDriver) ships in-repo — speculative
  extension point; the dynamicTransport handshake between driver and wrapper is implicit
  (keyed off a sync.Map).
- `bucket-lookup` accepts both "virtual-host" and "dns" aliases for one mode — mild config
  surface redundancy.
- FIXED (was: instance-scoped `observability` key dead): `Init` now resolves the
  observability policy via `resolveObservability` — instance-prefixed
  `spring.s3.<name>.observability.*` (bound into `Config.Observability`) overrides the
  top-level wrapper field per field. Residual caveat: binding fills defaults even when
  unset, so an instance value equal to a default cannot override a non-default top-level
  value.
- NEW: resource label is `s3:<endpoint>` only — two instances on one endpoint (like the
  example's `a`/`b`) share one resilience scope; no per-instance disambiguation.
- NEW: health probe and fail-fast probe are the same ListBuckets call but duplicated in code
  (starter.go HealthCheck vs health/health.go) — harmless but a small consolidation target.
