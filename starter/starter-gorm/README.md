# starter-gorm

The gorm family core: the shared scaffolding behind every go-spring gorm dialect
starter. Its Go package is `gormcore`.

**Applications never import this module.** There is nothing here to configure
on its own — import a dialect starter (`starter-gorm-mysql`, `-postgres`,
`-sqlite`, `-sqlserver`, `-clickhouse`) and this module comes along
transitively.

## What it does

Five dialect starters exist, and each one owns only its dialect: the `Config`
block, the DSN, and TLS/discovery dialing. Everything dialect-agnostic lives
here, shared by all five instead of copy-pasted:

- the per-instance open/ping/customize sequence, connection-pool tuning, and the
  `*DB` wrapper bean (embeds `*gorm.DB`, so every gorm method promotes
  unchanged);
- multi-instance assembly via `Module`: one `*DB` bean plus a paired health
  indicator per configured `spring.gorm.<dialect>.instances.<name>` entry;
- the gorm observe plugin — one client span, one duration metric and one
  access-log line per Create/Query/Update/Delete, riding the OTel globals so it
  is near-zero overhead without `starter-otel`;
- the resilience callbacks — every operation runs under one backend-neutral
  `resilience.Executor`, with `gorm.ErrRecordNotFound` counted as success;
- the post-open `DBCustomizer` extension seam, and the dialect-qualified bean
  naming (`<dialect>.<name>`) that lets two dialects carry an instance of the
  same name without colliding.

## Full reference

[USAGE.md](USAGE.md) is the single shared reference for all five dialect
starters — assembly, config binding, observe, resilience, health, teardown, with
a complete worked project. [USAGE_CN.md](USAGE_CN.md) is the Chinese edition.
Each dialect's own README documents only the DSN/TLS/discovery keys it adds.
