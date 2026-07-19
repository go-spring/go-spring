# stdlib/starter — Migration Guide

`stdlib/starter` is the shared, zero-dependency home for the three cross-cutting
concerns every starter used to hand-roll: TLS config, health indicators, and
startup fail-fast validation. This guide shows how each starter archetype
replaces its private copy with the shared helper.

Import path: `go-spring.org/stdlib/starter`. Because `stdlib` is already a
dependency of every starter (they import `stdlib/health`, `stdlib/errutil`),
adopting this package needs **no `go.mod` change** — the workspace resolves it.

The API surface:

| Concern    | Old (per-starter)                                   | New (shared)                                             |
|------------|-----------------------------------------------------|----------------------------------------------------------|
| TLS        | local `TLSConfig` struct + `buildTLSConfig(c)`      | `starter.TLSConfig` field + `cfg.TLS.Build()`            |
| Health     | local `xHealth{ name; client }` + 3 methods         | `starter.NewIndicator(name, probe, opts...)`             |
| Fail-fast  | inline `errutil.Explain(nil, "x: y is required")`   | `starter.RequireField` / `starter.RequireAny`            |

---

## 1. TLS (all archetypes that terminate/dial TLS)

**Before** — a private struct + builder (e.g. `starter-go-redis/config.go`):

```go
type TLSConfig struct {
    Enabled            bool   `value:"${enabled:=false}"`
    CertFile           string `value:"${cert-file:=}"`
    KeyFile            string `value:"${key-file:=}"`
    CAFile             string `value:"${ca-file:=}"`
    ServerName         string `value:"${server-name:=}"`
    InsecureSkipVerify bool   `value:"${insecure-skip-verify:=false}"`
}

func buildTLSConfig(c TLSConfig) (*tls.Config, error) { /* ~28 lines */ }
```

**After** — delete both; embed the shared type in your `Config`:

```go
import "go-spring.org/stdlib/starter"

type Config struct {
    // ...
    TLS starter.TLSConfig `value:"${tls}"`
}
```

Replace every `buildTLSConfig(c.TLS)` call site with `c.TLS.Build()`:

```go
tlsCfg, err := c.TLS.Build()   // (nil, nil) when disabled
if err != nil {
    return nil, errutil.Explain(err, "redis: build TLS") // add your prefix if wanted
}
```

Notes per archetype:

- **Client starters** (`go-redis`, `redigo`, `mongodb`, `neo4j`, `nats`,
  `kafka`, `kafka-sarama`, `rabbitmq`, `mqtt`, `mail`, `lock-consul`,
  `lock-etcd`, `registry-etcd`, ...): pass the `*tls.Config` into the client's
  dial options. `Build()` returning `nil` means "no TLS", which every client
  library accepts.
- **Server starters** using `*tls.Config` directly (gin/echo/hertz via
  `http.Server.TLSConfig`, gateway): assign `srv.TLSConfig = tlsCfg`.
- **gRPC / thrift** that load a file pair through their own credentials API
  (`credentials.NewServerTLSFromFile`): you can keep that call, OR switch to
  `credentials.NewTLS(tlsCfg)` after `cfg.TLS.Build()` to gain CA/ServerName
  support the file-pair API lacks. Field names are unchanged, so properties
  stay backward compatible.

Error-wording change: the shared builder emits generic `tls: ...` prefixes
instead of `redis: ...`. Wrap with `errutil.Explain(err, "redis: ...")` at the
call site if you want the component prefix back.

---

## 2. Health indicators (client / server starters exporting `health.Indicator`)

**Before** — a bespoke struct just to satisfy the interface
(`starter-go-redis/health.go`):

```go
type redisHealth struct {
    name   string
    client redis.UniversalClient
}
func (h *redisHealth) HealthName() string { return "redis:" + h.name }
func (h *redisHealth) CheckHealth(ctx context.Context) error { return h.client.Ping(ctx).Err() }
func newClientHealth(name string, c *redis.Client) *redisHealth { return &redisHealth{name, c} }
```

**After** — delete the struct and constructors; build inline where you register
the bean:

```go
import "go-spring.org/stdlib/starter"

ind := starter.NewIndicator("redis:"+name, func(ctx context.Context) error {
    return client.Ping(ctx).Err()
})
// gs.Provide(...).Export(gs.As[health.Indicator]())  — registration unchanged
```

Options cover the variations found in the tree:

- `starter.WithGroups(health.GroupReadiness, ...)` — implement `health.Grouped`.
  Omit it to keep the default (readiness + startup).
- `starter.NonCritical()` — implement `health.Critical` returning false for a
  degraded-but-tolerable dependency.

The returned value is a `health.Indicator`; export it exactly as before.

---

## 3. Fail-fast validation (client / config-provider constructors)

**Before** — inline required-field checks scattered across constructors:

```go
if cfg.Host == "" {
    return nil, errutil.Explain(nil, "mail: host is required")
}
if cfg.Addr == "" && cfg.ServiceName == "" {
    return nil, errutil.Explain(nil, "http-client: one of addr or service-name is required")
}
```

**After**:

```go
import "go-spring.org/stdlib/starter"

if err := starter.RequireField("mail", "host", cfg.Host); err != nil {
    return nil, err
}
if err := starter.RequireAny("http-client",
    starter.Field{Name: "addr", Value: cfg.Addr},
    starter.Field{Name: "service-name", Value: cfg.ServiceName},
); err != nil {
    return nil, err
}
```

`RequireField` produces `"<component>: <field> is required"`; `RequireAny`
produces `"<component>: one of <a> or <b> is required"`. A few starters used
slightly different wording (e.g. redis "... must be set"); those messages become
uniform after migration — a deliberate, acceptable change. Keep bespoke
`errutil.Explain` for mode-specific, multi-condition messages that these two
helpers don't express (e.g. redis's "master-name and sentinel-addrs are required
in sentinel mode").

---

## Verification

Each migrated starter must still build and pass its own tests. Because starters
are separate modules resolved through `go.work`, run from the starter directory:

```sh
GOWORK=/path/to/go.work go build ./... && go test ./...
```

Do not add a `require` on `stdlib` for this — the workspace already provides it.
