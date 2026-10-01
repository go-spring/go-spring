# starter-s3

[English](README.md) | [中文](README_CN.md)

`starter-s3` provides S3-protocol object storage support for Go-Spring:
multi-instance `*minio.Client` beans with fail-fast startup probes (bucket
list), per-request instrumentation (the starter declares each call's identity;
the resilience layer emits the span + metrics + access log), resilience
(rate limit / circuit breaking / fault injection on the HTTP transport), and
per-instance health indicators. It is built on
[minio-go](https://github.com/minio/minio-go), so one config surface covers
every S3-compatible endpoint.

## Compatibility

- **Native**: MinIO, AWS S3.
- **S3-compatible endpoints**: Aliyun OSS (`oss-<region>.aliyuncs.com`,
  `bucket-lookup=path`), Tencent COS (`cos.<region>.myqcloud.com`,
  `bucket-lookup=path`), and other clouds exposing an S3 facade. Set
  `bucket-lookup=path` where the endpoint does not support
  virtual-host-style addressing; vendor-specific features outside the S3
  facade (e.g. OSS image processing sub-resources) are out of scope — use a
  custom driver for those.

## Installation

```bash
go get go-spring.org/starter-s3
```

## Quick Start

### 1. Import

```go
import _ "go-spring.org/starter-s3"
```

### 2. Configure

```properties
spring.s3.instances.a.endpoint=127.0.0.1:9000
spring.s3.instances.a.access-key-id=minioadmin
spring.s3.instances.a.secret-access-key=minioadmin
spring.s3.instances.a.region=us-east-1
spring.s3.instances.a.use-ssl=false
spring.s3.instances.a.bucket-lookup=auto
```

### 3. Inject

```go
type Service struct {
    Client *StarterS3.Client `autowire:"a"`
}
```

### 4. Use

```go
_, err := s.Client.PutObject(ctx, "bucket", "key",
    bytes.NewReader(data), int64(len(data)),
    minio.PutObjectOptions{ContentType: "text/plain"})
```

The wrapper is `*Client`, not the raw `*minio.Client`: `NewClient` is the only
constructor and the wrapper embeds the raw client, so its full SDK surface is
promoted unchanged and every call still flows through the declaration +
governance transport.

## Core Features

- **Multi-instance clients** — every `spring.s3.instances.<name>` entry is its own bean
  with independent settings.
- **Fail-fast startup probe** — a `ListBuckets` round trip at boot catches
  wrong endpoints and rejected credentials before the first object
  operation.
- **Health indicator per instance** — the same probe is registered as
  `s3:<name>` and folded into `/readiness` by `starter-actuator` when
  imported.
- **Instrumentation** — the starter DECLARES each request's semantic identity
  (a `declareTransport` puts it on the context); the resilience layer on the
  same HTTP transport EMITS the signals from the one point that sees the whole
  call, retries included. One OTel client span (`db.system`/`db.operation`/
  `db.statement` attributes), the `db.client.operation.duration` histogram plus
  the `db.client.attempt.duration` histogram (attempt level — downstream latency
  per try, excluding backoff), and an access-log line (tag `_app_s3_access`).
  `db.system`/`db.operation` (the HTTP method) are bounded and become metric
  labels; `db.statement` (the URL path) is per-call detail and reaches the span
  and the log only, never a label. minio-go ships no OTel instrumentation of its
  own.
- **Resilience** — rate limiting, circuit breaking and fault injection are
  enforced on the client's HTTP transport through the executor built from the
  injected governance beans at construction; a hand-built client (no container
  governance) degrades to an observed-only, unmanaged executor that warns once.

## Advanced Features

**Multiple clients** — configure additional entries and inject by name:

```properties
spring.s3.instances.assets.endpoint=127.0.0.1:9000
spring.s3.instances.assets.access-key-id=...
spring.s3.instances.assets.secret-access-key=...
```

**Custom driver** — replace client assembly (e.g. to plug an IAM-role
credential provider or a custom `http.Transport`) by providing your own
`Driver` as an optional container bean. Every client under `spring.s3` is
built through it; when none is present the starter falls back to its bundled
`DefaultDriver`:

```go
func init() {
    gs.Provide(func() StarterS3.Driver { return iamDriver{} })
}
```
