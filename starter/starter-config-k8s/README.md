# starter-config-k8s

[English](README.md) | [中文](README_CN.md)

`starter-config-k8s` reads a **Kubernetes ConfigMap or Secret directly through
the API server** as a hot-reloadable configuration source. It complements
[starter-config-file](../starter-config-file): the file starter watches a volume
mount and inherits the kubelet's projection latency (~1 min for Secret
rotation); this starter opens a client-go informer straight onto the object, so
a `kubectl edit configmap` propagates to bound `gs.Dync` fields within seconds
and can target any namespace the ServiceAccount may read.

Blank-importing this starter registers a `k8s` config provider consumed via
`spring.app.imports`. This is a **provider-only** starter: there is no injectable
config bean; connection parameters come from the source string.

| Starter | Mechanism | Dependencies | RBAC | When |
| --- | --- | --- | --- | --- |
| `starter-config-file` | Watch a mounted volume (`..data` symlink swap) | none | none | Zero-permission; tolerates kubelet projection latency. |
| `starter-config-k8s` | client-go informer on the ConfigMap/Secret | client-go | `get/list/watch` | Immediate propagation; cross-namespace reach. |

## Installation

```bash
go get go-spring.org/starter-config-k8s
```

## Quick Start

### 1. Import the package

```go
import _ "go-spring.org/starter-config-k8s"
```

### 2. Import config from a ConfigMap/Secret

Source form: `<kind>/<name>[?namespace=..&key=..&format=..&kubeconfig=..]`.

```properties
spring.app.imports=k8s:configmap/app-config?namespace=default&key=application.yaml
```

- `secret/<name>` reads a Secret instead (its `data` is already base64-decoded).
- Add `optional:` (`optional:k8s:configmap/...`) to boot even when the object or
  cluster is absent — the read is skipped and bound fields fall back to defaults.
- To run out-of-cluster, add `&kubeconfig=/home/me/.kube/config`.

### 3. Bind a hot-reloadable field

```go
type Demo struct {
    Message gs.Dync[string] `value:"${demo.message:=none}"`
}
```

`kubectl edit configmap app-config` (change `demo.message`) updates the bound
field within seconds, no restart.

## Source parameters

| Part | Default | Description |
| --- | --- | --- |
| `<kind>` | — | `configmap` or `secret` (required). |
| `<name>` | — | Object name (required). |
| `namespace` | `default` | Object namespace. |
| `key` | (all) | When set, only that one `data` entry is read. |
| `format` | (by extension) | Force a parser (`yaml`/`properties`/`toml`/`json`) for entries without a recognized extension. |
| `kubeconfig` | (empty) | Path to a kubeconfig; empty uses in-cluster auth. |

Each `data` entry is parsed as a config document by its key's extension
(`application.yaml` → YAML) and flattened into properties, mirroring the file
starter's directory semantics; entries with an unknown extension and no forced
`format` are skipped (unless selected explicitly via `key`).

## How It Works

- `loadK8sConfig` reads the object once at startup, flattens its `data` entries
  into properties, and installs a namespaced, name-scoped informer.
- Every add/update/delete on the object triggers a full application property
  refresh, re-running the provider and propagating new values to bound
  `gs.Dync` fields. The refresh is wired via a `gs.Rooter` bridge bean that
  injects the framework's `PropertiesRefresher` (a stable bean name avoids the
  `__default__` Rooter collision).
- The bridge bean's destructor stops every informer on shutdown.

## RBAC

The ServiceAccount needs `get/list/watch` on the target `configmaps`/`secrets`
in its namespace (`get` for the initial read, `list/watch` for the informer).
See [example/deploy/rbac.yaml](example/deploy/rbac.yaml).

## Verifying in a cluster

The unit tests cover parse/read/key-filter/optional-missing and the
informer-driven refresh with the client-go fake clientset. End-to-end
hot-reload needs a real cluster:

```bash
kubectl apply -f example/deploy/rbac.yaml
kubectl apply -f example/deploy/configmap.yaml
# build/push an image for example/ and apply example/deploy/deployment.yaml, then:
kubectl logs deploy/config-k8s-example      # prints demo.message from the ConfigMap
kubectl edit configmap app-config           # change demo.message; the field hot-reloads
```
