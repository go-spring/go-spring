# starter-governance-file — Rule Source Adapters (file, plus a bundled HTTP one)

[English](README.md) | [中文](README_CN.md)

This starter is now **only the source adapters**.

- **The wiring moved to `cloud/governance`.** [starter.go](../../cloud/governance/starter.go) registers the center over the authorities and the always-on wiring bean ([wiring.go](../../cloud/governance/wiring.go)); the three authorities themselves are registered by the packages that own them (`cloud/resilience`, `cloud/loadbalance`, `cloud/fault`), which every client starter already imports. So this starter is not what makes governance — or even the authority beans — take effect.
- **What is left here:** the rule-source adapters, through which governance rules flow in via the `governance.Source` contract. The **file** source is the module's reason to exist; the **http** source rides along in the same module as a bonus (no third-party SDK, same shape). etcd and nacos each live in their own module (`starter-governance-etcd`, `starter-governance-nacos`), peers of this one, blank-imported on demand.

The family's third kind is the **driver backend**: `starter-governance-sentinel` contributes sentinel-golang as a `resilience.Driver` bean named `sentinel`, which the wiring bean above collects into the driver directory and the governance document's `spring.governance.driver=sentinel` selects. It is a peer of this module, but points the other way — a source backend decides **where** rules come from, a driver backend decides **who** executes them.

Governance configuration is **not written into `app.properties`** — it is its own document, and changing one rule refreshes governance only, without triggering an app-wide property re-bind.

## Positioning

```
Standalone rules file → FileSource ────────────┐
Governance console/rules API → HTTPSource ─────┼──→ governance.Source → Center → label diff → executor/fault hot-reload
Nacos dataId / etcd key (see starter-governance-nacos / starter-governance-etcd) ┘
```

Every rules document goes through `governance.Parse`: the same document (`spring.governance.*` keys, properties/yaml/json/toml) is **byte-portable** across the file/http/nacos/etcd backends.

- **Inert unless configured**: importing this starter without configuring `spring.governance.source.*` registers nothing, and governance stays disabled (`ExecutorFor` passes through).
- **Configured means it takes over**: the Source bean is injected onto the governance center (priority: an explicit `governance.SetSource` > this bean), and a rule change **refreshes governance only**, never an app-wide config re-bind.
- Single active source: a process has exactly one effective Source (the governance center's contract), so everything here is a conditional singleton bean, not a Group.

## The file source: a standalone rules file

```properties
# app.properties — the one-line wiring
spring.governance.source.file.path=/etc/app/governance.yaml
```

A rules file's keys are the `spring.governance.*` namespace, and its format is detected by extension (json/properties/yaml/toml):

```yaml
spring:
  governance:
    enabled: true
    client:
      default:
        enabled: true
        attempt-timeout: 100ms
      rules:
        - service: redis:cache
          attempt-timeout: 50ms
```

Behavior highlights:

- **fsnotify watches the parent directory** (not the file itself): this accommodates editors that save by atomic rename and the K8s ConfigMap's `..data` symlink atomic swap.
- **Validated at startup**: a missing path or a parse failure fails startup outright, instead of silently arming a disabled center.
- **Last-good on bad edits**: a runtime parse failure, or a file truncated to empty (no `spring.governance.*` key at all), keeps the last good configuration and logs — the correct way to turn governance off is `spring.governance.enabled=false` (the key present), not an empty file.
- **No change, no push**: DeepEqual dedupe, so a touch does not churn executors.

## Troubleshooting

| Symptom | Cause |
|---|---|
| `spring.governance.source.file.path` is configured but governance is not in effect | the bean must be `Export(gs.As[governance.Source]())` to be injected by the center — this starter exports it correctly; if you write your own Source bean and forget the Export, governance silently stays disabled |
| A hot edit does not take effect | check the log for `reload ... failed (keeping last good config)`; make sure you edited the file at the watched path |

## What comes next

Other backends (direct apollo/consul/vault, and so on) join in the FileSource pattern: implement `Snapshot/Subscribe(/Close)`, register conditionally with `OnProperty("spring.governance.source.<name>")`, and Export as `governance.Source`. The direct adapters that already exist each live in their own module, independent of any config-center module: **etcd** is in `starter-governance-etcd` (`spring.governance.source.etcd.*`, Watch push), **nacos** is in `starter-governance-nacos` (`spring.governance.source.nacos.*`, ListenConfig push).

## The http source: governance-console polling

```properties
spring.governance.source.http.url=https://console.example.com/rules/app.yaml
spring.governance.source.http.interval=10s        # default 5s
spring.governance.source.http.format=yaml         # by default inferred from the URL extension
spring.governance.source.http.headers.authorization=Bearer xxx
```

A console that is briefly unavailable (a failed fetch, a non-200, a bad document) keeps the last good configuration; a document that changed is pushed after DeepEqual dedupe.

### Log tag

Runtime logs from this module carry the tag `_app_governance` (the governance center). To tune them
independently of the main log, bind a logger to that tag:

```properties
logger.governance.type=Logger
logger.governance.level=WARN
logger.governance.tag=_app_governance
```
