# starter-gorm

The gorm family core: the shared scaffolding behind every go-spring gorm dialect
starter. Its Go package is `gormcore`.

**Applications never import this module.** There is nothing here to configure
on its own — import a dialect starter (`starter-gorm-mysql`, `-postgres`,
`-sqlite`, `-sqlserver`, `-clickhouse`) and this module comes along
transitively.

## What it does

Five dialect starters exist, and each one owns its dialect — the `Config` block,
the DSN, TLS/discovery dialing — and the `gs.Module` block that registers its
beans, written out in the starter so a reader sees exactly what it contributes:
one `*DB` plus a paired health indicator per configured
`spring.gorm.<dialect>.instances.<name>` entry. Everything dialect-agnostic lives
here, shared by all five instead of copy-pasted:

- the per-entry construction the starters' beans run, `NewDB`: the
  open/ping/customize sequence, connection-pool tuning, and the `*DB` wrapper
  bean (embeds `*gorm.DB`, so every gorm method promotes unchanged);
- the gorm observe plugin — it DECLARES each Create/Query/Update/Delete as a
  client operation (its name, `db.system`/`db.operation` labels, SQL statement
  and access tag) on the call's context. The signals themselves — the call
  span, the call- and attempt-level duration histograms, the in-flight gauge and
  the one access log — are emitted by the resilience layer, the single emitter
  on the executor chain. The plugin emits nothing, so it rides the OTel globals
  and is near-zero overhead without `starter-otel`;
- the resilience callbacks — every operation runs under one backend-neutral
  `resilience.ClientExecutor`, with `gorm.ErrRecordNotFound` counted as success;
- the post-open `DBCustomizer` extension seam, and the dialect-qualified bean
  name (`<dialect>.<name>`) each starter applies, so two dialects can carry an
  instance of the same name without colliding.

## Full reference

[USAGE.md](USAGE.md) is the single shared reference for all five dialect
starters — assembly, config binding, observe, resilience, health, teardown, with
a complete worked project. [USAGE_CN.md](USAGE_CN.md) is the Chinese edition.
Each dialect's own README documents only the DSN/TLS/discovery keys it adds.
