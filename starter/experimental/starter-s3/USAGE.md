# starter-s3 Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `config.go`, `driver.go`, `client.go`,
`command.go`, `health.go`, `s3_test.go`) and the runnable [example/](example/)
(smoke-verified against MinIO via docker-compose). **Object-storage operation semantics
(bucket/object APIs, retention, versioning) are [minio-go's documentation](https://github.com/minio/minio-go)**
— everything below is go-spring's increment: configuration, wiring, opt-in fail-fast startup,
per-request instrumentation, resilience, health.

**Activation**: `gs.OnProperty("spring.s3")` gates a gs.Module; every `spring.s3.instances.<name>`
subtree creates one `*Client` bean named `<name>` plus one health indicator named
`s3:<name>`. No keys → no beans.

The starter speaks the S3 protocol through **minio-go**, so one config surface covers MinIO
and AWS S3 natively and the S3-compatible endpoints of other clouds (Aliyun OSS, Tencent
COS, ...) — see README compatibility notes.

---

## 1. Complete worked project

A service storing objects in MinIO with health probes, per-request instrumentation and
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
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
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

// Service injects one named client. *Client is the whole surface: it embeds the
// raw *minio.Client, so every SDK method (PutObject, GetObject, StatObject, ...)
// is promoted. A second endpoint is a pure config change plus another field.
type Service struct {
    Client *starter.Client `autowire:"a"`
}

func init() {
    gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())
}

// Put demonstrates one upload; the declaration/resilience transport sits inside
// the client, so this call is already declared, spanned, metered, logged and
// governance-guarded (see §2.3).
func (s *Service) Put(ctx context.Context, bucket, key string, b []byte) error {
    _, err := s.Client.PutObject(ctx, bucket, key,
        bytes.NewReader(b), int64(len(b)), minio.PutObjectOptions{ContentType: "text/plain"})
    return err
}
```

**conf/app.properties** — the complete, commented surface (example config extended):

```properties
# --- s3 client "a" (one instance per spring.s3.instances.<name> subtree) ----------------
spring.s3.instances.a.endpoint=127.0.0.1:9000        # host:port, no scheme
spring.s3.instances.a.access-key-id=minioadmin
spring.s3.instances.a.secret-access-key=minioadmin
spring.s3.instances.a.region=us-east-1
spring.s3.instances.a.use-ssl=false
# path style keeps the demo friendly to S3-compatible clouds that reject
# virtual-host addressing.
spring.s3.instances.a.bucket-lookup=path

# a second client against the same cluster shows multi-instance wiring
spring.s3.instances.b.endpoint=127.0.0.1:9000
spring.s3.instances.b.access-key-id=minioadmin
spring.s3.instances.b.secret-access-key=minioadmin

# --- actuator (folds the s3:<name> indicators into readiness) -----------------
spring.actuator.addr=:9370

# --- governance (optional: retry/limiter/breaker/fault under s3:<endpoint>) ---
spring.governance.source.file.path=conf/governance.yaml
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
  └─ init(): gs.Module(gs.OnProperty("spring.s3.instances"), ...) — a Module (not gs.Group) so each
        instance's *Client can be PAIRED with a health.Indicator under the same name

gs.Run()
  ├─ conf.BindEach over spring.s3.instances.* → one Config per instance name
  ├─ per instance:
  │    ├─ r.Provide(newClient, IndexArg(1, ValueArg(c)), IndexArg(2, ?Driver))
  │    │      .Name(name).Destroy((*Client).Destroy).Caller(1)
  │    ├─ r.Provide(health indicator).Name("s3:"+name).Export(health.Indicator)
  │    │      — .Name is what keeps multi-instance (Name,Type) keys unique
  │    ├─ newClient: optional Driver bean (none → bundled DefaultDriver;
  │    │      several coexist → the entry selects one by name:
  │    │      spring.s3.instances.<name>.driver = <bean-name>, empty = `spring.s3.default.driver`, then the single
  │    │      Driver bean by type, naming a missing bean fails startup)
  │    │      → d.CreateClient(cfg, cloud.ClientParams{Resilience: mgr, Fault: inj}):
  │    │        static creds + region + bucket-lookup + a dynamicTransport inside minio.Options
  │    │      → NewClient builds the *Client wrapper: identity, exec =
  │    │        params.ExecutorFor("s3", "s3:<endpoint>") (mgr/inj are the injected
  │    │        *resilience.Manager / *fault.Injector beans), then installs the transport —
  │    │        declaration outermost, resilience inside
  │    ├─ ping=true only: fail-fast probe HealthCheck(ctx, client) — one ListBuckets
  │    │      straight to the raw client; an unreachable endpoint or rejected credentials
  │    │      abort startup and release the client (Destroy)
  ├─ Run / serve: readyz folds in every s3:<name> indicator (needs starter-actuator)
  └─ SIGTERM: Destroy() closes the resilience executor; minio holds no session to close
