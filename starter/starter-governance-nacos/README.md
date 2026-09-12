# starter-governance-nacos

[English](README.md) | [中文](README_CN.md)

`starter-governance-nacos` adapts [Nacos](https://nacos.io/) as a governance
rule source for Go-Spring. It listens on one Nacos dataId holding the
governance rule document and pushes every published version into the
[`go-spring.org/cloud/governance`](../../../cloud/governance) center through
the `governance.Source` contract, so a rule change re-resolves governance live
without a restart and without an application-wide property re-bind.

It is the Nacos member of the governance-source adapter family (the
Sentinel-datasource shape); the etcd member is
[starter-governance-etcd](../../starter-governance-etcd) and the file/http
members live in [starter-governance](../../starter-governance). All of them
parse the same rule document through `rules.Parse`, so a document is
byte-portable between backends.

## Relation to starter-config-nacos

This module is deliberately separate from
[starter-config-nacos](../starter-config-nacos), even though both speak to
Nacos:

* `starter-config-nacos` owns the `spring.config.import` **remote-configuration**
  role — application properties pulled into the merged property storage and
  hot-reloaded into `gs.Dync[T]` fields.
* `starter-governance-nacos` owns the **governance rule document** role. The
  document is governance's own; only its bootstrap key lives in
  `app.properties`, and its changes refresh governance only.

The two never turn each other on: enabling governance does not enable app
property refresh, and importing a remote config does not arm governance. This
source also builds and owns its Nacos client exclusively, so it carries its own
instrumentation independently of the config-import bootstrap client.

## Installation

```bash
go get go-spring.org/starter-governance-nacos
```

## Quick Start

### 1. Import the package

```go
import _ "go-spring.org/starter-governance-nacos"
```

The bean is registered only when a `govern.source.nacos.*` key is present, so
the blank import is inert otherwise.

### 2. Configure the source

Add the bootstrap keys to your [configuration file](example/conf/app.properties):

```properties
govern.source.nacos.server=127.0.0.1:8848
govern.source.nacos.data-id=app-govern.yaml
govern.source.nacos.group=DEFAULT_GROUP
```

`server` and `data-id` are required; the other keys default (see
[Configuration Keys](#configuration-keys)). The dataId must already exist: the
source's construction performs an initial `GetConfig`, so a missing or
unparseable document fails startup instead of arming a disabled center.

### 3. Publish the rule document

Put the rules in their OWN dataId, with the same `govern.*` keys an
`app.properties` entry would use:

```yaml
govern:
  enabled: true
  default:
    enabled: true
    attempt-timeout: 100ms
  rules:
    - resources: demo:resource
      attempt-timeout: 50ms
```

Every published version is re-parsed and pushed into the center. The rule
vocabulary (`govern.enabled`, `govern.default.*`, `govern.rules[n].*`,
`govern.fault.*`) belongs to the governance domain; see
[starter-governance's USAGE](../../starter-governance/USAGE.md) for the full
reference. Introducing the governance center itself is still
[starter-governance](../../starter-governance)'s job — this module only supplies
the source.

## Configuration Keys

All keys live under `govern.source.nacos`:

| Key         | Default                | Description                                   |
|-------------|------------------------|-----------------------------------------------|
| `server`    | (required)             | Nacos server address, `host:port`, one server |
| `data-id`   | (required)             | dataId holding the rules document             |
| `group`     | `DEFAULT_GROUP`        | Nacos group                                   |
| `namespace` | `""` (public)          | Namespace id, not name                        |
| `username`  | `""`                   | Auth username; empty means no auth            |
| `password`  | `""`                   | Auth password                                 |
| `format`    | dataId extension, else `properties` | `properties` / `yaml` / `toml` / `json` |

## Core Behavior

* **Fail-fast seed.** Construction calls `GetConfig` once and parses the result.
  A missing dataId, a client error, or an unparseable document fails
  construction (and startup), so a misconfigured source never silently disables
  governance.
* **Push on publish.** `ListenConfig` delivers each published version; the
  document is re-parsed and pushed only when the rules actually changed.
* **Keep-last-good.** A bad publish keeps the last good snapshot and logs; the
  center is never handed an unvouched config.
* **Dedupe.** A byte-equal re-delivery (Nacos may re-push on reconnect) is a
  no-op, and a semantically unchanged document does not churn executors.
* **Owns its client.** `Close` removes the listener and closes the client the
  bean built; the source implements the optional-close contract the center
  probes for on Destroy.

### Log tag

Runtime logs from this module carry the tag `_app_governance_nacos` (nacos
governance source). Tune them independently of the main log by binding a logger
to the tag:

```properties
logger.governance_nacos.type=Logger
logger.governance_nacos.level=WARN
logger.governance_nacos.tag=_app_governance_nacos
```

## Example

The runnable [example/](example) seeds the watched dataId before startup, then
publishes an updated document and self-asserts that the resolved policy moved —
and exits 0. Run it with a standalone Nacos:

```bash
cd example && ./check.sh
```

`check.sh` is docker-gated (it is skipped when docker or a compose command is
absent), brings Nacos up with `docker-compose.yml`, waits on the Nacos readiness
endpoint because Nacos boot is slow, runs the example under a watchdog, and
tears the container down.

See [USAGE.md](USAGE.md) for the full reference.
