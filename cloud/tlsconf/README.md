# tlsconf

[English](README.md) | [中文](README_CN.md)

`tlsconf` is the shared TLS configuration helper used by every Go-Spring
starter that terminates or dials TLS (redis, gorm dialects, kafka, nats,
mqtt, grpc, gin, gateway, neo4j, cassandra, registry/lock backends, ...).
It binds the off-by-default `tls.*` property block to a `*tls.Config`,
loading the key pair and CA bundle from disk when provided. It depends
only on the Go standard library and `go-spring.org/stdlib/errutil`, so
any module in the repo can adopt it without inheriting a dependency
graph.

## Installation

```
go get go-spring.org/cloud
```

## Embedding

Embed `TLSConfig` under a `tls` key in the starter's per-instance Config;
the bound properties then read as `spring.<client>.<name>.tls.*`:

```go
type Config struct {
    ...
    TLS tlsconf.TLSConfig `value:"${tls:=}"`
}
```

| Property | Default | Meaning |
|---|---|---|
| `tls.enabled` | `false` | Turn TLS on; never negotiated unless asked. |
| `tls.cert-file` / `tls.key-file` | empty | PEM key pair this side presents. |
| `tls.ca-file` | empty | CA bundle (PEM) to verify the peer. Empty = host root set. |
| `tls.server-name` | empty | Override the name checked against the peer cert (dial-by-IP, discovery labels). |
| `tls.insecure-skip-verify` | `false` | Disable verification. Local testing only. |

The config surface is shared across 20+ starters, so the semantics are
too: an operator moving between redis, kafka, and grpc finds the same
`tls.enabled` / `cert-file` / `ca-file` knobs behaving the same way.

## BuildClient (client side)

```go
tlsCfg, err := c.TLS.BuildClient()
if err != nil { return err }
client := somelib.NewClient(somelib.WithTLS(tlsCfg))
```

When `enabled=false`, `BuildClient` returns `(nil, nil)` — "no TLS" — which
every client library accepts as a nil `*tls.Config`, so starters pass the
result straight through with no branching. `CAFile` sets `RootCAs`; on
the client it is always just the root set for verification, never an
mTLS trigger. `MinVersion` follows the `crypto/tls` default; starters
needing a stricter floor can set `MinVersion` on the returned config.

Errors carry a generic `tls:` prefix (`BuildClient` does not know which
component it serves); wrap with `errutil.Explain(err, "redis: ...")` for
a component-specific one.

## BuildServer (server side)

```go
tlsCfg, err := c.TLS.BuildServer()
```

Server semantics differ in two ways:

- `CAFile` is the bundle of CAs trusted to sign **client** certificates:
  it sets `ClientCAs` and turns on `RequireAndVerifyClientCert` —
  requesting a CA file enables mutual TLS. Leave it empty for one-way
  TLS.
- `ServerName` and `InsecureSkipVerify` are client-side knobs; on the
  server they would describe verifying the peer *we* dial, so both are
  ignored.

As with `BuildClient`, `enabled=false` yields `(nil, nil)`.

## Boundaries

- The package stops at producing the `*tls.Config`: no provider setup,
  no listener wrapping. The starter hands the result to its library or
  `tls.NewListener`.
- No certificate reloading / rotation. `BuildClient` is called once at
  construction; rotation is a lifecycle concern of the starter.