```

### 2.2 Why the dynamicTransport exists (rationale from source)

minio-go fixes the `http.Transport` inside `minio.Options` at construction and exposes no
setter, while the real transport (declaration + resilience) is assembled by the wrapper
**after** the client exists.
`DefaultDriver.CreateClient` therefore installs a thin `dynamicTransport` — an atomic
RoundTripper indirection (RWMutex-guarded, not atomic.Value, because the active tripper is
one of several concrete types) — and returns the `*Client` wrapper built over it:
`NewClient` swaps the declaration+resilience stack in (its executor comes from the
`cloud.ClientParams` the ctor passes down). Until the swap happens, requests pass
straight through to `http.DefaultTransport`.

### 2.3 One upload, layer by layer

`PutObject(ctx, bucket, key, ...)`:

1. `Client.PutObject` is the raw `*minio.Client`'s method (promoted), which signs the request (SigV4
   static credentials) and issues the HTTP request through the configured transport — the
   swapped-in declaration transport.
2. declareTransport (command.go), outermost: it declares the request's identity on the
   context — `db.system=s3`, `db.operation=PUT` (bounded, becomes a metric label), and
   `db.statement=/bucket/key` (per-call detail) with span name `"PUT /bucket/key"` — and
   delegates inward. It MUST sit outside the resilience round-tripper: the emitter reads the
   operation at `Execute` entry, so a declaration nested inside the executor would run per
   attempt and be read by nobody.
3. Resilience round-tripper, inside the declaration: the request enters the executor built at
   construction for service `s3:<endpoint>` from the `cloud.ClientParams` the ctor passed
   down (`*resilience.Manager` / `*fault.Injector` beans) — retry / rate-limit /
   circuit-breaker / bulkhead when a governance rules source applies them (hot-reloadable through
   the governance center), an observed-only unmanaged executor otherwise; the
   `*fault.Injector` (nil-safe) may inject failures for drills. This executor is the SINGLE
   emitter: it opens the call span, records the call-level `db.client.operation.duration`,
   the attempt-level `db.client.attempt.duration` per retry, and writes one access log —
   covering the whole call, retries included (minio-go ships no OTel hooks of its own).
4. The response unwinds: the emitter writes the span attributes, the metrics and the
   access-log line via the `_app_s3_access` tag at the log package's native levels — an error
   at Warn, a success carrying detail (the URL path) at Debug, a plain success at Info;
   minio-go returns the object info to the caller.

### 2.4 Health

`HealthCheck` (starter.go) probes with `ListBuckets` — it verifies both endpoint
reachability **and** that the credential pair is accepted. It is the single liveness
implementation: the startup probe in `newClient` and the `NewClientHealth` indicator
(health.go) both call it. `NewClientHealth` registers per instance under `s3:<name>` and
exports a `health.Indicator`, so an app importing starter-actuator gets S3 readiness folded
into `/readiness` with no extra wiring; `StarterS3.HealthCheck(ctx, *Client) error` is
exported for ad-hoc probing. The probe goes straight to the raw client.

---

## 3. Per-key behavior reference

Prefix `spring.s3.instances.<name>.*` for the ctor-bound `Config` keys (config.go).

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `endpoint` | string | — | **Required** (`expr:"$ != ''"`). `host:port` without scheme. | Missing/empty → bind-time validation error at startup. |
| `access-key-id` | string | — | **Required**; static SigV4 credential. | Missing → startup error; wrong → error on first use (or the ListBuckets probe rejects boot when `ping=true`). |
| `secret-access-key` | string | — | **Required**; static SigV4 credential. | Same as above. |
| `session-token` | string | — | Optional third element (temporary credentials). | Permanent creds + stale token → signature rejected on first use (or at the boot probe when `ping=true`). |
| `region` | string | us-east-1 | Bucket region passed to minio.Options. | Wrong region → signature/redirect errors on region-aware endpoints (may pass the probe against region-agnostic MinIO, then fail per-bucket). |
| `use-ssl` | bool | false | HTTPS towards the endpoint. | false against an TLS-only endpoint (or true against plaintext) → boot probe fails. |
| `bucket-lookup` | string | auto | `auto` \| `virtual-host`/`dns` (aliases, `BucketLookupDNS`) \| `path`. | Some S3-compatible clouds only serve path style → wrong style yields per-request addressing failures. Unknown value → startup error listing valid values. |
| `ping` | bool | `false` | Startup connectivity probe: when true the ctor runs `HealthCheck` (a `ListBuckets`) once and fails the boot if it errors, restoring fail-fast. Off by default so an endpoint that is not up yet does not block startup. | `ping=true` against a wrong endpoint / rejected credentials → boot error "failed to reach s3 endpoint …". |
| `health` | bool | `true` | Whether this instance contributes a `health.Indicator` (name `s3:<name>`) for the actuator's readiness/startup probes. Set false to keep the instance out of the aggregated health report. | `health=false` → no `s3:<name>` component in `/readiness`. |

---

## 4. Verification & fault drills

### 4.1 Object round-trip drill (same path as example/check.sh)

```bash
cd example && docker compose up -d && go run .   # asserts put→read→stat→remove, prints marker
curl -s :9370/readyz | grep -o '"s3:a[^"]*":[^,}]*'   # indicator UP (with actuator)
```

The example self-asserts `bytes.Equal(got, content)` after GetObject — any transport-level
corruption fails the run.

### 4.2 Fail-fast drill (`ping=true`)

Set `spring.s3.instances.a.secret-access-key=wrong` (and the instance's `ping=true`): boot aborts
with `failed to reach s3 endpoint ... (the request signature we calculated does not match ...)`.
The probe (ListBuckets) exists so credential/endpoint mistakes never reach first use; with the
default `ping=false` they surface on the first object operation instead.

### 4.3 Health drill

Stop MinIO (`docker compose stop minio`) while the app runs: `curl :9370/readyz` flips the
`s3:a` component to DOWN (probe = ListBuckets). Restart and it recovers — the indicator is
per-request, not latched.

### 4.4 Instrumentation drill

With starter-otel imported, generate one upload and read the signals: a client span
named `PUT /go-spring-example/hello.txt` (attributes `db.system=s3`, `db.operation=PUT`,
`db.statement` carrying the URL path, truncated at 512 bytes), the
`db.client.operation.duration` histogram plus the attempt-level
`db.client.attempt.duration` histogram and the `db.client.active_requests` gauge, and
the access-log line under the `_app_s3_access` tag — an errored operation logs at Warn, a
successful one carrying the URL-path argument logs at Debug, a plain success at Info.
`db.operation` is the HTTP method (bounded — a metric label); `db.statement` is the URL
path (per-call, span + log only).

### 4.5 Fault/resilience drill (needs governance: a configured rules source)

Applies per endpoint service label `s3:127.0.0.1:9000`: a governance rule with
`fault.rate` against that service makes a fraction of uploads fail through the executor —
observable as outcome-tagged spans/counters from the resilience layer. Flip the rule
file to withdraw (hot-reload through the governance source). ⚠ note the retry policy retries
per round-trip, not per stream: uploads with large bodies may re-send the body.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Startup aborts "failed to reach s3 endpoint" | endpoint down, wrong port, `use-ssl` mismatch, or bad credentials | The probe error carries the underlying cause (signature mismatch ⇒ creds; connection refused ⇒ endpoint/ssl). |
| Startup aborts "unknown bucket-lookup" | invalid style string | One of auto / virtual-host / dns / path. |
| Custom driver client has no resilience | dynamicTransport handshake only exists for DefaultDriver | Accept no declaration/resilience, or install your own indirection in the driver so `NewClient` can swap the stack in. |
| Works against MinIO, 404/redirect on cloud X | virtual-host addressing not supported there | `bucket-lookup=path`. |
| readyz DOWN though app works | credential rotation invalidated the pair after boot | The indicator probes live; refresh credentials / restart. |
| Two instances → container duplicate-bean error on health | (historical) indicator registered without `.Name` | Current code registers `s3:<name>` — keep that pattern when copying it for your own starters. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 10 |
| Required | 3 (endpoint, access-key-id, secret-access-key) |
| Quickstart external deps | 1 (MinIO / any S3 endpoint) |
| "Watch out" entries | 5 |

Design suspects (for the audit ledger; first two carried over from the previous edition):
- Client assembly is a `Driver` optional container bean (no per-config `driver` key); only the
  bundled DefaultDriver ships in-repo, and the dynamicTransport handshake between driver and
  wrapper is implicit (keyed off a sync.Map).
- `bucket-lookup` accepts both "virtual-host" and "dns" aliases for one mode — mild config
  surface redundancy.
- NEW: service label is `s3:<endpoint>` only — two instances on one endpoint (like the
  example's `a`/`b`) share one resilience scope; no per-instance disambiguation.
- The health probe and the fail-fast probe are the same ListBuckets call, now defined once in
  `HealthCheck`; `NewClientHealth` and the `newClient` startup probe both delegate to it.
