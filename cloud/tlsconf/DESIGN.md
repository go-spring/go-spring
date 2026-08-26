# tlsconf Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`tlsconf` is the one TLS configuration face shared by 20+ starters (redis,
gorm dialects, kafka, nats, mqtt, grpc, gin, gateway, neo4j, cassandra,
registry/lock backends, ...). Before it existed, each starter declared its
own subset of these knobs with its own defaults; now there is one nested
block, one set of defaults, and one place the loading logic lives.

## 1. Responsibilities & Boundaries

- **Does:** define `TLSConfig` (the union of the fields starters were each
  declaring on their own), bind it under a `tls` key, and turn it into a
  `*tls.Config` via `Build` (client) or `BuildServer` (server), loading
  the key pair and CA bundle from disk.
- **Refuses:**
  - No provider setup, no listener wrapping. The starter hands the
    returned `*tls.Config` to its library / `tls.NewListener`; this
    package stops at producing the config.
  - No dependency beyond stdlib + `stdlib/errutil`. Being importable from
    every module in the repo without dragging a dependency graph is the
    whole reason the helper exists.
  - No certificate reloading / rotation. `Build` is called once at
    construction; rotation is a lifecycle concern of the starter.

## 2. Key Decisions

- **The config surface is shared, so the semantics must be.** With 20+
  starters embedding the same `TLSConfig`, an operator can move between
  redis, kafka, and grpc and find the same `tls.enabled` / `cert-file` /
  `ca-file` knobs behaving the same way. That uniformity is worth more
  than per-starter flexibility, which is why the struct is the union of
  previously divergent fields rather than an interface.
- **`MinVersion` follows the `crypto/tls` default — deliberately.** The
  package does not pin a minimum TLS version. Go's stdlib raises its
  default as old protocol versions are deprecated (and go-spring tracks a
  recent Go), so following stdlib keeps every starter's floor moving with
  platform guidance without a coordinated config change across 20
  starters. Starters that need a stricter floor can still set
  `MinVersion` on the returned config. Pinning a version *here* would
  freeze the floor at whatever was current when the helper was written.
- **`(nil, nil)` when disabled.** A nil `*tls.Config` means "no TLS" to
  every client library and to `tls.NewListener`-style server paths, so
  starters pass the result straight through with no branching. The
  alternative — returning an empty config — would enable TLS with default
  settings, the opposite of the off-by-default contract.
- **`BuildServer` interprets `CAFile` as the client CA bundle.** On the
  server, a CA file can only mean "certificates clients present must be
  signed by this", so setting `ClientCAs` + `RequireAndVerifyClientCert`
  (i.e. requesting a CA file turns on mTLS) is the least surprising
  reading; empty stays one-way TLS. `ServerName` and
  `InsecureSkipVerify` describe verifying a peer *we* dial, which a
  server does not, so they are ignored rather than misapplied.
- **Off by default.** `enabled` defaults to false: a starter never
  negotiates TLS unless the operator asks, matching the repo-wide
  "unconfigured = not wired" stance.

## 3. Trade-offs / Alternatives Rejected

- **One struct, not per-role structs.** Client-only and server-only field
  sets (two types) would be more precise but would split the shared
  configuration surface back in two; one struct with role-specific
  interpretation in `BuildServer` keeps the operator vocabulary single.
- **Generic `tls:` error prefix.** `Build` does not know which component
  it serves; it prefixes with `tls:` and lets the starter wrap
  (`errutil.Explain(err, "redis: ...")`). Baking component names in would
  require passing them in for a cosmetic gain.
- **No auto-detection of mTLS on the client.** Some libraries infer mutual
  TLS from the presence of a client cert; here `CAFile` on the client is
  always just the root set for verification. Explicit `BuildServer`
  semantics on the server side carry the mTLS decision instead.
