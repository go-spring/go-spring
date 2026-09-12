# starter-s3 Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `config.go`, `driver.go`, `client.go`,
`command.go`, `health/health.go`, `s3_test.go`) and the runnable [example/](example/)
(smoke-verified against MinIO via docker-compose). **Object-storage operation semantics
(bucket/object APIs, retention, versioning) are [minio-go's documentation](https://github.com/minio/minio-go)**
— everything below is go-spring's increment: configuration, wiring, fail-fast startup,
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

// Put demonstrates one upload; the instrumentation/resilience transport sits
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
  ├─ conf.BindEach over spring.s3.instances.* → one Config per instance name
  ├─ per instance:
  │    ├─ r.Provide(newClient, IndexArg(1, ValueArg(c)),
  │    │            IndexArg(2, ?Driver)).Name(name)
  │    │      .Init((*Client).Init).Destroy((*Client).Destroy).Caller(1)
  │    ├─ r.Provide(health indicator).Name("s3:"+name).Export(health.Indicator)
  │    │      — .Name is what keeps multi-instance (Name,Type) keys unique
  │    ├─ newClient: optional Driver bean (none → bundled DefaultDriver;
  │    │      several coexist → the entry selects one by name:
  │    │      spring.s3.instances.<name>.driver = <bean-name>, empty = `spring.s3.default.driver`, then the single
  │    │      Driver bean by type, naming a missing bean fails startup)
  │    │      → d.CreateClient: static creds + region + bucket-lookup
  │    │        + a dynamicTransport placeholder inside minio.Options
  │    │      → dynamicTransports.LoadAndDelete hands the placeholder to the wrapper
  │    ├─ fail-fast probe: HealthCheck → ListBuckets — unreachable endpoint or rejected
  │    │      credentials abort startup (starter.go:76-78)
  │    └─ Init() [client.go]:
  │          obsTransport (span + db.client.* metrics + access log, observe.go)
  │          exec := fault.WrapExecutor(resilience.ExecutorFor("s3", "s3:<endpoint>"))  // outcome spans/counter
  │          dyn.Swap(resilience.NewRoundTripper(obsTransport, exec, → resource))
  ├─ Run / serve: readyz folds in every s3:<name> indicator (needs starter-actuator)
  └─ SIGTERM: Destroy() closes the resilience executor; minio holds no session to close
```

### 2.2 Why the dynamicTransport exists (rationale from source)

minio-go fixes the `http.Transport` inside `minio.Options` at construction and exposes no
setter, while the real transport (instrumentation + resilience) can only be swapped in
**after** the client exists.
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
   failures for drills. The observe layer resolved inside the executor emits an outcome
   span + call counter + duration histogram + access log for breaker trips, limit rejects,
   bulkhead rejections.
3. obsTransport (command.go:38): starts the per-request observer span with operation
   `"PUT /bucket/key"` (method + URL path), runs the base `http.DefaultTransport`, ends the
   span with the error — the span + duration metric + access log all carry that operation
   name (minio-go ships no OTel hooks of its own, so the starter's transport carries all
   three signals).
4. The response unwinds: span attributes/metrics recorded, access-log line emitted via the
   `_app_s3_access` tag at the log package's native levels — an error at Warn, a success
   carrying the URL-path argument at Debug, a plain success at Info; minio-go returns the
   object info to the caller.

### 2.4 Health

`NewClientHealth` (health/health.go:33) probes with `ListBuckets` — it verifies both endpoint
reachability **and** that the credential pair is accepted. Registered per instance under
`s3:<name>` and exported as `health.Indicator`, so an app importing starter-actuator gets S3
readiness folded into `/readiness` with no extra wiring. The same function is exported as
`StarterS3.HealthCheck(ctx, *Client) error` for ad-hoc probing.

---

## 3. Per-key behavior reference

Prefix `spring.s3.instances.<name>.*` for the ctor-bound `Config` keys (config.go).

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `endpoint` | string | — | **Required** (`expr:"$ != ''"`). `host:port` without scheme. | Missing/empty → bind-time validation error at startup. |
| `access-key-id` | string | — | **Required**; static SigV4 credential. | Missing → startup error; wrong → the ListBuckets fail-fast probe rejects boot. |
| `secret-access-key` | string | — | **Required**; static SigV4 credential. | Same as above. |
| `session-token` | string | — | Optional third element (temporary credentials). | Permanent creds + stale token → signature rejected at the boot probe. |
| `region` | string | us-east-1 | Bucket region passed to minio.Options. | Wrong region → signature/redirect errors on region-aware endpoints (may pass the probe against region-agnostic MinIO, then fail per-bucket). |
| `use-ssl` | bool | false | HTTPS towards the endpoint. | false against an TLS-only endpoint (or true against plaintext) → boot probe fails. |
| `bucket-lookup` | string | auto | `auto` \| `virtual-host`/`dns` (aliases, `BucketLookupDNS`) \| `path`. | Some S3-compatible clouds only serve path style → wrong style yields per-request addressing failures. Unknown value → startup error listing valid values. |

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

Set `spring.s3.instances.a.secret-access-key=wrong`: boot aborts with `failed to reach s3 endpoint ...
(the request signature we calculated does not match ...)`. The probe (ListBuckets) exists so
credential/endpoint mistakes never reach first use.

### 4.3 Health drill

Stop MinIO (`docker compose stop minio`) while the app runs: `curl :9370/readyz` flips the
`s3:a` component to DOWN (probe = ListBuckets). Restart and it recovers — the indicator is
per-request, not latched.

### 4.4 Instrumentation drill

With starter-otel imported, generate one upload and read the three signals: a client span
named `PUT /go-spring-example/hello.txt` (attributes `db.system=s3`, `db.operation`,
`db.statement` carrying the URL path, truncated at 512 bytes), the
`db.client.operation.duration` histogram (plus the `db.client.active_requests` gauge), and
the access-log line under the `_app_s3_access` tag — an errored operation logs at Warn, a
successful one carrying the URL-path argument logs at Debug, a plain success at Info.

### 4.5 Fault/resilience drill (needs starter-governance)

Arms per endpoint resource label `s3:127.0.0.1:9000`: a governance rule with
`fault.rate` against that resource makes a fraction of uploads fail through the executor —
observable as outcome-tagged spans/counters from the executor's observe layer. Flip the rule
file to withdraw (hot-reload through the governance source). ⚠ note the retry policy retries
per round-trip, not per stream: uploads with large bodies may re-send the body.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Startup aborts "failed to reach s3 endpoint" | endpoint down, wrong port, `use-ssl` mismatch, or bad credentials | The probe error carries the underlying cause (signature mismatch ⇒ creds; connection refused ⇒ endpoint/ssl). |
| Startup aborts "unknown bucket-lookup" | invalid style string | One of auto / virtual-host / dns / path. |
| Custom driver client has no resilience | dynamicTransport handshake only exists for DefaultDriver | Accept observe-only, or install your own indirection in the driver. |
| Works against MinIO, 404/redirect on cloud X | virtual-host addressing not supported there | `bucket-lookup=path`. |
| readyz DOWN though app works | credential rotation invalidated the pair after boot | The indicator probes live; refresh credentials / restart. |
| Two instances → container duplicate-bean error on health | (historical) indicator registered without `.Name` | Current code registers `s3:<name>` — keep that pattern when copying it for your own starters. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 8 |
| Required | 3 (endpoint, access-key-id, secret-access-key) |
| Quickstart external deps | 1 (MinIO / any S3 endpoint) |
| "Watch out" entries | 5 |

Design suspects (for the audit ledger; first two carried over from the previous edition):
- Client assembly is a `Driver` optional container bean (no per-config `driver` key); only the
  bundled DefaultDriver ships in-repo, and the dynamicTransport handshake between driver and
  wrapper is implicit (keyed off a sync.Map).
- `bucket-lookup` accepts both "virtual-host" and "dns" aliases for one mode — mild config
  surface redundancy.
- NEW: resource label is `s3:<endpoint>` only — two instances on one endpoint (like the
  example's `a`/`b`) share one resilience scope; no per-instance disambiguation.
- NEW: health probe and fail-fast probe are the same ListBuckets call but duplicated in code
  (starter.go HealthCheck vs health/health.go) — harmless but a small consolidation target.
