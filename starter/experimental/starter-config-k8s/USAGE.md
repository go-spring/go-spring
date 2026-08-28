# starter-config-k8s Usage — Reference Manual

Detailed usage reference. Overview: [README.md](README.md). Every behavior claim below is verified
against the starter source (`starter.go`, `provider.go`, `informer.go`, `provider_test.go`), the
core grammar in `spring/conf/provider/provider.go:59-104`, the refresh chain in
`spring/gs/internal/gs_conf/conf.go` and `spring/gs/internal/gs_app/app.go`, and the runnable
[example/](example/). Kubernetes ConfigMap/Secret semantics themselves are
[the Kubernetes docs](https://kubernetes.io/docs/concepts/configuration/configmap/) — everything
below is go-spring's increment.

**Note: this starter lives under `experimental/` — an "unreviewed" marker, not a quality grade.**

**Activation**: a blank import registers the `k8s` config provider
(`provider.go:67`); the provider activates only when a `k8s:` entry appears in
`spring.config.import`. There is no `enabled` key and no other switch.

---

## 1. Complete worked project

A service that pulls its config from a ConfigMap, binds one key to a hot-reloadable
`gs.Dync` field, and demonstrably reloads on `kubectl edit` without restart. Isomorphic to the
smoke-tested [example/](example/). File tree:

```
demo/
├── go.mod
├── main.go
└── conf/
    └── app.properties
```

**go.mod** (deps that matter):

```
require (
    go-spring.org/spring            v1.3.x
    go-spring.org/starter-config-k8s latest
    k8s.io/client-go                 v0.34.x   // pulled in by the starter
)
```

**main.go**:

```go
package main

import (
    "fmt"
    "os"
    "syscall"

    "go-spring.org/log"
    "go-spring.org/spring/gs"

    // Blank-import registers the "k8s" config provider consumed via
    // spring.config.import (provider.go init).
    _ "go-spring.org/starter-config-k8s"
)

// Demo binds a dynamic configuration field sourced from the imported ConfigMap.
// demo.message is a TOP-LEVEL absolute key produced by flattening the ConfigMap
// entry "application.yaml" — not instance-prefixed. Registered as a root object
// so the container creates it eagerly.
type Demo struct {
    Message gs.Dync[string] `value:"${demo.message:=none}"`
}

func main() {
    demoBean := gs.Provide(&Demo{}).Export(gs.As[gs.Rooter]())
    fmt.Println("demo.message =", demoBean.Interface().(*Demo).Message.Value())
    gs.Run()
}
```

**conf/app.properties** — the complete, commented surface:

```properties
spring.app.name=config-k8s-demo

# Import config straight from a Kubernetes ConfigMap via the "k8s" provider.
# Grammar: [optional:]k8s:<kind>/<name>[?namespace=..&key=..&format=..&kubeconfig=..]
#   optional:  boots with defaults when no cluster is reachable (local dev)
#   kind/name: configmap/app-config
#   namespace: default (query param; any namespace the ServiceAccount may read)
#   key:       read only the "application.yaml" data entry of the ConfigMap
spring.config.import=optional:k8s:configmap/app-config?namespace=default&key=application.yaml

# The example only exercises the config provider; disable the default HTTP server.
spring.http.server.enabled=false
```

**The ConfigMap** (from [example/deploy/configmap.yaml](example/deploy/configmap.yaml)):

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
  namespace: default
data:
  application.yaml: |
    demo:
      message: hello-from-configmap
```

**RBAC** — apply [example/deploy/rbac.yaml](example/deploy/rbac.yaml) verbatim; the
ServiceAccount needs `get` (initial read), `list` + `watch` (informer) on configmaps (and/or
secrets) in the namespace.

**Verify**:

```bash
# 0) no cluster at all — boots, prints default, exits cleanly (optional: semantics)
go run .                                    # demo.message = none

# 1) against a real cluster, out-of-cluster: add &kubeconfig=$HOME/.kube/config
#    (and drop optional: to make a missing object fatal)

# 2) in-cluster (deploy example/deploy/*.yaml):
kubectl logs deploy/config-k8s-example       # demo.message = hello-from-configmap

# 3) hot-reload demo — no restart, no volume mount:
kubectl edit configmap app-config            # change demo.message to "v2"
kubectl logs -f deploy/config-k8s-example    # bound gs.Dync field updates within seconds
```

The reload path is real: the informer on the object fires on every add/update/delete
(`informer.go:105-109`) and calls `RefreshProperties` (`starter.go:55-63`), which re-runs the
whole import chain and updates all `gs.Dync[T]` fields (`gs_app/app.go:234-256`). Unit test
`TestHotReloadTriggersRefresh` (`provider_test.go:126-152`) proves the trigger with a fake
clientset.

---

## 2. Assembly & timing

### 2.1 When the import resolves — pre-bean, by design

```
blank import starter-config-k8s
  ├─ init (provider.go:67): conf.RegisterProvider("k8s", k8sController.Load)
  └─ init (starter.go:29):  gs.Provide(k8sController).Name("k8sController")
                            .Export(gs.As[gs.Rooter]()).Destroy((*k8sCtrl).Destroy)

gs.Run()
  ├─ AppConfig.Refresh (gs_conf/conf.go:83-128) — BEFORE any bean is wired:
  │    1. load base properties (files/env/args)
  │    2. bind ${spring.config.import:=} (conf.go:218) — one level only:
  │       imports declared inside an imported file are ignored (conf.go:213-215)
  │    3. for each entry, conf.Load strips [optional:]<provider>: then calls
  │       the provider (provider/provider.go:84-103) → k8sCtrl.Load
  │         a. parseSource (provider.go:85-117)
  │         b. buildClient — in-cluster or kubeconfig (informer.go:38-59)
  │         c. fetch the object via the API server (provider.go:175-192)
  │         d. ensureWatch — informer installed BEFORE returning, so a change
  │            landing right after the initial read is not missed (provider.go:158-161)
  │         e. parseEntries → flatten → merged into the property storage
  │    └─ merged snapshot becomes the storage every value tag binds against
  ├─ container wiring: k8sController gets *gs.PropertiesRefresher autowired
  │  (starter.go:44); app.started flips true (app.go:179-183)
  ├─ Runners/Servers start; ready
  └─ SIGTERM → bean destructor Destroy → manager.stopAll() stops every informer
```

Why pre-bean: the provider's output must be part of the property storage *before* `value:`
tags resolve, so ConfigMap-sourced keys can inject into ordinary bean fields at first wiring —
the same reason `.env` and all config providers run in step 2 of the lifecycle, ahead of
starters. Until the container wires the controller, `TriggerRefresh` is a harmless no-op
(starter.go:52-63): the startup load already captured the initial state, and
`RefreshProperties` itself refuses to run before `started` (app.go:247-250).

### 2.2 The watch/refresh path, walked once

1. `ensureWatch` deduplicates by `kind/namespace/name` (`informer.go:76-84`) — repeated imports
   of the same object never stack informers.
2. A namespaced, `metadata.name=`-field-selected SharedInformerFactory is created with resync
   period 0 (event-driven only; `informer.go:86-93`), on ConfigMaps or Secrets per kind.
3. Add/Update/Delete handlers all funnel into `k8sCtrl.TriggerRefresh` (`informer.go:105-109`).
4. Watch setup is best-effort: if handler registration or cache sync fails, the id is
   forgotten so a later Load may retry, and only hot-reload for that object is lost — the
   static snapshot still loads (`informer.go:110-125`, comment at `provider.go:158-161`).
5. After wiring, each event calls `Refresher.RefreshProperties()` → full `AppConfig.Refresh` →
   **every** provider re-runs its import (the ConfigMap is re-fetched, not diffed) → merged
   storage swapped atomically → all `gs.Dync[T]` fields update. Only `gs.Dync[T]` hot-reloads;
   plain fields and `OnProperty` conditions are startup-only.

---

## 3. Per-key behavior reference

There are **no `value:` tags in this starter** — its entire configuration surface is the
import-string grammar plus the core `spring.config.import` key. The only `value:` tag in the
tree is the *example's* `value:"${demo.message:=none}"` (example/example.go), which binds a
top-level absolute key flattened out of the ConfigMap document.

### 3.1 Import-string grammar

Overall form (core, `provider/provider.go:62-64`):
`[optional:]<provider>:<path>` — `optional:` is cut first (provider.go:85-88), then the first
`:` splits provider from path (provider.go:89-92); a bare path defaults to the `file` provider.
For this starter the `<path>` part is parsed by `parseSource` (`provider.go:85-117`) as:

```
<kind>/<name>[?namespace=..&key=..&format=..&kubeconfig=..]
```

| Param | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-------|------|---------|-------------------------|------------------------------|
| `optional:` prefix | flag | off | With it, client-build failure AND NotFound of the object are logged (Warn) and skipped → app boots with defaults (provider.go:133-138, 150-153). | — (that's its purpose); hides typos in the object name until you drop the prefix. |
| `kind` | string | — **required** | Path segment before the first `/`, lowercased. Only `configmap` and `secret` (provider.go:51-54, 106-110). Secrets' Data is already base64-decoded by the API (provider.go:185-188). | `deployment/x` → boot error "unsupported k8s config kind"; missing `/` or empty name → "must be `<kind>/<name>`". |
| `name` | string | — **required** | Object name; also the informer's field selector (`informer.go:91`). | Not-found without `optional:` → boot error "get configmap/secret … not found". |
| `namespace` | query | `default` | Both the Get and the informer's namespace scope (provider.go:95-96, informer.go:89). Cross-namespace reads need RBAC there. | Forbidden → boot error (or silent skip under `optional:`). |
| `key` | query | all entries | Selects ONE data entry of the object (provider.go:212-214). ⚠ In `key=` mode an entry with an unknown extension is a hard error demanding `format=` (provider.go:222-224), while the same entry is silently *skipped* in all-entries mode — see §6. | `?key=README` on a ConfigMap containing a bare `README` → boot error "entry … has no known format; set format=". |
| `format` | query | by extension | Forces the parser for every read entry (provider.go:215, 111-115). Value must be known to `reader.Has` — validated at parse time. | Unknown value → boot error "unsupported k8s config format" even before any cluster call. |
| `kubeconfig` | query | in-cluster | Path to a kubeconfig file for out-of-cluster runs (informer.go:43-47). Empty → `rest.InClusterConfig()` (informer.go:49-52). | Outside a cluster with no param → boot error "in-cluster config (set kubeconfig when running outside a cluster)" — or Warn+skip under `optional:`. |

### 3.2 Property keys

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `spring.config.import` | list | empty | Core key (bound at `gs_conf/conf.go:218`). One level only: `spring.config.import` inside an imported document is ignored (`conf.go:213-215`). Multiple entries = later-loaded sources override earlier ones (`conf.go:202`). | Missing `k8s:` entry → provider never runs (starter inert). |
| ConfigMap/Secret data keys (e.g. `demo.message`) | any | none | Flattened from each parsed entry (`provider.go:233`); become top-level absolute properties bindable by any `value:"${...}"` tag anywhere. | Bad YAML/properties inside an entry → boot error "parse entry %q" (provider.go:230-232). |
| `spring.http.server.enabled=false` | bool | true | Example-only convenience to skip the default HTTP server for a config-only app. | — |

---

## 4. Verification & fault drills

The automated smoke (`example/check.sh`) is two-part: unit tests with the fake clientset, and a
clean-exit boot outside a cluster (`optional:` path). The drills below are the in-cluster
counterparts, same shape as the example deployment.

### 4.1 Cold load

```bash
kubectl apply -f example/deploy/rbac.yaml -f example/deploy/configmap.yaml -f example/deploy/deployment.yaml
kubectl logs deploy/config-k8s-example
# demo.message = hello-from-configmap            <- from the ConfigMap, not the default
```

Local, no cluster: `cd example && go run .` prints `demo.message = none` (the `:=` default)
and self-terminates — proving the wiring registers and boots without a control plane.

### 4.2 Watch push (hot-reload, no restart)

```bash
kubectl edit configmap app-config        # demo.message: hello-v2
kubectl logs -f deploy/config-k8s-example  # any gs.Dync consumer sees hello-v2 within seconds
```

Seconds vs the ~1-minute kubelet projection of a volume mount: the informer talks to the API
server directly (package doc, `provider.go:24-30`). Deleting the ConfigMap also fires the
informer (DeleteFunc), but the re-import then fails — properties keep the last good snapshot
only until another refresh succeeds; treat delete-while-running as an incident, not a feature.

### 4.3 Malformed ConfigMap content

Put invalid YAML in the `application.yaml` entry and restart (cold path):

```bash
kubectl edit configmap app-config   # break the YAML
kubectl rollout restart deploy/config-k8s-example
kubectl logs deploy/config-k8s-example   # boot error: k8s config: parse entry "application.yaml" ...
```

Note asymmetry: at *runtime* a malformed edit first fails the refresh (error logged, storage
not swapped — `RefreshProperties` applies no partial updates, app.go:244-246); the process
keeps serving the previous values.

### 4.4 optional vs required

```properties
spring.config.import=k8s:configmap/app-config?...        # required: missing object = boot fails
spring.config.import=optional:k8s:configmap/app-config?... # missing object = Warn + defaults
```

`optional:` covers two distinct failures (provider.go:133-138 and 150-153): no cluster/client
buildable, and object NotFound. Everything else — RBAC Forbidden, malformed content, bad
`format=` — is still fatal.

### 4.5 API server unreachable at boot

Required mode: `go run .` fails with the client-build or Get error chain. Optional mode: Warn
`optional config build client failed (skipped)` and defaults load. Post-boot API-server loss:
the informer retries internally (client-go reflector backoff); refreshes stall until it
returns; the loaded snapshot keeps serving.

### 4.6 Key-filter and format drills

```bash
# secret with db.properties only:
spring.config.import=k8s:secret/app-creds?key=db.properties
# entry without a known extension, selected via key= -> must add &format=properties or boot fails
```

Covered by `TestLoadSecretPropsWithKeyFilter` and `TestUnknownExtensionSkippedButKeyFilterErrors`
(provider_test.go:74-124). Reproduce the unit suite anytime with:

```bash
cd starter/experimental/starter-config-k8s && go test -gcflags="all=-N -l" ./...
```

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Boot error `in-cluster config (set kubeconfig when running outside a cluster)` | running locally, no `kubeconfig=` param | add `?kubeconfig=$HOME/.kube/config`, or `optional:` to skip |
| Boot error `get configmap … not found` | wrong `name`/`namespace`, or object not applied | fix the source string or apply the ConfigMap; `optional:` to tolerate |
| Boot error `forbidden: User … cannot get/list/watch configmaps` | RBAC missing/too narrow | apply example/deploy/rbac.yaml; needs get+list+watch (informer), in the target namespace |
| Config loads but never hot-reloads | informer failed best-effort (cache sync) or watch verbs missing while get works | check RBAC `watch`; restart to retry watch setup; look for Warn/Error under tag `_app_def` |
| `demo.message` stays at default in-cluster | import not actually listed in `spring.config.import`, or `key=` selects an entry that doesn't exist | empty `key=` match yields an empty (not failed) import — verify the data-entry name |
| Boot error `entry "README" has no known format; set format=` | `key=` selected an extension-less entry | add `&format=properties` (etc.), or import without `key=` so it's skipped |
| Boot error `unsupported k8s config kind "deployment"` | only `configmap`/`secret` supported | pick one of the two kinds |
| Logs show nothing from the starter | Debug-level lines hidden (provider.go:129 logs the parsed source at Debug) | raise logger level for tag `_app_def` |

---

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys (import-string params) | 7 (incl. `optional:`) |
| Property keys owned | 0 (only `spring.config.import`, a core key) |
| Required params | 2 (`kind`, `name`) |
| Quickstart external deps | 0 local / 1 cluster |
| "Watch out" entries | 4 (key/format severity asymmetry, one-level imports, delete-while-running, watch best-effort) |

Suspects (carried over + new, for the audit ledger):

- `key=` with an extension-less entry hard-fails while all-entries mode silently skips the same
  entry — inconsistent severity for one misconfig (`provider.go:222-226`).
- A clientset is rebuilt per import (no shared client cache across imports of the same
  kubeconfig) — minor waste; candidate dedup following the etcd/nacos `clientFor` pattern.
- Refresh is whole-app, not per-object: one ConfigMap edit re-runs *every* import (file, etcd,
  nacos, …) — correct but O(all sources) per push; fine at config-edit frequency, worth
  remembering if imports ever become chatty.
- Delete-event handling is a latent gap: the informer fires refresh on delete, the re-import
  then fails, and the "last good snapshot" behavior is emergent rather than designed.
- No observability surface of its own (no log tag, no health indicator for "watch alive",
  no metric on refresh count) — a silent stale-config failure mode is undetectable from outside.
