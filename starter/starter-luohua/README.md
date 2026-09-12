# Starter Luohua

[English](README.md) | [中文](README_CN.md)

A **company umbrella starter** (the *aggregator / profile* archetype): it
re-bases go-spring onto a hypothetical company's conventions by wiring existing
go-spring standard components through their public extension seams — it never
re-implements them.

Blank-import it and set `spring.luohua.*`:

```go
import _ "go-spring.org/starter-luohua"
```

Governing rule — *standard component + company customization point*: for every
default luohua provides there is an explicit seam (a config key, a
`Register*` driver registry, or an `OnMissingBean` default) through which a team
that uses the standard component but wants to customise here can do so. If a
luohua default ever needs a private path to work, the seam — not the default —
is wrong.

## What it re-bases today

### Wire vocabulary — `spring.luohua.propagate.*`

| Key | Default | Meaning |
| --- | --- | --- |
| `enabled` | `true` | Master switch for the whole baseline. |
| `propagate.load-test-header` | (empty) | Override `traffic.HeaderLoadTest` so load-test detection/injection uses luohua's own header (the G1 seam). The gRPC metadata key is derived by lower-casing. |
| `propagate.headers` | (empty) | Business headers a luohua named-header propagator carries across every hop (`X-Tenant`, `X-User`, …). They ride the OTel global propagator, so httpx / gin / echo / grpc all honour them. |
| `observability.fields` | (empty) | Context fields luohua prints on every log line (same names as `propagate.headers`, e.g. `X-Tenant`). |

The named-header propagator is registered into `starter-otel/trace` under the
name `luohua` (the G2 seam). To run it, compose it into the fleet propagator:

```properties
spring.observability.trace.propagator=w3c,luohua
spring.luohua.propagate.headers=X-Tenant
```

Registration alone is inert — a name nobody lists never runs.

### Identity — `spring.luohua.identity.*`

| Key | Default | Meaning |
| --- | --- | --- |
| `secret` | (required) | Shared HMAC secret. Present arms a `LuohuaSSO` `security.TokenValidator` bean. |
| `issuer` | `luohua` | Expected token issuer. |

`LuohuaSSO.Issue(subject, tenant, authorities, ttl)` mints a token for tests /
the example; `Validate` plugs into the per-family `Authenticate` / `Authorize`
middleware shells unchanged. Bring your own OIDC/JWT verifier by providing your
own `security.TokenValidator` — luohua never special-cases its own.

### Error catalog — `spring.luohua.i18n.*`

| Key | Default | Meaning |
| --- | --- | --- |
| `default-locale` | `zh` | Catalog fallback locale. |

Luohua's one catalog is provided as an `i18n.MessageSource` default and steps
aside (`OnMissingBean`) if the application supplies its own.

### Standard cache — `luohua`

Luohua provides an in-process, TTL-aware cache as a `cache.Cache` bean under the
uniform company name `luohua`:

```go
Cache *cache.Cache `autowire:"luohua"`
```

The bean carries no config gate: if nothing injects it, it never instantiates.

## Governance

Luohua deliberately brings **no governance engine of its own** — outbound calls
already funnel through go-spring's neutral `resilience.ExecutorFor` /
`fault.InjectorFor` seams under the single governance authority, and a company
that has no bespoke backend should ride the official one and pin its policy per
fleet. Do that in a governance rules document (see starter-governance):

```properties
govern.driver=default
govern.rules[0].resources=orders
govern.rules[0].timeout=500ms
govern.rules[0].max-retries=2
```

Registering a company resilience backend (instead of `default`) is a plain
`resilience.RegisterDriver(name, …)` in a fork of this starter — the seam exists,
luohua just chooses not to ship a pretend engine.

## Arming

The module arms on any `spring.luohua.*` key; `spring.luohua.enabled=false`
silences the whole baseline. Capabilities compose existing starters and step
aside (`OnMissingBean`) when an application provides its own — luohua never
fights a company bean.

## Customization points you keep

Everything luohua touches stays overridable: the traffic header is a package
variable you may re-bind; the propagator list is config; the log hook composes
(not replaces) the previous one. If you need a stricter ordering, install your
own `log.FieldsFromContext` after luohua runs.

## Full baseline (`app.properties`)

A service blank-imports `starter-luohua` **and** `starter-otel` (the otel starter
owns the Tracer/Meter providers and the propagator the example composed into),
then arms the whole baseline with one config block:

```properties
# otel: no exporter needed offline, but propagate trace + luohua's headers.
spring.observability.trace.exporter=none
spring.observability.metrics.enable=false
spring.observability.trace.propagator=w3c,luohua

# luohua baseline
spring.luohua.enabled=true
spring.luohua.identity.secret=replace-me
spring.luohua.identity.issuer=luohua
spring.luohua.i18n.default-locale=zh
spring.luohua.propagate.load-test-header=X-Luohua-LoadTest
spring.luohua.propagate.headers=X-Tenant,X-User
spring.luohua.observability.fields=X-Tenant
```

`spring.luohua.propagate.headers` names the business headers the `luohua`
propagator carries (it is inert unless `luohua` appears in
`spring.observability.trace.propagator`); `spring.luohua.observability.fields`
names which of them also print on every log line.
