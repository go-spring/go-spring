# starter-governance-etcd

[English](README.md) | [中文](README_CN.md)

`starter-governance-etcd` is the etcd member of the governance rule-source family
— the same datasource shape as the file and http sources in
[`starter-governance`](../starter-governance/README.md). It watches ONE etcd key
holding a governance rule document and pushes each changed version into the
governance center through the `governance.Source` contract of
[`go-spring.org/cloud/governance`](../../cloud/governance).

Blank-importing this module is inert until `govern.source.etcd.*` is configured;
then it registers one `governance.Source` bean. The document in the key uses the
same `govern.*` keys any other source accepts, so a rules file is byte-portable
between the file, http and etcd backends.

## Why a separate module from `starter-config-etcd`

Both starters speak etcd, but they play different roles and must not be merged:

* The governance rule document is **governance's own**, not an application-config
  import. Enabling governance must never turn on app-wide property refresh, and
  enabling remote configuration must never arm governance.
* [`starter-config-etcd`](../starter-config-etcd/README.md) owns the
  `spring.config.import` remote-configuration role: a change there re-reads and
  re-binds the whole application. This module owns the governance rules role: a
  change on its key refreshes governance **only**.
* Splitting them also lets the source own its client exclusively, free to carry
  its own instrumentation independently of the config-import bootstrap client.

## Installation

```bash
go get go-spring.org/starter-governance-etcd
```

## Quick Start

### 1. Import the starter (and the wiring)

```go
import (
    _ "go-spring.org/starter-governance"
    _ "go-spring.org/starter-governance-etcd"
)
```

`starter-governance` is the module that hands the injected `governance.Source`
bean to the center; importing only the etcd adapter registers the bean but arms
nothing.

### 2. Configure the source

Add the two bootstrap keys to your
[configuration file](example/conf/app.properties):

```properties
govern.source.etcd.endpoint=127.0.0.1:2379
govern.source.etcd.key=/app/govern.yaml
```

Only `endpoint` and `key` are required; auth and format fall back to their
defaults.

### 3. Publish the rules document to the key

The key holds governance's rules, using the `govern.*` namespace:

```yaml
govern:
  enabled: true
  default:
    enabled: true
    attempt-timeout: 100ms
```

### 4. Run

The initial `Get` seeds the snapshot at startup, so the key must exist before
the application boots; a missing or unparseable document fails startup. After
that, every PUT on the key is pushed into the center without a restart.

## Configuration Keys

All keys live under `govern.source.etcd` (exact match):

| Key | Default | Description |
|-----|---------|-------------|
| `endpoint` | (required) | etcd endpoint to watch |
| `key` | (required) | KV key holding the rules document |
| `username` | `""` | etcd auth username; empty means no auth |
| `password` | `""` | etcd auth password |
| `format` | key extension, else `properties` | document format override |

## Core Behavior

* **Conditional registration.** The bean is registered only when a
  `govern.source.etcd.*` key is present (`gs.OnProperty` is a prefix check), so a
  blank import without configuration is inert.
* **Fail-fast seeding.** Construction does an initial `Get`; a missing key or a
  document that does not parse fails startup instead of arming a silently
  disabled center.
* **Watch push.** After startup the source watches the one key and applies every
  PUT: the value is parsed with the shared rule parser and, when the rules
  actually changed, pushed to the center.
* **Last-good on bad edits.** A value that fails to parse keeps the previous
  snapshot and is logged, never pushed; governance is turned off with
  `govern.enabled=false`, not with a broken or emptied document.
* **No-change dedupe.** A byte-identical re-delivery, or a re-parse that yields
  an equal config, pushes nothing — a touch does not churn executors.
* **Owned client.** The source builds and owns its etcd client, and closes it
  when the bean is destroyed.

See [USAGE.md](USAGE.md) for the full behavior reference and the runnable
[example](example) (`example/check.sh`, docker-gated and self-asserting).
