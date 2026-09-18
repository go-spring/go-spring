# starter-governance — Governance Center Wiring + Dynamic Source Adapters

[English](README.md) | [中文](README_CN.md)

This starter has two identities:

1. **Wiring** (the always-registered wiring bean, [wiring.go](wiring.go)): it hands the injected `governance.Source` bean to the governance center, registers the executor/fault seams, and fires OnReady — `cloud/governance` itself is container-free (it does not import spring/gs), so **blank-importing this starter makes the whole governance chain take effect**.
2. **Dynamic source adapters**: the self-built refresh chain through which governance rules flow in via the `governance.Source` contract. The file and http sources live in this module; etcd and nacos each live in their own module (`starter-governance-etcd`, `starter-governance-nacos`), a peer of this one, blank-imported on demand.

The family's third kind is the **driver backend**: `starter-governance-sentinel` contributes sentinel-golang as a `resilience.Driver` bean named `sentinel`, which the wiring bean above collects into the driver directory and the governance document's `govern.driver=sentinel` selects. It is a peer of this module, but points the other way — a source backend decides **where** rules come from, a driver backend decides **who** executes them.

Governance configuration is **not written into `app.properties`** — it is its own document, and changing one rule refreshes governance only, without triggering an app-wide property re-bind.

## Positioning

```
Standalone rules file → FileSource ────────────┐
Governance console/rules API → HTTPSource ─────┼──→ governance.Source → Center → label diff → executor/fault hot-reload
Nacos dataId / etcd key (see starter-governance-nacos / starter-governance-etcd) ┘
```

Every rules document goes through `rules.Parse`: the same document (`govern.*` keys, properties/yaml/json/toml) is **byte-portable** across the file/http/nacos/etcd backends.

- **Inert unless configured**: importing this starter without configuring `govern.source.*` registers nothing, and governance stays disabled (`ExecutorFor` passes through).
- **Configured means it takes over**: the Source bean is injected onto the governance center (priority: an explicit `governance.SetSource` > this bean), and a rule change **refreshes governance only**, never an app-wide config re-bind.
- Single active source: a process has exactly one effective Source (the governance center's contract), so everything here is a conditional singleton bean, not a Group.

## The file source: a standalone rules file

```properties
# app.properties — the one-line wiring
govern.source.file.path=/etc/app/govern.yaml
```

A rules file's keys are the `govern.*` namespace, and its format is detected by extension (json/properties/yaml/toml):

```yaml
govern:
  enabled: true
  default:
    enabled: true
    attempt-timeout: 100ms
  rules:
    - resources: redis:cache
      attempt-timeout: 50ms
```

Behavior highlights:

- **fsnotify watches the parent directory** (not the file itself): this accommodates editors that save by atomic rename and the K8s ConfigMap's `..data` symlink atomic swap.
- **Validated at startup**: a missing path or a parse failure fails startup outright, instead of silently arming a disabled center.
- **Last-good on bad edits**: a runtime parse failure, or a file truncated to empty (no `govern.*` key at all), keeps the last good configuration and logs — the correct way to turn governance off is `govern.enabled=false` (the key present), not an empty file.
- **No change, no push**: DeepEqual dedupe, so a touch does not churn executors.

## Troubleshooting

| Symptom | Cause |
|---|---|
| `govern.source.file.path` is configured but governance is not in effect | the bean must be `Export(gs.As[governance.Source]())` to be injected by the center — this starter exports it correctly; if you write your own Source bean and forget the Export, governance silently stays disabled |
| A hot edit does not take effect | check the log for `reload ... failed (keeping last good config)`; make sure you edited the file at the watched path |

## What comes next

Other backends (direct apollo/consul/vault, and so on) join in the FileSource pattern: implement `Snapshot/Subscribe(/Close)`, register conditionally with `OnProperty("govern.source.<name>")`, and Export as `governance.Source`. The direct adapters that already exist each live in their own module, independent of any config-center module: **etcd** is in `starter-governance-etcd` (`govern.source.etcd.*`, Watch push), **nacos** is in `starter-governance-nacos` (`govern.source.nacos.*`, ListenConfig push).

## The http source: governance-console polling

```properties
govern.source.http.url=https://console.example.com/rules/app.yaml
govern.source.http.interval=10s        # default 5s
govern.source.http.format=yaml         # by default inferred from the URL extension
govern.source.http.headers.authorization=Bearer xxx
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
